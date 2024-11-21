package ctrlauth

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/benw10-1/brotato-exporter/brotatomod/brotatomodtypes"
	"github.com/benw10-1/brotato-exporter/exporterstore"
	"github.com/benw10-1/brotato-exporter/exporterstore/exporterstoretypes"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAuth(t *testing.T) {
	asserter := require.New(t)

	jwtKey := []byte("567A7A74316D31396E614B4758474951")

	sessionInfoMap := new(SessionInfoMap)

	exporterStore, err := exporterstore.NewExporterStore(filepath.Join(t.TempDir(), "user.db"))
	asserter.NoError(err)

	defer exporterStore.Close()

	authAPI := NewAuthAPI(jwtKey, sessionInfoMap, exporterStore)

	testUser := &exporterstoretypes.ExporterUser{
		UserID:         uuid.New(),
		MaxSubscribers: 10,
	}
	err = exporterStore.UpsertUser(testUser)
	asserter.NoError(err)

	testAuthToken := []byte("test")

	err = exporterStore.UpsertAuthKeyUserID(testAuthToken, testUser.UserID)
	asserter.NoError(err)

	doReq := func(req *http.Request) (nextCtx context.Context, w *httptest.ResponseRecorder) {
		w = httptest.NewRecorder()

		nextCtx = authAPI.ServeHTTPNextCtx(w, req)

		return nextCtx, w
	}

	t.Run("TestAuthenticateUser", func(t *testing.T) {
		t.Run("TestInvalidToken", func(t *testing.T) {
			asserter := require.New(t)
			// check that we get a 401 when we don't have a token
			req, err := http.NewRequest("POST", "/api/auth/authenticate", nil)
			asserter.NoError(err)

			_, w := doReq(req)
			asserter.Equal(http.StatusUnauthorized, w.Code)

			// check that we get a 401 when we have an invalid token
			req, err = http.NewRequest("POST", "/api/auth/authenticate", nil)
			asserter.NoError(err)
			req.Header.Set("Authorization", "Bearer invalid")

			_, w = doReq(req)
			asserter.Equal(http.StatusUnauthorized, w.Code)
		})

		t.Run("TestValidToken", func(t *testing.T) {
			asserter := require.New(t)

			req, err := http.NewRequest("POST", "/api/auth/authenticate", nil)
			asserter.NoError(err)
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", testAuthToken))

			_, w := doReq(req)
			asserter.Equal(http.StatusOK, w.Code)

			// should be octet-stream by default
			asserter.Equal("application/octet-stream", w.Result().Header.Get("Content-Type"))

			sessionTokenStreamBytes, err := io.ReadAll(w.Body)
			asserter.NoError(err)

			asserter.GreaterOrEqual(len(sessionTokenStreamBytes), 8)

			microTime := brotatomodtypes.MicroTime(binary.LittleEndian.Uint64(sessionTokenStreamBytes[:8]))
			expireTime := microTime.Time()

			// check that it expires in about 24 hours
			diff := time.Until(expireTime).Microseconds() - expirationDuration.Microseconds()
			asserter.Less(diff, 1000000*time.Microsecond)

			asserter.GreaterOrEqual(len(sessionTokenStreamBytes), 10)

			bytesLen := binary.LittleEndian.Uint16(sessionTokenStreamBytes[8:10])

			asserter.Equal(len(sessionTokenStreamBytes), 10+int(bytesLen))

			sessionToken := sessionTokenStreamBytes[10:]

			sess, ok := sessionInfoMap.Load(testUser.UserID)
			asserter.True(ok)

			sessRes, err := ParseSessionToken(jwtKey, string(sessionToken))
			asserter.NoError(err)

			asserter.Equal(sess.Session.UserID, sessRes.UserID)

			// test JSON output

			req, err = http.NewRequest("POST", "/api/auth/authenticate", nil)
			asserter.NoError(err)

			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", testAuthToken))
			req.Header.Set("Content-Type", "application/json")

			_, w = doReq(req)
			asserter.Equal(http.StatusOK, w.Code)

			asserter.Equal("application/json", w.Result().Header.Get("Content-Type"))

			authResponse := new(AuthResponse)
			err = json.Unmarshal(w.Body.Bytes(), authResponse)
			asserter.NoError(err)

			sess, ok = sessionInfoMap.Load(testUser.UserID)
			asserter.True(ok)

			sessRes, err = ParseSessionToken(jwtKey, string(authResponse.SessionToken))
			asserter.NoError(err)

			asserter.Equal(sess.Session.UserID, sessRes.UserID)
		})

		t.Run("TestExpiredToken", func(t *testing.T) {
			asserter := require.New(t)

			req, err := http.NewRequest("POST", "/api/auth/authenticate", nil)
			asserter.NoError(err)

			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", testAuthToken))
			req.Header.Set("Content-Type", "application/json")

			_, w := doReq(req)
			asserter.Equal(http.StatusOK, w.Code)

			asserter.Equal("application/json", w.Result().Header.Get("Content-Type"))

			authResponse := new(AuthResponse)
			err = json.Unmarshal(w.Body.Bytes(), authResponse)
			asserter.NoError(err)

			req, err = http.NewRequest("POST", "/", nil)
			asserter.NoError(err)

			req.Header.Set("Authorization", fmt.Sprintf("JWT %s", authResponse.SessionToken))

			nextCtx, w := doReq(req)
			asserter.Equal(http.StatusOK, w.Code)

			sess, ok := GetSessionFromCtx(nextCtx)
			asserter.True(ok)

			asserter.Equal(testUser.UserID, sess.UserID)
		})
	})
}

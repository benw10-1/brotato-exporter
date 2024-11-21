package ctrlauth

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/benw10-1/brotato-exporter/brotatomod/brotatomodtypes"
	"github.com/benw10-1/brotato-exporter/brotatomod/brotatoserial"
	"github.com/benw10-1/brotato-exporter/errutil"
	"github.com/benw10-1/brotato-exporter/exporterserver/exporterserverutil"
	"github.com/benw10-1/brotato-exporter/exporterstore"
	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
)

// AuthAPI
type AuthAPI struct {
	sessionInfoMap *SessionInfoMap

	jwtKey []byte

	exporterStore *exporterstore.ExporterStore

	*httprouter.Router
}

// NewAuthAPI
func NewAuthAPI(jwtKey []byte, sessionInfoMap *SessionInfoMap, exporterStore *exporterstore.ExporterStore) *AuthAPI {
	router := httprouter.New()

	api := &AuthAPI{
		jwtKey:         jwtKey,
		sessionInfoMap: sessionInfoMap,
		exporterStore:  exporterStore,
		Router:         router,
	}

	router.POST("/api/auth/authenticate", api.authenticateUser)
	router.NotFound = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	return api
}

// AuthResponse
type AuthResponse struct {
	SessionToken string `json:"token"`
	ExpireTime   string `json:"expire_time"`
}

// WriteStream
func (ar *AuthResponse) WriteStream(w io.Writer) error {
	// token len + expire time + token length header
	buf := make([]byte, 0, len(ar.SessionToken)+8+2)

	t, err := time.Parse(timeFormat, ar.ExpireTime)
	if err != nil {
		return errutil.NewStackError(err)
	}

	microSecondEpoch := brotatomodtypes.MicroTimeFromTime(t)

	buf = binary.LittleEndian.AppendUint64(buf, uint64(microSecondEpoch))

	buf = binary.LittleEndian.AppendUint16(buf, uint16(len(ar.SessionToken)))
	buf = append(buf, []byte(ar.SessionToken)...)

	_, err = w.Write(buf)
	if err != nil {
		return errutil.NewStackError(err)
	}

	return nil
}

const timeFormat = "2006-01-02T15:04:05Z07:00"

// authenticateUser (no swagger header, this is internal)
func (api *AuthAPI) authenticateUser(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	exporterserverutil.WriteError(w, func() error {
		userID, ok := GetUserIDFromCtx(r.Context())
		if !ok {
			return exporterserverutil.NewResponseError(nil, http.StatusUnauthorized, "Unauthorized")
		}

		tokenStr, sess, err := NewSessionToken(api.jwtKey, userID)
		if err != nil {
			return exporterserverutil.NewResponseError(errutil.NewStackError(err), http.StatusInternalServerError, "Failed to create session token")
		}

		sessInfo := &sessionInfo{
			Session: sess,
		}

		oldSess, ok := api.sessionInfoMap.Load(userID)
		if ok {
			// retain old session message reader
			// block if something is mid read
			oldSess.Lock()
			defer oldSess.Unlock()

			sessInfo.MessageReader = oldSess.MessageReader
			sessInfo.CurrentSessionState = oldSess.CurrentSessionState
		} else {
			sessInfo.MessageReader = brotatoserial.NewMessageReader(nil, make([]byte, 1024))
			sessInfo.CurrentSessionState = make(map[string]json.RawMessage)
		}

		api.sessionInfoMap.Store(sess.UserID, sessInfo)

		authResponse := &AuthResponse{
			SessionToken: tokenStr,
			ExpireTime:   sess.ExpiresAt.Time.Format(timeFormat),
		}

		if r.Header.Get("Content-Type") == "application/json" {
			w.Header().Set("Content-Type", "application/json")

			err = json.NewEncoder(w).Encode(authResponse)
			if err != nil {
				return exporterserverutil.NewResponseError(errutil.NewStackError(err), http.StatusInternalServerError, "Failed to write JSON")
			}

			return nil
		}
		w.Header().Set("Content-Type", "application/octet-stream")

		err = authResponse.WriteStream(w)
		if err != nil {
			return exporterserverutil.NewResponseError(errutil.NewStackError(err), http.StatusInternalServerError, "Failed to write stream")
		}

		w.WriteHeader(http.StatusOK)

		return nil
	}())
}

// ServeHTTPNextCtx "middleware"
func (api *AuthAPI) ServeHTTPNextCtx(w http.ResponseWriter, r *http.Request) context.Context {
	authHeaderValue := r.Header.Get("Authorization")
	if authHeaderValue == "" {
		return r.Context()
	}

	var nextCtx context.Context
	if strings.HasPrefix(authHeaderValue, "JWT ") {
		tokenString := authHeaderValue[4:] // remove "JWT " prefix

		session, err := ParseSessionToken(api.jwtKey, tokenString)
		if err != nil {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return r.Context()
		}
		nextCtx = context.WithValue(r.Context(), SessionCtxKey, session)
	} else if strings.HasPrefix(authHeaderValue, "Bearer ") {
		authToken := authHeaderValue[7:] // remove "Bearer " prefix

		userID, err := api.exporterStore.GetUserIDByAuthKey([]byte(authToken))
		if err != nil {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return r.Context()
		}

		nextCtx = context.WithValue(r.Context(), UserIDCtxKeyStr, userID)
	} else {
		nextCtx = r.Context()
	}

	api.Router.ServeHTTP(w, r.WithContext(nextCtx))
	return nextCtx
}

type UserIDCtxKey string

const UserIDCtxKeyStr UserIDCtxKey = "user_id"

// GetUserIDFromCtx user ID for the provided Bearer token
func GetUserIDFromCtx(ctx context.Context) (uuid.UUID, bool) {
	userID, ok := ctx.Value(UserIDCtxKeyStr).(uuid.UUID)
	return userID, ok
}

package ctrlauth

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/benw10-1/brotato-exporter/brotatomod/brotatoserial"
	"github.com/benw10-1/brotato-exporter/errutil"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

type SessionCtxKeyType string

const SessionCtxKey SessionCtxKeyType = "session"

const expirationDuration = time.Minute * 15

// Session
type Session struct {
	UserID uuid.UUID `json:"user_id"`

	jwt.RegisteredClaims
}

// NewSessionToken
func NewSessionToken(authKey []byte, userID uuid.UUID) (tokenStr string, sess *Session, err error) {
	sess = &Session{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expirationDuration)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, sess)
	tokenStr, err = token.SignedString(authKey)
	if err != nil {
		return "", nil, errutil.NewStackError(err)
	}

	return tokenStr, sess, nil
}

// ParseSessionToken
func ParseSessionToken(authKey []byte, tokenStr string) (*Session, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Session{}, func(token *jwt.Token) (interface{}, error) {
		return authKey, nil
	})
	if err != nil {
		return nil, errutil.NewStackError(err)
	}

	if !token.Valid {
		return nil, errutil.NewStackError("invalid token")
	}

	return token.Claims.(*Session), nil
}

// GetSessionFromCtx
func GetSessionFromCtx(ctx context.Context) (*Session, bool) {
	sessIface := ctx.Value(SessionCtxKey)
	if sessIface == nil {
		return nil, false
	}

	sess, ok := sessIface.(*Session)
	if !ok {
		return nil, false
	}

	if time.Now().After(sess.ExpiresAt.Time) {
		return nil, false
	}

	return sess, ok
}

// sessionInfo
type sessionInfo struct {
	Session *Session

	MessageReader *brotatoserial.BrotatoMessageReader

	// CurrentSessionState mapped keys to their values
	CurrentSessionState map[string]json.RawMessage

	// lock to handle edge-case where next message is sent before the previous message has finished reading.
	// If its just 1 thread htting this lock it will just be a CAS so this does not impact performance too much.
	sync.Mutex
}

// SessionInfoMap
type SessionInfoMap struct {
	sync.Map
}

// Store
func (s *SessionInfoMap) Store(userID uuid.UUID, value *sessionInfo) {
	s.Map.Store(userID, value)
}

// Load
func (s *SessionInfoMap) Load(userID uuid.UUID) (*sessionInfo, bool) {
	value, ok := s.Map.Load(userID)
	if !ok {
		return nil, false
	}

	return value.(*sessionInfo), true
}

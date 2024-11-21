package exporterstore

import (
	"errors"

	"github.com/benw10-1/brotato-exporter/errutil"
	"github.com/benw10-1/brotato-exporter/exporterstore/exporterstoretypes"
	"github.com/google/uuid"
)

const userBucket = "users"

var ErrUserNotFound = errors.New("user not found")

// GetUserByID
func (es *ExporterStore) GetUserByID(userID uuid.UUID) (*exporterstoretypes.ExporterUser, error) {
	cachedUser, ok := es.userCache.Get(userID)
	if ok {
		return &cachedUser, nil
	}

	tx, err := es.boltDB.Begin(false)
	if err != nil {
		return nil, errutil.NewStackError(err)
	}
	defer tx.Rollback()

	bucket := tx.Bucket([]byte(userBucket))

	userBytes := bucket.Get(userID[:])
	if userBytes == nil {
		return nil, errutil.NewStackError(ErrUserNotFound)
	}

	user := new(exporterstoretypes.ExporterUser)

	err = user.UnmarshalMsg(userBytes)
	if err != nil {
		return nil, errutil.NewStackError(err)
	}

	es.userCache.Add(userID, *user)

	return user, nil
}

// UpsertUser
func (es *ExporterStore) UpsertUser(user *exporterstoretypes.ExporterUser) error {
	tx, err := es.boltDB.Begin(true)
	if err != nil {
		return errutil.NewStackError(err)
	}
	defer tx.Rollback()

	if user.UserID == uuid.Nil {
		user.UserID = uuid.New()
	}

	bucket := tx.Bucket([]byte(userBucket))

	userBytes, err := user.MarshalMsg()
	if err != nil {
		return errutil.NewStackError(err)
	}

	err = bucket.Put(user.UserID[:], userBytes)
	if err != nil {
		return errutil.NewStackError(err)
	}

	err = tx.Commit()
	if err != nil {
		return errutil.NewStackError(err)
	}

	es.userCache.Add(user.UserID, *user)

	return nil
}

package exporterstore

import (
	"github.com/benw10-1/brotato-exporter/errutil"
	"github.com/google/uuid"
)

const authKeyBucket = "authkeys"

// GetUserIDByAuthKey
func (es *ExporterStore) GetUserIDByAuthKey(authKey []byte) (uuid.UUID, error) {
	userID, ok := es.authKeyCache.Get(string(authKey))
	if ok {
		if userID == uuid.Nil {
			return uuid.Nil, errutil.NewStackError(ErrUserNotFound)
		}

		return userID, nil
	}

	tx, err := es.boltDB.Begin(false)
	if err != nil {
		return uuid.Nil, errutil.NewStackError(err)
	}
	defer tx.Rollback()

	bucket := tx.Bucket([]byte(authKeyBucket))

	userIDBytes := bucket.Get(authKey)
	if userIDBytes == nil {
		// also cache misses
		es.authKeyCache.Add(string(authKey), uuid.Nil)
		return uuid.Nil, errutil.NewStackError(ErrUserNotFound)
	}

	err = userID.UnmarshalBinary(userIDBytes)
	if err != nil {
		return uuid.Nil, errutil.NewStackError(err)
	}

	es.authKeyCache.Add(string(authKey), userID)

	return userID, nil
}

// UpsertAuthKeyUserID
func (es *ExporterStore) UpsertAuthKeyUserID(authKey []byte, userID uuid.UUID) error {
	tx, err := es.boltDB.Begin(true)
	if err != nil {
		return errutil.NewStackError(err)
	}
	defer tx.Rollback()

	bucket := tx.Bucket([]byte(authKeyBucket))

	userIDBytes, err := userID.MarshalBinary()
	if err != nil {
		return errutil.NewStackError(err)
	}

	err = bucket.Put(authKey, userIDBytes)
	if err != nil {
		return errutil.NewStackError(err)
	}

	err = tx.Commit()
	if err != nil {
		return errutil.NewStackError(err)
	}

	es.authKeyCache.Add(string(authKey), userID)

	return nil
}

package exporterstore

import (
	"os"
	"path/filepath"

	"github.com/benw10-1/brotato-exporter/errutil"
	"github.com/benw10-1/brotato-exporter/exporterstore/exporterstoreauthkeycache"
	"github.com/benw10-1/brotato-exporter/exporterstore/exporterstoreusercache"
	"github.com/boltdb/bolt"
)

// ExporterStore
type ExporterStore struct {
	boltDB       *bolt.DB
	userCache    *exporterstoreusercache.UserIDUserCache
	authKeyCache *exporterstoreauthkeycache.AuthKeyUserIDCache
}

// NewExporterStore
func NewExporterStore(boltDBPath string) (*ExporterStore, error) {
	err := os.MkdirAll(filepath.Dir(boltDBPath), 0755)
	if err != nil {
		panic(err)
	}

	boltDB, err := bolt.Open(boltDBPath, 0600, nil)
	if err != nil {
		return nil, errutil.NewStackError(err)
	}

	userCache, err := exporterstoreusercache.NewExporterStoreUserCache(100)
	if err != nil {
		return nil, errutil.NewStackError(err)
	}

	authKeyCache, err := exporterstoreauthkeycache.NewExporterAuthKeyCache(100)
	if err != nil {
		return nil, errutil.NewStackError(err)
	}

	es := &ExporterStore{
		boltDB:       boltDB,
		userCache:    userCache,
		authKeyCache: authKeyCache,
	}

	err = es.initBuckets()
	if err != nil {
		return nil, errutil.NewStackError(err)
	}

	return es, nil
}

// initBuckets
func (es *ExporterStore) initBuckets() error {
	tx, err := es.boltDB.Begin(true)
	if err != nil {
		return errutil.NewStackError(err)
	}
	defer tx.Rollback()

	_, err = tx.CreateBucketIfNotExists([]byte(authKeyBucket))
	if err != nil {
		return errutil.NewStackError(err)
	}

	_, err = tx.CreateBucketIfNotExists([]byte(userBucket))
	if err != nil {
		return errutil.NewStackError(err)
	}

	err = tx.Commit()
	if err != nil {
		return errutil.NewStackError(err)
	}

	return nil
}

// Close
func (es *ExporterStore) Close() error {
	err := es.boltDB.Close()
	if err != nil {
		return errutil.NewStackError(err)
	}

	return nil
}

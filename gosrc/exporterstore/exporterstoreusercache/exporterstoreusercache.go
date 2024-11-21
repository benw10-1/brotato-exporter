package exporterstoreusercache

import (
	"github.com/benw10-1/brotato-exporter/errutil"
	"github.com/benw10-1/brotato-exporter/exporterstore/exporterstoretypes"
	"github.com/google/uuid"
	lru "github.com/hashicorp/golang-lru/v2"
)

// UserIDUserCache
type UserIDUserCache struct {
	*lru.Cache[uuid.UUID, exporterstoretypes.ExporterUser]
}

// NewExporterStoreUserCache
func NewExporterStoreUserCache(maxEntries int) (*UserIDUserCache, error) {
	cache, err := lru.New[uuid.UUID, exporterstoretypes.ExporterUser](maxEntries)
	if err != nil {
		return nil, errutil.NewStackError(err)
	}

	return &UserIDUserCache{cache}, nil
}

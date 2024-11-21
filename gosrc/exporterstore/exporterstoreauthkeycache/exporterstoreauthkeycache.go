package exporterstoreauthkeycache

import (
	"github.com/benw10-1/brotato-exporter/errutil"
	"github.com/google/uuid"
	lru "github.com/hashicorp/golang-lru/v2"
)

// AuthKeyUserIDCache
type AuthKeyUserIDCache struct {
	*lru.Cache[string, uuid.UUID]
}

// NewExporterAuthKeyCache
func NewExporterAuthKeyCache(maxEntries int) (*AuthKeyUserIDCache, error) {
	cache, err := lru.New[string, uuid.UUID](maxEntries)
	if err != nil {
		return nil, errutil.NewStackError(err)
	}

	return &AuthKeyUserIDCache{cache}, nil
}

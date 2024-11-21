package exporterstore

import (
	"path/filepath"
	"testing"

	"github.com/benw10-1/brotato-exporter/exporterstore/exporterstoretypes"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestUser(t *testing.T) {
	asserter := require.New(t)

	exporterStore, err := NewExporterStore(filepath.Join(t.TempDir(), "user.db"))
	asserter.NoError(err)
	defer exporterStore.Close()

	user := &exporterstoretypes.ExporterUser{
		UserID:         uuid.New(),
		MaxSubscribers: 5,
	}

	err = exporterStore.UpsertUser(user)
	asserter.NoError(err)

	user2, err := exporterStore.GetUserByID(user.UserID)
	asserter.NoError(err)

	asserter.Equal(user.UserID, user2.UserID)
	asserter.Equal(user.MaxSubscribers, user2.MaxSubscribers)

	user.MaxSubscribers = 10

	err = exporterStore.UpsertUser(user)
	asserter.NoError(err)

	user3, err := exporterStore.GetUserByID(user.UserID)
	asserter.NoError(err)

	asserter.Equal(user.UserID, user3.UserID)
	asserter.Equal(user.MaxSubscribers, user3.MaxSubscribers)
}

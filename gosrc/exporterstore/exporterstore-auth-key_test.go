package exporterstore

import (
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAuthKeyUserID(t *testing.T) {
	asserter := require.New(t)

	exporterStore, err := NewExporterStore(filepath.Join(t.TempDir(), "authkey.db"))
	asserter.NoError(err)
	defer exporterStore.Close()

	authKey := "authkey"
	userID := uuid.New()

	err = exporterStore.UpsertAuthKeyUserID([]byte(authKey), userID)
	asserter.NoError(err)

	userID2, err := exporterStore.GetUserIDByAuthKey([]byte(authKey))
	asserter.NoError(err)

	asserter.Equal(userID, userID2)
}

package database

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBackfillTenantAPIKeyHintsConvertsStoredTokens(t *testing.T) {
	repoRoot := sqliteRepoRoot(t)
	chdirAndRestore(t, copySQLiteMigrationsThrough(t, repoRoot, 35))
	dbPath := filepath.Join(t.TempDir(), "keys.db")
	opts := MigrationOptions{SQLiteDBPath: dbPath}
	require.NoError(t, RunMigrationsWithOptions("sqlite3://unused", opts))
	raw := openSQLiteDB(t, dbPath)
	_, err := raw.Exec(`INSERT INTO tenants (id, name, business) VALUES (7, 'Upgraded workspace', 'test')`)
	require.NoError(t, err)
	const legacyToken = "sk-legacyTokenStoredInPlaintext1234"
	_, err = raw.Exec(`INSERT INTO tenant_api_keys (tenant_id, name, key_hash, api_key)
		VALUES (7, 'Tenant API key', 'migrated-tenant-7', ?)`, legacyToken)
	require.NoError(t, err)

	chdirAndRestore(t, repoRoot)
	require.NoError(t, RunMigrationsWithOptions("sqlite3://unused", opts))
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, BackfillTenantAPIKeyHints(context.Background(), db))
	require.NoError(t, BackfillTenantAPIKeyHints(context.Background(), db), "a second start is a no-op")

	var keyHash, keyHint, apiKey string
	require.NoError(t, raw.QueryRow(`SELECT key_hash, key_hint, api_key FROM tenant_api_keys WHERE tenant_id = 7`).
		Scan(&keyHash, &keyHint, &apiKey))
	require.Equal(t, types.HashTenantAPIKey(legacyToken), keyHash, "the upgraded key authenticates by hash")
	require.Equal(t, "sk-lega...1234", keyHint)
	require.Empty(t, apiKey, "the token is no longer stored")
}

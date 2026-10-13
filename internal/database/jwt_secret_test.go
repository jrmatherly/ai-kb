package database

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/utils"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestResolveDefaultJWTSecretPersistsOnePerInstallSecret(t *testing.T) {
	chdirAndRestore(t, sqliteRepoRoot(t))
	dbPath := filepath.Join(t.TempDir(), "jwt.db")
	require.NoError(t, RunMigrationsWithOptions("sqlite3://unused", MigrationOptions{SQLiteDBPath: dbPath}))
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)

	t.Setenv("JWT_SECRET", utils.DefaultJWTSecret)
	require.NoError(t, ResolveDefaultJWTSecret(db))
	first := os.Getenv("JWT_SECRET")
	require.Len(t, first, 64)

	t.Setenv("JWT_SECRET", "CHANGE-ME-jwt-secret")
	require.NoError(t, ResolveDefaultJWTSecret(db))
	require.Equal(t, first, os.Getenv("JWT_SECRET"), "a restart reuses the stored secret")

	t.Setenv("JWT_SECRET", "operator-chosen-secret")
	require.NoError(t, ResolveDefaultJWTSecret(db))
	require.Equal(t, "operator-chosen-secret", os.Getenv("JWT_SECRET"), "an explicit secret is left alone")
}

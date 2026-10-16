package im

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	apperrors "github.com/jrmatherly/ai-kb/internal/errors"
)

func TestCheckDuplicateBotNamesOnlySameTenantChannels(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.Exec(`CREATE TABLE im_channels (
		id TEXT PRIMARY KEY, tenant_id INTEGER NOT NULL, name TEXT NOT NULL DEFAULT '',
		bot_identity TEXT NOT NULL DEFAULT '', deleted_at DATETIME)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO im_channels (id, tenant_id, name, bot_identity)
		VALUES ('ch-1', 1, 'Support bot', 'feishu:cli_shared')`).Error)
	svc := &Service{db: db}
	candidate := func(tenantID uint64) *IMChannel {
		return &IMChannel{TenantID: tenantID, Platform: "feishu", Credentials: []byte(`{"app_id":"cli_shared"}`)}
	}

	for _, tc := range []struct {
		name     string
		tenantID uint64
		want     string
	}{
		{
			"same tenant", 1,
			`this bot is already bound to channel "Support bot" (ch-1); each bot can only be connected to one channel`,
		},
		{
			"other tenant", 2,
			"this bot is already bound to another channel; each bot can only be connected to one channel",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			appErr, ok := apperrors.IsAppError(svc.checkDuplicateBot(candidate(tc.tenantID), ""))
			require.True(t, ok)
			require.Equal(t, http.StatusConflict, appErr.HTTPCode)
			require.Equal(t, tc.want, appErr.Message)
		})
	}
	require.NoError(t, svc.checkDuplicateBot(candidate(1), "ch-1"), "the channel itself is not a duplicate")
}

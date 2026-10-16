package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/jrmatherly/ai-kb/internal/im"
	"github.com/jrmatherly/ai-kb/internal/middleware"
)

func TestIMCallbackRejectionsWriteEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.Exec(`CREATE TABLE im_channels (
		id TEXT PRIMARY KEY, tenant_id INTEGER NOT NULL, agent_id TEXT NOT NULL, platform TEXT NOT NULL,
		name TEXT NOT NULL DEFAULT '', enabled NUMERIC NOT NULL DEFAULT 1, mode TEXT NOT NULL DEFAULT 'websocket',
		output_mode TEXT NOT NULL DEFAULT 'stream', locale TEXT NOT NULL DEFAULT '', knowledge_base_id TEXT DEFAULT '',
		bot_identity TEXT NOT NULL DEFAULT '', session_mode TEXT NOT NULL DEFAULT 'user',
		credentials TEXT NOT NULL DEFAULT '{}', created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO im_channels (id, tenant_id, agent_id, platform, enabled, mode) VALUES
		('off', 1, 'a', 'slack', 0, 'webhook'), ('no-adapter', 1, 'a', 'slack', 1, 'webhook')`).Error)

	svc := im.NewService(db, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	t.Cleanup(svc.Stop)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.POST("/im/callback/:channel_id", NewIMHandler(svc).IMCallback)

	for _, tc := range []struct {
		channel string
		status  int
		body    string
	}{
		{"missing", http.StatusNotFound, `{"success":false,"error":{"code":1003,"message":"channel not found"}}`},
		{
			"off", http.StatusServiceUnavailable,
			`{"success":false,"error":{"code":1008,"message":"channel is disabled"}}`,
		},
		{
			"no-adapter", http.StatusServiceUnavailable,
			`{"success":false,"error":{"code":1008,"message":"channel not available"}}`,
		},
	} {
		t.Run(tc.channel, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/im/callback/"+tc.channel, nil))
			require.Equal(t, tc.status, w.Code, w.Body.String())
			require.JSONEq(t, tc.body, w.Body.String())
		})
	}
}

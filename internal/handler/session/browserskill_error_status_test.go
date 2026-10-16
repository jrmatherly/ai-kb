package session

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/jrmatherly/ai-kb/internal/browserskill"
	apperrors "github.com/jrmatherly/ai-kb/internal/errors"
	"github.com/jrmatherly/ai-kb/internal/middleware"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
)

func TestBrowserSkillErrorStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	storeErr := fmt.Errorf("%w: %w", browserskill.ErrStore, errors.New("dial tcp 10.0.0.5:5432: connection refused"))
	for _, tc := range []struct {
		name   string
		err    error
		newErr func(string) *apperrors.AppError
		status int
		want   string
	}{
		{
			"store failure on the 503 path", storeErr, apperrors.NewServiceUnavailableError, 503,
			`{"success":false,"error":{"code":1008,"message":"browser connection status unavailable"}}`,
		},
		{
			"store failure on the 409 path", storeErr, apperrors.NewConflictError, 503,
			`{"success":false,"error":{"code":1008,"message":"browser connection status unavailable"}}`,
		},
		{
			"authorization is 403 on the 409 path", browserskill.ErrAuthorization, apperrors.NewConflictError, 403,
			`{"success":false,"error":{"code":1002,"message":"browser authorization is invalid or expired"}}`,
		},
		{
			"upstream failure is 502", fmt.Errorf("%w: BrowserSkill daemon unavailable", browserskill.ErrUpstream),
			apperrors.NewServiceUnavailableError, 502,
			`{"success":false,"error":{"code":1011,` +
				`"message":"browser upstream failed: BrowserSkill daemon unavailable"}}`,
		},
		{
			"daemon RPC error is 502", &browserskill.RPCError{Code: "not_found", Message: "fixture ref missing"},
			apperrors.NewConflictError, 502,
			`{"success":false,"error":{"code":1011,"message":"not_found: fixture ref missing"}}`,
		},
		{
			"other errors keep the call site's status", errors.New("browser disconnected; wait for reconnection"),
			apperrors.NewConflictError, 409,
			`{"success":false,"error":{"code":1005,"message":"browser disconnected; wait for reconnection"}}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.Use(middleware.ErrorHandler())
			r.GET("/", func(c *gin.Context) { _ = c.Error(browserSkillError(c.Request.Context(), tc.err, tc.newErr)) })
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
			require.Equal(t, tc.status, w.Code, w.Body.String())
			require.JSONEq(t, tc.want, w.Body.String())
		})
	}
}

type browserSkillTestSessions struct{ interfaces.SessionService }

func (browserSkillTestSessions) GetOwnedSession(context.Context, string) (*types.Session, error) {
	return &types.Session{ID: "s1", TenantID: 7}, nil
}

func TestBrowserSkillControlHidesStoreWriteFailure(t *testing.T) {
	t.Setenv("BROWSERSKILL_BINARY", "/configured/bsk")
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(
		sqlite.Open(filepath.Join(t.TempDir(), "browser.db")),
		&gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)},
	)
	require.NoError(t, err)
	migration, err := os.ReadFile("../../../migrations/sqlite/000014_browser_authorization.up.sql")
	require.NoError(t, err)
	for _, stmt := range []string{
		"CREATE TABLE users (id TEXT, tenant_id INTEGER, is_active BOOLEAN, can_access_all_tenants BOOLEAN, " +
			"deleted_at DATETIME)",
		"CREATE TABLE tenant_members (user_id TEXT, tenant_id INTEGER, status TEXT, deleted_at DATETIME)",
		"INSERT INTO users VALUES ('alice', 7, 1, 0, NULL)",
		"INSERT INTO tenant_members VALUES ('alice', 7, 'active', NULL)",
		string(migration),
		// The stop action with no connected device only writes this table.
		"DROP TABLE browser_task_interruptions",
	} {
		require.NoError(t, db.Exec(stmt).Error)
	}
	h := &Handler{
		browserSkill:   browserskill.NewManager(browserskill.NewStore(db)),
		sessionService: browserSkillTestSessions{},
	}
	r := gin.New()
	r.Use(middleware.ErrorHandler(), func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(7))
		c.Request = c.Request.WithContext(context.WithValue(ctx, types.UserIDContextKey, "alice"))
	})
	r.POST("/sessions/:session_id/local-browser", h.BrowserSkillConnection)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/sessions/s1/local-browser", strings.NewReader(`{"action":"stop"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
	require.JSONEq(t,
		`{"success":false,"error":{"code":1008,"message":"browser connection status unavailable"}}`, w.Body.String())
}

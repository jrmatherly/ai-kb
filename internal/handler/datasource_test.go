package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jrmatherly/ai-kb/internal/middleware"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
)

type stubDataSourceService struct {
	interfaces.DataSourceService
	getSyncLogs   func(ctx context.Context, dsID string, limit int, offset int) ([]*types.SyncLog, error)
	getDataSource func(ctx context.Context, id string) (*types.DataSource, error)
	manualSyncs   []bool
	listForConfig func(ctx context.Context, config *types.DataSourceConfig, parentID string) ([]types.Resource, error)
}

func (s *stubDataSourceService) ListResourcesForConfig(
	ctx context.Context, config *types.DataSourceConfig, parentID string,
) ([]types.Resource, error) {
	return s.listForConfig(ctx, config, parentID)
}

func (s *stubDataSourceService) ManualSync(
	_ context.Context, dsID string, allowMassDeletion bool,
) (*types.SyncLog, error) {
	s.manualSyncs = append(s.manualSyncs, allowMassDeletion)
	return &types.SyncLog{ID: "log-" + dsID, DataSourceID: dsID}, nil
}

func (s *stubDataSourceService) GetSyncLogs(ctx context.Context, dsID string, limit int, offset int) ([]*types.SyncLog, error) {
	if s.getSyncLogs != nil {
		return s.getSyncLogs(ctx, dsID, limit, offset)
	}
	return nil, nil
}

func (s *stubDataSourceService) GetDataSource(ctx context.Context, id string) (*types.DataSource, error) {
	if s.getDataSource != nil {
		return s.getDataSource(ctx, id)
	}
	return nil, nil
}

type stubKBServiceForDS struct {
	interfaces.KnowledgeBaseService
	getByID func(ctx context.Context, id string) (*types.KnowledgeBase, error)
}

func (s *stubKBServiceForDS) GetKnowledgeBaseByID(ctx context.Context, id string) (*types.KnowledgeBase, error) {
	if s.getByID != nil {
		return s.getByID(ctx, id)
	}
	return nil, nil
}

func newDataSourceTestRouter(h *DataSourceHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(func(c *gin.Context) {
		if tenantID, ok := c.Request.Context().Value(types.TenantIDContextKey).(uint64); ok {
			c.Set(types.TenantIDContextKey.String(), tenantID)
		}
		c.Next()
	})
	r.GET("/datasource/:id/logs", h.GetSyncLogs)
	r.POST("/datasource/:id/sync", h.ManualSync)
	r.POST("/datasource/list-resources", h.ListResourcesForConfig)
	return r
}

func TestDataSource_ManualSync_PassesAllowMassDeletion(t *testing.T) {
	cases := []struct {
		body     string
		wantCode int
		want     []bool
	}{
		{body: "", wantCode: http.StatusOK, want: []bool{false}},
		{body: `{}`, wantCode: http.StatusOK, want: []bool{false}},
		{body: `{"allow_mass_deletion":true}`, wantCode: http.StatusOK, want: []bool{true}},
		{body: `{"allow_mass_deletion":"yes"}`, wantCode: http.StatusBadRequest},
	}
	for _, c := range cases {
		dsSvc := &stubDataSourceService{
			getDataSource: func(_ context.Context, id string) (*types.DataSource, error) {
				return &types.DataSource{ID: id, KnowledgeBaseID: "kb1"}, nil
			},
		}
		kbSvc := &stubKBServiceForDS{
			getByID: func(_ context.Context, _ string) (*types.KnowledgeBase, error) {
				return &types.KnowledgeBase{ID: "kb1", TenantID: 1}, nil
			},
		}
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/datasource/ds1/sync", strings.NewReader(c.body))
		req.Header.Set("Content-Type", "application/json")
		newDataSourceTestRouter(NewDataSourceHandler(dsSvc, kbSvc)).ServeHTTP(w, withDSCtx(req, 1))

		if w.Code != c.wantCode {
			t.Fatalf("body %q: status %d, want %d (%s)", c.body, w.Code, c.wantCode, w.Body.String())
		}
		if !slices.Equal(dsSvc.manualSyncs, c.want) {
			t.Fatalf("body %q: ManualSync calls %v, want %v", c.body, dsSvc.manualSyncs, c.want)
		}
	}
}

func withDSCtx(req *http.Request, tenantID uint64) *http.Request {
	ctx := req.Context()
	ctx = context.WithValue(ctx, types.TenantIDContextKey, tenantID)
	return req.WithContext(ctx)
}

func TestDataSource_GetSyncLogs_ValidLimitWithinBounds(t *testing.T) {
	var capturedLimit, capturedOffset int
	dsSvc := &stubDataSourceService{
		getDataSource: func(_ context.Context, id string) (*types.DataSource, error) {
			return &types.DataSource{ID: id, KnowledgeBaseID: "kb1"}, nil
		},
		getSyncLogs: func(_ context.Context, _ string, limit int, offset int) ([]*types.SyncLog, error) {
			capturedLimit = limit
			capturedOffset = offset
			return []*types.SyncLog{
				{ID: "log1", DataSourceID: "ds1"},
			}, nil
		},
	}
	kbSvc := &stubKBServiceForDS{
		getByID: func(_ context.Context, _ string) (*types.KnowledgeBase, error) {
			return &types.KnowledgeBase{ID: "kb1", TenantID: 1}, nil
		},
	}
	h := NewDataSourceHandler(dsSvc, kbSvc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/datasource/ds1/logs?limit=50&offset=25", nil)
	req = withDSCtx(req, 1)
	newDataSourceTestRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if capturedLimit != 50 {
		t.Fatalf("expected limit=50, got %d", capturedLimit)
	}
	if capturedOffset != 25 {
		t.Fatalf("expected offset=25, got %d", capturedOffset)
	}
}

func TestDataSource_GetSyncLogs_LimitExceedingMaximum(t *testing.T) {
	dsSvc := &stubDataSourceService{
		getDataSource: func(_ context.Context, id string) (*types.DataSource, error) {
			return &types.DataSource{ID: id, KnowledgeBaseID: "kb1"}, nil
		},
		getSyncLogs: func(_ context.Context, _ string, _ int, _ int) ([]*types.SyncLog, error) {
			t.Fatalf("service must not be called when limit exceeds maximum")
			return nil, nil
		},
	}
	kbSvc := &stubKBServiceForDS{
		getByID: func(_ context.Context, _ string) (*types.KnowledgeBase, error) {
			return &types.KnowledgeBase{ID: "kb1", TenantID: 1}, nil
		},
	}
	h := NewDataSourceHandler(dsSvc, kbSvc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/datasource/ds1/logs?limit=999", nil)
	req = withDSCtx(req, 1)
	newDataSourceTestRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for limit > 100, got %d body=%s", w.Code, w.Body.String())
	}
	want := `{"success":false,"error":{"code":1000,"message":"limit must be between 1 and 100"}}`
	if got := w.Body.String(); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestDataSource_GetSyncLogs_ForeignDataSourceIsForbidden(t *testing.T) {
	dsSvc := &stubDataSourceService{
		getDataSource: func(_ context.Context, id string) (*types.DataSource, error) {
			return &types.DataSource{ID: id, KnowledgeBaseID: "kb1"}, nil
		},
	}
	kbSvc := &stubKBServiceForDS{
		getByID: func(_ context.Context, _ string) (*types.KnowledgeBase, error) {
			return &types.KnowledgeBase{ID: "kb1", TenantID: 2}, nil
		},
	}
	h := NewDataSourceHandler(dsSvc, kbSvc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/datasource/ds1/logs", nil)
	req = withDSCtx(req, 1)
	newDataSourceTestRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", w.Code, w.Body.String())
	}
	want := `{"success":false,"error":{"code":1002,"message":"access denied"}}`
	if got := w.Body.String(); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestDataSource_GetSyncLogs_MissingLimitDefaultsCorrectly(t *testing.T) {
	var capturedLimit, capturedOffset int
	dsSvc := &stubDataSourceService{
		getDataSource: func(_ context.Context, id string) (*types.DataSource, error) {
			return &types.DataSource{ID: id, KnowledgeBaseID: "kb1"}, nil
		},
		getSyncLogs: func(_ context.Context, _ string, limit int, offset int) ([]*types.SyncLog, error) {
			capturedLimit = limit
			capturedOffset = offset
			return []*types.SyncLog{}, nil
		},
	}
	kbSvc := &stubKBServiceForDS{
		getByID: func(_ context.Context, _ string) (*types.KnowledgeBase, error) {
			return &types.KnowledgeBase{ID: "kb1", TenantID: 1}, nil
		},
	}
	h := NewDataSourceHandler(dsSvc, kbSvc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/datasource/ds1/logs", nil)
	req = withDSCtx(req, 1)
	newDataSourceTestRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if capturedLimit != 10 {
		t.Fatalf("expected default limit=10, got %d", capturedLimit)
	}
	if capturedOffset != 0 {
		t.Fatalf("expected default offset=0 (page 1), got %d", capturedOffset)
	}
}

func TestDataSource_GetSyncLogs_NonNumericLimitRejected(t *testing.T) {
	dsSvc := &stubDataSourceService{
		getDataSource: func(_ context.Context, id string) (*types.DataSource, error) {
			return &types.DataSource{ID: id, KnowledgeBaseID: "kb1"}, nil
		},
		getSyncLogs: func(_ context.Context, _ string, _ int, _ int) ([]*types.SyncLog, error) {
			t.Fatalf("service must not be called with non-numeric limit")
			return nil, nil
		},
	}
	kbSvc := &stubKBServiceForDS{
		getByID: func(_ context.Context, _ string) (*types.KnowledgeBase, error) {
			return &types.KnowledgeBase{ID: "kb1", TenantID: 1}, nil
		},
	}
	h := NewDataSourceHandler(dsSvc, kbSvc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/datasource/ds1/logs?limit=abc", nil)
	req = withDSCtx(req, 1)
	newDataSourceTestRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-numeric limit, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestDataSource_GetSyncLogs_ZeroLimitRejected(t *testing.T) {
	dsSvc := &stubDataSourceService{
		getDataSource: func(_ context.Context, id string) (*types.DataSource, error) {
			return &types.DataSource{ID: id, KnowledgeBaseID: "kb1"}, nil
		},
		getSyncLogs: func(_ context.Context, _ string, _ int, _ int) ([]*types.SyncLog, error) {
			t.Fatalf("service must not be called with limit=0")
			return nil, nil
		},
	}
	kbSvc := &stubKBServiceForDS{
		getByID: func(_ context.Context, _ string) (*types.KnowledgeBase, error) {
			return &types.KnowledgeBase{ID: "kb1", TenantID: 1}, nil
		},
	}
	h := NewDataSourceHandler(dsSvc, kbSvc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/datasource/ds1/logs?limit=0", nil)
	req = withDSCtx(req, 1)
	newDataSourceTestRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for limit=0, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestDataSource_GetSyncLogs_NegativeLimitRejected(t *testing.T) {
	dsSvc := &stubDataSourceService{
		getDataSource: func(_ context.Context, id string) (*types.DataSource, error) {
			return &types.DataSource{ID: id, KnowledgeBaseID: "kb1"}, nil
		},
		getSyncLogs: func(_ context.Context, _ string, _ int, _ int) ([]*types.SyncLog, error) {
			t.Fatalf("service must not be called with negative limit")
			return nil, nil
		},
	}
	kbSvc := &stubKBServiceForDS{
		getByID: func(_ context.Context, _ string) (*types.KnowledgeBase, error) {
			return &types.KnowledgeBase{ID: "kb1", TenantID: 1}, nil
		},
	}
	h := NewDataSourceHandler(dsSvc, kbSvc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/datasource/ds1/logs?limit=-5", nil)
	req = withDSCtx(req, 1)
	newDataSourceTestRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for negative limit, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestDataSource_ListResourcesForConfig(t *testing.T) {
	var gotConfig *types.DataSourceConfig
	var gotParent string
	dsSvc := &stubDataSourceService{
		listForConfig: func(
			_ context.Context, config *types.DataSourceConfig, parentID string,
		) ([]types.Resource, error) {
			gotConfig, gotParent = config, parentID
			if parentID == "boom" {
				return nil, errors.New("upstream refused")
			}
			if parentID == "leaf" {
				return nil, nil
			}
			return []types.Resource{{ExternalID: "feed-1", Name: "Feed"}}, nil
		},
	}
	router := newDataSourceTestRouter(NewDataSourceHandler(dsSvc, &stubKBServiceForDS{}))
	post := func(body string, tenantID uint64) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/datasource/list-resources", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, withDSCtx(req, tenantID))
		return w
	}

	w := post(`{"type":"rss","credentials":{"auth_headers":"X: y"},`+
		`"settings":{"feed_urls":"https://example.com/feed"},"resource_ids":["root"],"parent_id":"p1"}`, 1)
	wantBody := `[{"external_id":"feed-1","name":"Feed","type":"","description":"","url":"",` +
		`"modified_at":"0001-01-01T00:00:00Z"}]`
	if w.Code != http.StatusOK || w.Body.String() != wantBody {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if gotParent != "p1" || gotConfig.Type != "rss" || gotConfig.Credentials["auth_headers"] != "X: y" ||
		gotConfig.Settings["feed_urls"] != "https://example.com/feed" ||
		!slices.Equal(gotConfig.ResourceIDs, []string{"root"}) {
		t.Fatalf("service got config %+v parent %q", gotConfig, gotParent)
	}

	w = post(`{"type":"rss","credentials":{},"parent_id":"leaf"}`, 1)
	if w.Code != http.StatusOK || w.Body.String() != `[]` {
		t.Fatalf("empty listing: status %d body %s", w.Code, w.Body.String())
	}

	for _, c := range []struct {
		name, body string
		tenantID   uint64
		wantCode   int
	}{
		{"missing type", `{"credentials":{}}`, 1, http.StatusBadRequest},
		{"missing credentials", `{"type":"rss"}`, 1, http.StatusBadRequest},
		{"connector error", `{"type":"rss","credentials":{},"parent_id":"boom"}`, 1, http.StatusBadRequest},
		{"no tenant", `{"type":"rss","credentials":{}}`, 0, http.StatusUnauthorized},
	} {
		if w := post(c.body, c.tenantID); w.Code != c.wantCode {
			t.Fatalf("%s: status %d, want %d (%s)", c.name, w.Code, c.wantCode, w.Body.String())
		}
	}
}

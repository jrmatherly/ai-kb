package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jrmatherly/ai-kb/internal/application/service"
	"github.com/jrmatherly/ai-kb/internal/datasource"
	"github.com/jrmatherly/ai-kb/internal/types"
)

// failingConnector fails ListResources with the error keyed by parentID.
type failingConnector struct {
	datasource.Connector
	errs map[string]error
}

func (failingConnector) Type() string { return "fake" }

func (c failingConnector) ListResources(_ context.Context, _ *types.DataSourceConfig, parentID string) (
	[]types.Resource, error,
) {
	return nil, c.errs[parentID]
}

func TestListResourcesClassifiesConnectorFailures(t *testing.T) {
	registry := datasource.NewConnectorRegistry()
	require.NoError(t, registry.Register(failingConnector{errs: map[string]error{
		"forbidden": errors.New(`feishu api error: status=403 body={"code":1061004}`),
		"secret":    fmt.Errorf("%w: client_secret is required", datasource.ErrInvalidCredentials),
		"missing":   errors.New("list wiki nodes error: code=1061003 msg=folder missing"),
		"timeout":   errors.New("dial tcp 10.0.0.1:443: i/o timeout"),
	}}))
	svc := service.NewDataSourceService(nil, nil, nil, nil, nil, registry, nil, nil, nil, nil)
	r := newDataSourceTestRouter(NewDataSourceHandler(svc, nil))

	for _, tc := range []struct {
		name, body string
		status     int
		want       string
	}{
		{
			"permission", `{"type":"fake","credentials":{},"parent_id":"forbidden"}`, http.StatusBadGateway,
			`{"success":false,"error":{"code":1011,"message":"failed to list resources: feishu api error: ` +
				`status=403 body={\"code\":1061004}","details":{"code":"permission_denied"}}}`,
		},
		{
			"credentials", `{"type":"fake","credentials":{},"parent_id":"secret"}`, http.StatusBadRequest,
			`{"success":false,"error":{"code":1000,"message":"invalid credentials: client_secret is required",` +
				`"details":{"code":"auth_failed"}}}`,
		},
		{
			"not found", `{"type":"fake","credentials":{},"parent_id":"missing"}`, http.StatusBadGateway,
			`{"success":false,"error":{"code":1011,"message":"failed to list resources: list wiki nodes error: ` +
				`code=1061003 msg=folder missing","details":{"code":"not_found"}}}`,
		},
		{
			"unclassified", `{"type":"fake","credentials":{},"parent_id":"timeout"}`, http.StatusBadGateway,
			`{"success":false,"error":{"code":1011,"message":"failed to list resources: dial tcp 10.0.0.1:443: ` +
				`i/o timeout","details":{"code":"upstream_error"}}}`,
		},
		{
			"unknown connector", `{"type":"nope","credentials":{}}`, http.StatusBadRequest,
			`{"success":false,"error":{"code":1000,"message":"connector type not found in registry"}}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/datasource/list-resources", strings.NewReader(tc.body))
			req = req.WithContext(context.WithValue(req.Context(), types.TenantIDContextKey, uint64(1)))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			require.Equal(t, tc.status, w.Code, w.Body.String())
			require.JSONEq(t, tc.want, w.Body.String())
		})
	}
}

func TestGetSyncLogsRepositoryFailureIsScrubbed500(t *testing.T) {
	svc := &stubDataSourceService{
		getDataSource: func(context.Context, string) (*types.DataSource, error) {
			return &types.DataSource{ID: "ds-1", KnowledgeBaseID: "kb-1"}, nil
		},
		getSyncLogs: func(context.Context, string, int, int) ([]*types.SyncLog, error) {
			return nil, errors.New(`SQLSTATE 42P01: relation "sync_logs" does not exist`)
		},
	}
	kb := &stubKBServiceForDS{getByID: func(context.Context, string) (*types.KnowledgeBase, error) {
		return &types.KnowledgeBase{ID: "kb-1", TenantID: 1}, nil
	}}
	r := newDataSourceTestRouter(NewDataSourceHandler(svc, kb))
	req := httptest.NewRequest(http.MethodGet, "/datasource/ds-1/logs", nil)
	req = req.WithContext(context.WithValue(req.Context(), types.TenantIDContextKey, uint64(1)))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.JSONEq(t, `{"success":false,"error":{"code":1007,"message":"Internal server error"}}`, w.Body.String())
}

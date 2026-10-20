package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type idpSettings struct {
	interfaces.SystemSettingService
	mapping []string
	calls   int
}

func (s *idpSettings) GetString(_ context.Context, _, _ string, def string) string {
	s.calls++
	return def
}

func (s *idpSettings) GetStringList(_ context.Context, key, _ string, def []string) []string {
	s.calls++
	if key == "oidc.group_role_map" {
		return s.mapping
	}
	return def
}

type idpTenants struct {
	interfaces.TenantService
}

func (idpTenants) GetTenantBySlug(_ context.Context, slug string) (*types.Tenant, error) {
	if slug == "home" {
		return &types.Tenant{ID: 10}, nil
	}
	return nil, nil
}

func managedFlag(t *testing.T, settings *idpSettings, tenantID uint64) bool {
	t.Helper()
	ms := &stubMemberService{
		listTenant: func(context.Context, uint64) ([]*types.TenantMember, error) { return nil, nil },
	}
	h := NewTenantMemberHandler(ms, &stubMemberUserService{}, settings, idpTenants{})
	path := fmt.Sprintf("/tenants/%d/members", tenantID)
	opts := memberCtxOpts{callerID: "caller", tenantID: tenantID}
	w := doJSONWithCtx(t, memberTestRouter(h), http.MethodGet, path, nil, opts)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body struct {
		Data struct {
			ManagedByIdp bool `json:"managed_by_idp"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body.Data.ManagedByIdp
}

func TestListMembers_ManagedByIdp(t *testing.T) {
	settings := &idpSettings{mapping: []string{"eng:home:admin"}}
	assert.True(t, managedFlag(t, settings, 10))
	assert.False(t, managedFlag(t, settings, 11))

	off := &idpSettings{}
	assert.False(t, managedFlag(t, off, 10))
}

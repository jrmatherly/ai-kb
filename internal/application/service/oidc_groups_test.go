package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func one(group, workspace string, role types.TenantRole) []types.OIDCGroupMapping {
	return []types.OIDCGroupMapping{{Group: group, Workspace: workspace, Role: role}}
}

func TestParseOIDCGroupMap(t *testing.T) {
	for _, tc := range []struct {
		name    string
		entries []string
		want    []types.OIDCGroupMapping
		wantErr string
	}{
		{name: "slug", entries: []string{"eng:home:admin"}, want: one("eng", "home", types.TenantRoleAdmin)},
		{name: "numeric id", entries: []string{"eng:42:viewer"}, want: one("eng", "42", types.TenantRoleViewer)},
		{name: "trims", entries: []string{" eng : home : owner "}, want: one("eng", "home", types.TenantRoleOwner)},
		{name: "empty list", entries: nil, want: nil},
		{name: "two parts", entries: []string{"eng:home"}, wantErr: "expects group:workspace:role"},
		{
			name: "colon in the group", entries: []string{"org:team:home:admin"},
			want: one("org:team", "home", types.TenantRoleAdmin),
		},
		{name: "colon in the group with a bad role", entries: []string{"a:b:c:d"}, wantErr: "unknown role"},
		{name: "unknown role", entries: []string{"eng:home:root"}, wantErr: "unknown role"},
		{name: "invalid workspace", entries: []string{"eng:Home:admin"}, wantErr: "invalid workspace"},
		{name: "zero id", entries: []string{"eng:0:admin"}, wantErr: "invalid workspace"},
		{name: "empty group", entries: []string{":home:admin"}, wantErr: "expects group:workspace:role"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseOIDCGroupMap(tc.entries)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestSystemSettingUpdate_RejectsBadGroupRoleMap(t *testing.T) {
	svc := &systemSettingService{repo: &registrationSettingRepo{}, cache: make(map[string]*types.SystemSetting)}
	ctx := context.WithValue(context.Background(), types.UserIDContextKey, "system-admin")
	_, err := svc.Update(ctx, "oidc.group_role_map", []string{"x:y"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expects group:workspace:role")
	_, err = svc.Update(ctx, "oidc.group_role_map", []string{"eng:home:admin"})
	require.NoError(t, err)
}

// oidcSettingRepo serves one stored row per key.
type oidcSettingRepo struct {
	interfaces.SystemSettingRepository
	rows map[string]string
}

func (r *oidcSettingRepo) Get(_ context.Context, key string) (*types.SystemSetting, error) {
	v, ok := r.rows[key]
	if !ok {
		return nil, nil
	}
	return &types.SystemSetting{Key: key, Value: types.JSON(v)}, nil
}

func TestLoadOIDCGroupSettings_ReadsDBThenEnv(t *testing.T) {
	claim, _ := json.Marshal("roles")
	repo := &oidcSettingRepo{rows: map[string]string{"oidc.groups_claim": string(claim)}}
	svc := &systemSettingService{repo: repo, cache: make(map[string]*types.SystemSetting)}
	t.Setenv("OIDC_AUTH_GROUPS_CLAIM", "memberOf")
	t.Setenv("OIDC_AUTH_SYSTEM_ADMIN_GROUPS", "")
	t.Setenv("OIDC_AUTH_GROUP_ROLE_MAP", "")
	got, err := LoadOIDCGroupSettings(context.Background(), svc)
	require.NoError(t, err)
	assert.Equal(t, "roles", got.Claim, "a stored row wins over the environment")
	assert.False(t, got.Enabled(), "off while both lists are empty")

	t.Setenv("OIDC_AUTH_GROUP_ROLE_MAP", "eng:home:admin, ops:42:viewer")
	t.Setenv("OIDC_AUTH_SYSTEM_ADMIN_GROUPS", "cluster_admins")
	got, err = LoadOIDCGroupSettings(context.Background(), svc)
	require.NoError(t, err)
	assert.True(t, got.Enabled())
	assert.Equal(t, []string{"cluster_admins"}, got.AdminGroups)
	assert.Equal(t, []types.OIDCGroupMapping{
		{Group: "eng", Workspace: "home", Role: types.TenantRoleAdmin},
		{Group: "ops", Workspace: "42", Role: types.TenantRoleViewer},
	}, got.Map)
}

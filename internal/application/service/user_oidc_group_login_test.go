package service

import (
	"context"
	"errors"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loginUserRepo holds one user and serves every read from its current row, so a stale copy is visible.
type loginUserRepo struct {
	interfaces.UserRepository
	user    *types.User
	updates int
}

func (r *loginUserRepo) GetUserByEmail(_ context.Context, email string) (*types.User, error) {
	if r.user != nil && r.user.Email == email {
		copied := *r.user
		return &copied, nil
	}
	return nil, errors.New("user not found")
}

func (r *loginUserRepo) GetUserByID(_ context.Context, id string) (*types.User, error) {
	if r.user != nil && r.user.ID == id {
		copied := *r.user
		return &copied, nil
	}
	return nil, errors.New("user not found")
}

func (r *loginUserRepo) UpdateUser(_ context.Context, user *types.User) error {
	r.updates++
	copied := *user
	r.user = &copied
	return nil
}

func (r *loginUserRepo) GrantSystemAdmin(context.Context, string) error {
	r.user.IsSystemAdmin = true
	return nil
}

func (r *loginUserRepo) RevokeSystemAdmin(context.Context, string, string) (*types.User, error) {
	r.user.IsSystemAdmin = false
	copied := *r.user
	return &copied, nil
}

type loginSettings struct {
	interfaces.SystemSettingService
	values map[string][]string
}

func (s *loginSettings) GetString(_ context.Context, _, _ string, def string) string { return def }

func (s *loginSettings) GetStringList(_ context.Context, key, _ string, def []string) []string {
	if v, ok := s.values[key]; ok {
		return v
	}
	return def
}

type loginMemberService struct {
	syncMemberService
	memberships map[uint64]*types.TenantMember
}

func (s *loginMemberService) GetMembership(_ context.Context, _ string, tenantID uint64) (*types.TenantMember, error) {
	if m, ok := s.memberships[tenantID]; ok {
		return m, nil
	}
	return nil, errors.New("not a member")
}

func (s *loginMemberService) AddMember(
	ctx context.Context, userID string, tenantID uint64, role types.TenantRole, invitedBy *string,
) (*types.TenantMember, error) {
	m, err := s.syncMemberService.AddMember(ctx, userID, tenantID, role, invitedBy)
	if err == nil {
		m.Status = types.TenantMemberStatusActive
		s.memberships[tenantID] = m
		s.members = append(s.members, m)
	}
	return m, err
}

func loginFixture(
	t *testing.T, user *types.User, mapEntries []string,
) (*userService, *loginUserRepo, *loginMemberService) {
	t.Setenv("JWT_SECRET", "test-jwt-secret-for-oidc-group-login-tests")
	repo := &loginUserRepo{user: user}
	members := &loginMemberService{
		syncMemberService: syncMemberService{t: t}, memberships: map[uint64]*types.TenantMember{},
	}
	settings := &loginSettings{values: map[string][]string{"oidc.group_role_map": mapEntries}}
	svc := &userService{
		userRepo:  repo,
		tokenRepo: &stubAuthTokenRepo{tokens: map[string]*types.AuthToken{}},
		tenantService: &syncTenantService{
			bySlug: map[string]uint64{"home": 20}, byID: map[uint64]bool{20: true},
		},
		memberService:    members,
		systemSettingSvc: settings,
	}
	return svc, repo, members
}

const authorsAdmin = "authors:home:admin"

var activeUser = func() *types.User { return &types.User{ID: "u1", Email: "a@x", IsActive: true} }

func TestCompleteOIDCLogin_RunsGroupSyncBeforeTenantResolution(t *testing.T) {
	svc, _, members := loginFixture(t, activeUser(), []string{authorsAdmin})
	info := &types.OIDCUserInfo{Email: "a@x", Claims: map[string]any{"groups": []any{"authors"}}}
	resp, err := svc.completeOIDCLogin(context.Background(), info, types.TenantProvisioningTenantless)
	require.NoError(t, err)
	require.True(t, resp.Success)
	assert.Equal(t, []memberCall{{"add", 20, types.TenantRoleAdmin}}, members.calls)
	require.NotNil(t, resp.Tenant)
	assert.Equal(t, uint64(20), resp.Tenant.ID)
	require.NotNil(t, resp.GroupSync)
	assert.Equal(t, []uint64{20}, resp.GroupSync.Added)
}

func TestCompleteOIDCLogin_NoGroupsClaimLeavesMembershipsAlone(t *testing.T) {
	svc, _, members := loginFixture(t, activeUser(), []string{authorsAdmin})
	info := &types.OIDCUserInfo{Email: "a@x", Claims: map[string]any{"email": "a@x"}}
	resp, err := svc.completeOIDCLogin(context.Background(), info, types.TenantProvisioningTenantless)
	require.NoError(t, err)
	assert.Empty(t, members.calls)
	assert.Nil(t, resp.GroupSync)
}

func TestCompleteOIDCLogin_DisabledUserIsNotSynced(t *testing.T) {
	disabled := &types.User{ID: "u1", Email: "a@x", IsActive: false}
	svc, repo, members := loginFixture(t, disabled, []string{authorsAdmin})
	info := &types.OIDCUserInfo{Email: "a@x", Claims: map[string]any{"groups": []any{"authors"}}}
	resp, err := svc.completeOIDCLogin(context.Background(), info, types.TenantProvisioningTenantless)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Empty(t, members.calls)
	assert.Zero(t, repo.updates)
}

func TestCompleteOIDCLogin_RevokedAdminStaysRevokedAfterMemberError(t *testing.T) {
	user := &types.User{ID: "u1", Email: "a@x", IsActive: true, IsSystemAdmin: true, TenantID: 20}
	svc, repo, members := loginFixture(t, user, []string{authorsAdmin})
	svc.systemSettingSvc.(*loginSettings).values["oidc.system_admin_groups"] = []string{"admins"}
	members.addErr = errors.New("db down")
	info := &types.OIDCUserInfo{Email: "a@x", Claims: map[string]any{"groups": []any{"authors"}}}
	resp, err := svc.completeOIDCLogin(context.Background(), info, types.TenantProvisioningTenantless)
	require.NoError(t, err)
	require.True(t, resp.Success)
	assert.False(t, resp.User.IsSystemAdmin)
	assert.False(t, repo.user.IsSystemAdmin, "a stale struct save must not bring the flag back")
}

func TestCompleteOIDCLogin_SyncErrorDoesNotFailLogin(t *testing.T) {
	svc, _, members := loginFixture(t, activeUser(), []string{authorsAdmin})
	members.addErr = errors.New("db down")
	info := &types.OIDCUserInfo{Email: "a@x", Claims: map[string]any{"groups": []any{"authors"}}}
	resp, err := svc.completeOIDCLogin(context.Background(), info, types.TenantProvisioningTenantless)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.NotEmpty(t, resp.Token)
	require.NotNil(t, resp.GroupSync)
	assert.Contains(t, resp.GroupSync.Skipped, "error", "the audit row must read partial")
}

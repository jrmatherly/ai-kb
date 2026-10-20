package service

import (
	"context"
	"errors"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/application/repository"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type syncUserRepo struct {
	interfaces.UserRepository
	granted   []string
	revoked   [][2]string
	revokeErr error
}

func (r *syncUserRepo) GrantSystemAdmin(_ context.Context, userID string) error {
	r.granted = append(r.granted, userID)
	return nil
}

func (r *syncUserRepo) RevokeSystemAdmin(_ context.Context, userID, actorID string) (*types.User, error) {
	r.revoked = append(r.revoked, [2]string{userID, actorID})
	if r.revokeErr != nil {
		return nil, r.revokeErr
	}
	return &types.User{ID: userID}, nil
}

type syncTenantService struct {
	interfaces.TenantService
	bySlug  map[string]uint64
	byID    map[uint64]bool
	calls   int
	slugErr error
}

func (s *syncTenantService) GetTenantBySlug(_ context.Context, slug string) (*types.Tenant, error) {
	s.calls++
	if s.slugErr != nil {
		return nil, s.slugErr
	}
	if id, ok := s.bySlug[slug]; ok {
		return &types.Tenant{ID: id}, nil
	}
	return nil, nil
}

func (s *syncTenantService) GetTenantByID(_ context.Context, id uint64) (*types.Tenant, error) {
	s.calls++
	if s.byID[id] {
		return &types.Tenant{ID: id}, nil
	}
	return nil, repository.ErrTenantNotFound
}

type memberCall struct {
	op     string
	tenant uint64
	role   types.TenantRole
}

type syncMemberService struct {
	interfaces.TenantMemberService
	t         *testing.T
	members   []*types.TenantMember
	calls     []memberCall
	addErr    error
	updateErr error
	removeErr error
}

func (s *syncMemberService) noActor(ctx context.Context) {
	if actor, ok := types.UserIDFromContext(ctx); ok && actor != "" {
		s.t.Errorf("member call carried actor %q", actor)
	}
}

func (s *syncMemberService) ListByUser(context.Context, string) ([]*types.TenantMember, error) {
	return s.members, nil
}

func (s *syncMemberService) AddMember(
	ctx context.Context, _ string, tenantID uint64, role types.TenantRole, _ *string,
) (*types.TenantMember, error) {
	s.noActor(ctx)
	s.calls = append(s.calls, memberCall{"add", tenantID, role})
	if s.addErr != nil {
		return nil, s.addErr
	}
	return &types.TenantMember{TenantID: tenantID, Role: role}, nil
}

func (s *syncMemberService) UpdateRole(ctx context.Context, _ string, tenantID uint64, role types.TenantRole) error {
	s.noActor(ctx)
	s.calls = append(s.calls, memberCall{"update", tenantID, role})
	return s.updateErr
}

func (s *syncMemberService) RemoveMember(ctx context.Context, _ string, tenantID uint64) error {
	s.noActor(ctx)
	s.calls = append(s.calls, memberCall{"remove", tenantID, ""})
	return s.removeErr
}

func member(tenant uint64, role types.TenantRole) *types.TenantMember {
	return &types.TenantMember{UserID: "u1", TenantID: tenant, Role: role, Status: types.TenantMemberStatusActive}
}

func syncFixture(t *testing.T) (*userService, *syncUserRepo, *syncTenantService, *syncMemberService) {
	repo := &syncUserRepo{}
	tenants := &syncTenantService{bySlug: map[string]uint64{"home": 10, "ops": 11}, byID: map[uint64]bool{20: true}}
	members := &syncMemberService{t: t}
	return &userService{userRepo: repo, tenantService: tenants, memberService: members}, repo, tenants, members
}

func claims(groups ...string) map[string]any {
	list := make([]any, 0, len(groups))
	for _, g := range groups {
		list = append(list, g)
	}
	return map[string]any{"groups": list}
}

var mapped = types.OIDCGroupSettings{Claim: "groups", Map: mapping}

var mapping = []types.OIDCGroupMapping{
	{Group: "eng", Workspace: "home", Role: types.TenantRoleAdmin},
	{Group: "ops", Workspace: "ops", Role: types.TenantRoleViewer},
	{Group: "ops", Workspace: "20", Role: types.TenantRoleContributor},
	{Group: "leads", Workspace: "home", Role: types.TenantRoleOwner},
}

func TestGroupsFromClaim(t *testing.T) {
	g, present := groupsFromClaim(map[string]any{"groups": []any{"a", 1, "b"}}, "groups")
	assert.True(t, present)
	assert.Equal(t, []string{"a", "b"}, g)
	g, present = groupsFromClaim(map[string]any{"groups": "solo"}, "groups")
	assert.True(t, present)
	assert.Equal(t, []string{"solo"}, g)
	g, present = groupsFromClaim(map[string]any{"groups": "a, b"}, "groups")
	assert.True(t, present)
	assert.Equal(t, []string{"a", "b"}, g)
	g, present = groupsFromClaim(claims(), "groups")
	assert.True(t, present)
	assert.Empty(t, g)
	_, present = groupsFromClaim(map[string]any{"roles": []any{"a"}}, "groups")
	assert.False(t, present)
}

func TestDesiredRoles_HighestRoleWins(t *testing.T) {
	resolve := func(key string) (uint64, bool, error) {
		switch key {
		case "home":
			return 10, true, nil
		case "ops":
			return 11, true, nil
		case "20":
			return 20, true, nil
		}
		return 0, false, nil
	}
	m := append(mapping,
		types.OIDCGroupMapping{Group: "x", Workspace: "gone", Role: types.TenantRoleViewer},
		types.OIDCGroupMapping{Group: "y", Workspace: "gone", Role: types.TenantRoleViewer},
	)
	roles, governed, unknown, err := desiredRoles([]string{"eng", "leads", "ops"}, m, resolve)
	require.NoError(t, err)
	assert.Equal(t, map[uint64]types.TenantRole{
		10: types.TenantRoleOwner, 11: types.TenantRoleViewer, 20: types.TenantRoleContributor,
	}, roles)
	assert.Equal(t, map[uint64]struct{}{10: {}, 11: {}, 20: {}}, governed)
	assert.Equal(t, []string{"gone"}, unknown)
}

func TestSyncOIDCGroups_AbsentClaimReturnsNil(t *testing.T) {
	svc, repo, _, members := syncFixture(t)
	res, err := svc.syncOIDCGroups(context.Background(), &types.User{ID: "u1"}, map[string]any{}, mapped)
	require.NoError(t, err)
	assert.Nil(t, res)
	assert.Empty(t, members.calls)
	assert.Empty(t, repo.granted)
	assert.Empty(t, repo.revoked)
}

func TestSyncOIDCGroups_AddsUpdatesRemovesInGovernedWorkspaces(t *testing.T) {
	svc, _, _, members := syncFixture(t)
	members.members = []*types.TenantMember{
		member(10, types.TenantRoleViewer), member(11, types.TenantRoleContributor), member(12, types.TenantRoleOwner),
	}
	claims := claims("eng", "ops")
	m := []types.OIDCGroupMapping{
		{Group: "eng", Workspace: "home", Role: types.TenantRoleAdmin},
		{Group: "ops", Workspace: "20", Role: types.TenantRoleContributor},
		{Group: "sre", Workspace: "ops", Role: types.TenantRoleViewer},
	}
	res, err := svc.syncOIDCGroups(context.Background(), &types.User{ID: "u1"}, claims,
		types.OIDCGroupSettings{Claim: "groups", Map: m})
	require.NoError(t, err)
	assert.ElementsMatch(t, []memberCall{
		{"update", 10, types.TenantRoleAdmin}, {"remove", 11, ""}, {"add", 20, types.TenantRoleContributor},
	}, members.calls)
	assert.Equal(t, []uint64{20}, res.Added)
	assert.Equal(t, []uint64{10}, res.RoleChanged)
	assert.Equal(t, []uint64{11}, res.Removed)
	assert.Equal(t, "unchanged", res.Admin)
	assert.Empty(t, res.Skipped)
}

func TestSyncOIDCGroups_LastOwnerKept(t *testing.T) {
	svc, _, _, members := syncFixture(t)
	members.members = []*types.TenantMember{member(11, types.TenantRoleOwner)}
	members.removeErr = ErrLastOwner
	res, err := svc.syncOIDCGroups(context.Background(), &types.User{ID: "u1"}, claims(), mapped)
	require.NoError(t, err)
	assert.Equal(t, []string{"last_owner:11"}, res.Skipped)
	assert.Empty(t, res.Removed)
}

func TestSyncOIDCGroups_AlreadyMemberGetsTheRole(t *testing.T) {
	svc, _, _, members := syncFixture(t)
	members.addErr = ErrMembershipAlreadyExists
	res, err := svc.syncOIDCGroups(context.Background(), &types.User{ID: "u1"}, claims("eng"), mapped)
	require.NoError(t, err)
	assert.Empty(t, res.Added)
	assert.Empty(t, res.Skipped)
	assert.Equal(t, []uint64{10}, res.RoleChanged)
	assert.Contains(t, members.calls, memberCall{"update", 10, types.TenantRoleAdmin})
}

func TestSyncOIDCGroups_UnknownWorkspaceIsSkipped(t *testing.T) {
	svc, _, _, members := syncFixture(t)
	m := []types.OIDCGroupMapping{
		{Group: "eng", Workspace: "nowhere", Role: types.TenantRoleAdmin},
		{Group: "eng", Workspace: "77", Role: types.TenantRoleAdmin},
	}
	res, err := svc.syncOIDCGroups(context.Background(), &types.User{ID: "u1"}, claims("eng"),
		types.OIDCGroupSettings{Claim: "groups", Map: m})
	require.NoError(t, err)
	assert.Empty(t, members.calls)
	assert.ElementsMatch(t, []string{"unknown_workspace:nowhere", "unknown_workspace:77"}, res.Skipped)
}

func TestSyncOIDCGroups_AdminGrantedAndRevoked(t *testing.T) {
	settings := types.OIDCGroupSettings{Claim: "groups", AdminGroups: []string{"admins"}}
	in := claims("admins")
	out := claims("eng")

	svc, repo, _, _ := syncFixture(t)
	res, err := svc.syncOIDCGroups(context.Background(), &types.User{ID: "u1"}, in, settings)
	require.NoError(t, err)
	assert.Equal(t, []string{"u1"}, repo.granted)
	assert.Equal(t, "granted", res.Admin)

	svc, repo, _, _ = syncFixture(t)
	res, err = svc.syncOIDCGroups(context.Background(), &types.User{ID: "u1", IsSystemAdmin: true}, in, settings)
	require.NoError(t, err)
	assert.Empty(t, repo.granted)
	assert.Empty(t, repo.revoked)
	assert.Equal(t, "unchanged", res.Admin)

	svc, repo, _, _ = syncFixture(t)
	res, err = svc.syncOIDCGroups(context.Background(), &types.User{ID: "u1", IsSystemAdmin: true}, out, settings)
	require.NoError(t, err)
	assert.Equal(t, [][2]string{{"u1", ""}}, repo.revoked)
	assert.Equal(t, "revoked", res.Admin)

	svc, repo, _, _ = syncFixture(t)
	repo.revokeErr = repository.ErrLastSystemAdmin
	res, err = svc.syncOIDCGroups(context.Background(), &types.User{ID: "u1", IsSystemAdmin: true}, out, settings)
	require.NoError(t, err)
	assert.Equal(t, "unchanged", res.Admin)
	assert.Equal(t, []string{"last_admin"}, res.Skipped)

	svc, repo, _, _ = syncFixture(t)
	res, err = svc.syncOIDCGroups(context.Background(), &types.User{ID: "u1"}, out, settings)
	require.NoError(t, err)
	assert.Empty(t, repo.granted)
	assert.Empty(t, repo.revoked)
	assert.Equal(t, "unchanged", res.Admin)
}

func TestSyncOIDCGroups_EmptyPresentClaimRevokesWithinGovernedOnly(t *testing.T) {
	svc, _, _, members := syncFixture(t)
	members.members = []*types.TenantMember{member(10, types.TenantRoleAdmin), member(12, types.TenantRoleOwner)}
	res, err := svc.syncOIDCGroups(context.Background(), &types.User{ID: "u1"}, claims(), mapped)
	require.NoError(t, err)
	assert.Equal(t, []memberCall{{"remove", 10, ""}}, members.calls)
	assert.Equal(t, []uint64{10}, res.Removed)
}

func TestSyncOIDCGroups_NoActorInContext(t *testing.T) {
	svc, _, _, members := syncFixture(t)
	ctx := context.WithValue(context.Background(), types.UserIDContextKey, "u1")
	_, err := svc.syncOIDCGroups(ctx, &types.User{ID: "u1"}, claims("eng"), mapped)
	require.NoError(t, err)
	assert.Equal(t, []memberCall{{"add", 10, types.TenantRoleAdmin}}, members.calls)
}

func TestSyncOIDCGroups_MemberErrorIsReturned(t *testing.T) {
	svc, _, _, members := syncFixture(t)
	members.addErr = errors.New("db down")
	_, err := svc.syncOIDCGroups(context.Background(), &types.User{ID: "u1"}, claims("eng"), mapped)
	require.Error(t, err)
}

func TestSyncOIDCGroups_LookupErrorAbortsBeforeAnyChange(t *testing.T) {
	svc, _, tenants, members := syncFixture(t)
	tenants.slugErr = errors.New("db down")
	members.members = []*types.TenantMember{member(20, types.TenantRoleAdmin)}
	m := []types.OIDCGroupMapping{
		{Group: "eng", Workspace: "20", Role: types.TenantRoleAdmin},
		{Group: "ops", Workspace: "home", Role: types.TenantRoleViewer},
	}
	_, err := svc.syncOIDCGroups(context.Background(), &types.User{ID: "u1"}, claims("ops"),
		types.OIDCGroupSettings{Claim: "groups", Map: m})
	require.Error(t, err)
	assert.Empty(t, members.calls, "a lookup failure must not turn into a removal")
}

func TestGroupsFromClaim_UnexpectedShapeIsAbsent(t *testing.T) {
	for _, raw := range []any{42, map[string]any{"a": 1}, []any{1, 2}, nil} {
		_, present := groupsFromClaim(map[string]any{"groups": raw}, "groups")
		assert.False(t, present, "%v", raw)
	}
}

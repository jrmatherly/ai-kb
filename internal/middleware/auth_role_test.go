package middleware

import (
	"context"
	"errors"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/config"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
)

// fakeMemberService is a hand-rolled stand-in for
// interfaces.TenantMemberService. It backs Get/HasAnyMembers/AddMember
// with two in-memory maps and lets each test seed exactly the rows it
// cares about. Other interface methods are stubbed because resolveTenantRole
// never touches them.
type fakeMemberService struct {
	members map[string]*types.TenantMember // key = userID + "|" + tenantID
	// addCalls records every AddMember invocation so tests can assert
	// that resolveTenantRole did (or didn't) attempt to auto-promote.
	addCalls []struct {
		UserID   string
		TenantID uint64
		Role     types.TenantRole
	}
	failGet    error
	failHasAny error
	failAdd    error
	// Keep auto-promote from flipping hasAny: by default a successful AddMember also writes the members map.
}

func newFakeMemberService() *fakeMemberService {
	return &fakeMemberService{members: map[string]*types.TenantMember{}}
}

func memberKey(u string, t uint64) string {
	return u + "|" + uintToStr(t)
}

func uintToStr(t uint64) string {
	// Plain number-to-string conversion, to avoid an extra dependency.
	if t == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for t > 0 {
		i--
		buf[i] = byte('0' + t%10)
		t /= 10
	}
	return string(buf[i:])
}

func (f *fakeMemberService) seedActive(userID string, tenantID uint64, role types.TenantRole) {
	f.members[memberKey(userID, tenantID)] = &types.TenantMember{
		UserID:   userID,
		TenantID: tenantID,
		Role:     role,
		Status:   types.TenantMemberStatusActive,
	}
}

func (f *fakeMemberService) AddMember(
	ctx context.Context, userID string, tenantID uint64, role types.TenantRole, invitedBy *string,
) (*types.TenantMember, error) {
	f.addCalls = append(f.addCalls, struct {
		UserID   string
		TenantID uint64
		Role     types.TenantRole
	}{userID, tenantID, role})
	if f.failAdd != nil {
		return nil, f.failAdd
	}
	m := &types.TenantMember{UserID: userID, TenantID: tenantID, Role: role, Status: types.TenantMemberStatusActive}
	f.members[memberKey(userID, tenantID)] = m
	return m, nil
}

func (f *fakeMemberService) EnsureOwner(
	ctx context.Context, userID string, tenantID uint64,
) (*types.TenantMember, error) {
	if existing, ok := f.members[memberKey(userID, tenantID)]; ok {
		return existing, nil
	}
	return f.AddMember(ctx, userID, tenantID, types.TenantRoleOwner, nil)
}

func (f *fakeMemberService) GetMembership(
	ctx context.Context, userID string, tenantID uint64,
) (*types.TenantMember, error) {
	if f.failGet != nil {
		return nil, f.failGet
	}
	m, ok := f.members[memberKey(userID, tenantID)]
	if !ok {
		return nil, nil
	}
	cp := *m
	return &cp, nil
}

func (f *fakeMemberService) ListByUser(ctx context.Context, userID string) ([]*types.TenantMember, error) {
	var out []*types.TenantMember
	for _, member := range f.members {
		if member.UserID == userID && member.Status == types.TenantMemberStatusActive {
			copy := *member
			out = append(out, &copy)
		}
	}
	return out, nil
}
func (f *fakeMemberService) ListByTenant(ctx context.Context, tenantID uint64) ([]*types.TenantMember, error) {
	return nil, nil
}
func (f *fakeMemberService) ListMembersPage(
	ctx context.Context, tenantID uint64, query string, page, pageSize int,
) ([]*types.TenantMember, int64, error) {
	return nil, 0, nil
}
func (f *fakeMemberService) HasAnyMembers(ctx context.Context, tenantID uint64) (bool, error) {
	if f.failHasAny != nil {
		return false, f.failHasAny
	}
	for _, m := range f.members {
		if m.TenantID == tenantID && m.Status == types.TenantMemberStatusActive {
			return true, nil
		}
	}
	return false, nil
}
func (f *fakeMemberService) UpdateRole(
	ctx context.Context, userID string, tenantID uint64, newRole types.TenantRole,
) error {
	return nil
}
func (f *fakeMemberService) RemoveMember(ctx context.Context, userID string, tenantID uint64) error {
	return nil
}

var _ interfaces.TenantMemberService = (*fakeMemberService)(nil)

func cfgWithRBAC(enabled bool) *config.Config {
	return &config.Config{Tenant: &config.TenantConfig{EnableRBAC: &enabled}}
}

func TestResolveTenantRole_ActiveMembershipWins(t *testing.T) {
	svc := newFakeMemberService()
	svc.seedActive("u1", 10, types.TenantRoleContributor)

	got, ok := resolveTenantRole(context.Background(), svc,
		&types.User{ID: "u1", TenantID: 10}, 10, false, cfgWithRBAC(true))
	if !ok || got != types.TenantRoleContributor {
		t.Fatalf("got (%v, %v), want (contributor, true)", got, ok)
	}
	if len(svc.addCalls) != 0 {
		t.Fatalf("must not auto-promote when membership exists, got %d AddMember calls", len(svc.addCalls))
	}
}

func TestResolveTenantRole_CrossTenantSuperuserGetsAdmin_NoAutoPromote(t *testing.T) {
	// Regression H1: a cross-workspace super admin switching to another workspace must never write tenant_members.
	svc := newFakeMemberService()
	user := &types.User{ID: "super", TenantID: 1, CanAccessAllTenants: true}

	got, ok := resolveTenantRole(context.Background(), svc, user, 99, true, cfgWithRBAC(true))
	if !ok || got != types.TenantRoleAdmin {
		t.Fatalf("got (%v, %v), want (admin, true)", got, ok)
	}
	if len(svc.addCalls) != 0 {
		t.Fatalf("cross-tenant superuser must not trigger auto-promote, got %+v", svc.addCalls)
	}
}

func TestResolveTenantRole_AutoPromoteRequiresHomeTenant(t *testing.T) {
	// Regression H1: even when the target is an orphan workspace, it must not auto-promote to Owner
	// unless it is the user's own home tenant.
	svc := newFakeMemberService() // empty: every workspace is an orphan
	user := &types.User{ID: "u1", TenantID: 1, CanAccessAllTenants: true}

	got, ok := resolveTenantRole(context.Background(), svc, user, 42, true, cfgWithRBAC(true))
	if !ok || got != types.TenantRoleAdmin {
		t.Fatalf("cross-tenant superuser should still get visitor Admin, got (%v, %v)", got, ok)
	}
	if len(svc.addCalls) != 0 {
		t.Fatalf("auto-promote must skip cross-tenant target, got %+v", svc.addCalls)
	}
}

func TestResolveTenantRole_AutoPromoteHomeTenant(t *testing.T) {
	// home tenant + orphan workspace + no switch -> auto-promote to Owner is allowed.
	svc := newFakeMemberService()
	user := &types.User{ID: "u1", TenantID: 7}

	got, ok := resolveTenantRole(context.Background(), svc, user, 7, false, cfgWithRBAC(true))
	if !ok || got != types.TenantRoleOwner {
		t.Fatalf("got (%v, %v), want (owner, true)", got, ok)
	}
	if len(svc.addCalls) != 1 || svc.addCalls[0].Role != types.TenantRoleOwner {
		t.Fatalf("expected exactly one Owner AddMember call, got %+v", svc.addCalls)
	}
}

func TestResolveTenantRole_AutoPromoteSkippedIfTenantHasMembers(t *testing.T) {
	svc := newFakeMemberService()
	// The same home tenant already has other members: the new login must not be auto-promoted.
	svc.seedActive("other", 7, types.TenantRoleOwner)
	user := &types.User{ID: "u1", TenantID: 7}

	got, ok := resolveTenantRole(context.Background(), svc, user, 7, false, cfgWithRBAC(true))
	if ok {
		t.Fatalf("RBAC enabled + no membership for u1 should be rejected, got role=%v", got)
	}
	if len(svc.addCalls) != 0 {
		t.Fatalf("must not auto-promote into a tenant that already has members, got %+v", svc.addCalls)
	}
}

func TestResolveTenantRole_FailOpenAdminWhenRBACDisabled(t *testing.T) {
	svc := newFakeMemberService()
	user := &types.User{ID: "u1", TenantID: 7}
	// targetTenantID != home, so the auto-promote branch is not taken.
	got, ok := resolveTenantRole(context.Background(), svc, user, 8, false, cfgWithRBAC(false))
	if !ok || got != types.TenantRoleAdmin {
		t.Fatalf("EnableRBAC=false should fail open Admin, got (%v, %v)", got, ok)
	}
}

func TestResolveTenantRole_FailClosedWhenRBACEnabled(t *testing.T) {
	svc := newFakeMemberService()
	// Other members exist, so auto-promotion is closed; RBAC is enabled -> must be 403.
	svc.seedActive("other", 8, types.TenantRoleOwner)
	user := &types.User{ID: "u1", TenantID: 7}
	if _, ok := resolveTenantRole(context.Background(), svc, user, 8, false, cfgWithRBAC(true)); ok {
		t.Fatalf("EnableRBAC=true + no membership should be rejected")
	}
}

func TestResolveTenantRole_LookupErrorFailsOpenWhenRBACDisabled(t *testing.T) {
	// On a transient DB error, fail-open mode must not lock out existing users. targetTenantID is
	// deliberately different from home, to avoid the home-tenant auto-promote branch.
	svc := newFakeMemberService()
	svc.failGet = errors.New("transient db failure")
	// Make HasAnyMembers return true, closing the orphan workspace self-heal path.
	svc.seedActive("placeholder", 8, types.TenantRoleAdmin)
	user := &types.User{ID: "u1", TenantID: 7}

	got, ok := resolveTenantRole(context.Background(), svc, user, 8, false, cfgWithRBAC(false))
	if !ok || got != types.TenantRoleAdmin {
		t.Fatalf("transient lookup error under RBAC=false should fail open Admin, got (%v, %v)", got, ok)
	}
}

func TestResolveTenantRole_DemotedUserCannotReclaimViaOrphan(t *testing.T) {
	// Edge case: after an admin soft-deletes every member, an evicted user logging in to their own home tenant
	// must not regain Owner just because HasAnyMembers=false.
	// The current policy "home tenant + orphan => Owner" is a design choice; this test pins that
	// path, so tightening the policy later means updating it.
	svc := newFakeMemberService()
	user := &types.User{ID: "demoted", TenantID: 5}
	got, ok := resolveTenantRole(context.Background(), svc, user, 5, false, cfgWithRBAC(true))
	if !ok || got != types.TenantRoleOwner {
		t.Fatalf("current policy allows orphan-tenant self-heal on home tenant, got (%v, %v)", got, ok)
	}
}

package service

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/jrmatherly/ai-kb/internal/application/repository"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
)

// groupsFromClaim reads the group names out of a verified claim set: a list of strings, a single name or a
// comma list. Any other shape reads as absent, so a misconfigured provider changes nothing.
func groupsFromClaim(claims map[string]any, name string) (groups []string, present bool) {
	switch v := claims[name].(type) {
	case []any:
		groups = []string{}
		for _, item := range v {
			if s, ok := item.(string); ok {
				groups = append(groups, s)
			}
		}
		if len(v) > 0 && len(groups) == 0 {
			return nil, false
		}
		return groups, true
	case string:
		groups = []string{}
		for _, s := range strings.Split(v, ",") {
			if s = strings.TrimSpace(s); s != "" {
				groups = append(groups, s)
			}
		}
		return groups, true
	}
	return nil, false
}

// desiredRoles folds the mapping over the user's groups: governed holds every workspace the mapping names,
// roles the highest role the groups give in each, unknown the workspace keys that resolve to nothing.
func desiredRoles(
	groups []string,
	m []types.OIDCGroupMapping,
	resolve func(string) (uint64, bool, error),
) (roles map[uint64]types.TenantRole, governed map[uint64]struct{}, unknown []string, err error) {
	roles = map[uint64]types.TenantRole{}
	governed = map[uint64]struct{}{}
	seenUnknown := map[string]bool{}
	for _, entry := range m {
		id, found, rerr := resolve(entry.Workspace)
		if rerr != nil {
			return nil, nil, nil, rerr
		}
		if !found {
			if !seenUnknown[entry.Workspace] {
				seenUnknown[entry.Workspace] = true
				unknown = append(unknown, entry.Workspace)
			}
			continue
		}
		governed[id] = struct{}{}
		if slices.Contains(groups, entry.Group) && entry.Role.Level() > roles[id].Level() {
			roles[id] = entry.Role
		}
	}
	return roles, governed, unknown, nil
}

// oidcWorkspaceResolver looks a mapping key up once: all digits is an id, anything else a slug. Only a true
// not-found is unknown; a lookup error aborts the sync, since a workspace named twice would otherwise be
// governed through the other key with the user wanted nowhere, and that removes memberships.
func oidcWorkspaceResolver(ctx context.Context, tenants interfaces.TenantService) func(string) (uint64, bool, error) {
	cache := map[string]uint64{}
	return func(key string) (uint64, bool, error) {
		if id, ok := cache[key]; ok {
			return id, id != 0, nil
		}
		var tenant *types.Tenant
		var err error
		if id, perr := strconv.ParseUint(key, 10, 64); perr == nil {
			tenant, err = tenants.GetTenantByID(ctx, id)
			if errors.Is(err, repository.ErrTenantNotFound) {
				err = nil
			}
		} else {
			tenant, err = tenants.GetTenantBySlug(ctx, key)
		}
		if err != nil {
			return 0, false, fmt.Errorf("resolve workspace %q: %w", key, err)
		}
		if tenant == nil {
			cache[key] = 0
			return 0, false, nil
		}
		cache[key] = tenant.ID
		return tenant.ID, true, nil
	}
}

// OIDCGovernedWorkspaces returns the ids the configured mapping names; the members page shows a banner for them.
func OIDCGovernedWorkspaces(
	ctx context.Context, settingsSvc interfaces.SystemSettingService, tenants interfaces.TenantService,
) (map[uint64]struct{}, error) {
	settings, err := LoadOIDCGroupSettings(ctx, settingsSvc)
	if err != nil || len(settings.Map) == 0 {
		return map[uint64]struct{}{}, err
	}
	_, governed, _, err := desiredRoles(nil, settings.Map, oidcWorkspaceResolver(ctx, tenants))
	return governed, err
}

// syncOIDCGroups applies the groups claim to the system admin flag and to membership in every governed
// workspace. It returns nil when the claim is absent. The context carries no actor on purpose: an actor equal
// to the target makes the member service record a removal as the member leaving.
func (s *userService) syncOIDCGroups(
	ctx context.Context,
	user *types.User,
	claims map[string]any,
	settings types.OIDCGroupSettings,
) (*types.OIDCGroupSync, error) {
	groups, present := groupsFromClaim(claims, settings.Claim)
	if !present {
		return nil, nil
	}
	ctx = context.WithValue(ctx, types.UserIDContextKey, "")
	res := &types.OIDCGroupSync{
		Added: []uint64{}, RoleChanged: []uint64{}, Removed: []uint64{}, Admin: "unchanged", Skipped: []string{},
	}

	if len(settings.AdminGroups) > 0 {
		if err := s.syncSystemAdmin(ctx, user, groups, settings.AdminGroups, res); err != nil {
			return res, err
		}
	}
	if len(settings.Map) == 0 {
		return res, nil
	}

	roles, governed, unknown, err := desiredRoles(groups, settings.Map, oidcWorkspaceResolver(ctx, s.tenantService))
	if err != nil {
		return res, err
	}
	for _, key := range unknown {
		res.Skipped = append(res.Skipped, "unknown_workspace:"+key)
	}
	current, err := s.memberService.ListByUser(ctx, user.ID)
	if err != nil {
		return res, fmt.Errorf("list memberships: %w", err)
	}
	have := map[uint64]types.TenantRole{}
	for _, m := range current {
		if _, ok := governed[m.TenantID]; ok {
			have[m.TenantID] = m.Role
		}
	}
	for _, id := range slices.Sorted(maps.Keys(governed)) {
		want, wanted := roles[id]
		got, member := have[id]
		switch {
		case wanted && !member:
			if _, err := s.memberService.AddMember(ctx, user.ID, id, want, nil); err != nil {
				if errors.Is(err, ErrMembershipAlreadyExists) {
					continue
				}
				return res, fmt.Errorf("add member %d: %w", id, err)
			}
			res.Added = append(res.Added, id)
		case wanted && got != want:
			if err := s.memberService.UpdateRole(ctx, user.ID, id, want); err != nil {
				if errors.Is(err, ErrLastOwner) {
					res.Skipped = append(res.Skipped, fmt.Sprintf("last_owner:%d", id))
					continue
				}
				return res, fmt.Errorf("update role %d: %w", id, err)
			}
			res.RoleChanged = append(res.RoleChanged, id)
		case !wanted && member:
			if err := s.memberService.RemoveMember(ctx, user.ID, id); err != nil {
				if errors.Is(err, ErrLastOwner) {
					res.Skipped = append(res.Skipped, fmt.Sprintf("last_owner:%d", id))
					continue
				}
				return res, fmt.Errorf("remove member %d: %w", id, err)
			}
			res.Removed = append(res.Removed, id)
		}
	}
	return res, nil
}

// syncSystemAdmin grants or revokes the flag from the admin groups. The flag is checked before any repository
// call so a sign-in that changes nothing never takes the admin row locks.
func (s *userService) syncSystemAdmin(
	ctx context.Context, user *types.User, groups, adminGroups []string, res *types.OIDCGroupSync,
) error {
	listed := slices.ContainsFunc(groups, func(g string) bool { return slices.Contains(adminGroups, g) })
	switch {
	case listed && !user.IsSystemAdmin:
		if err := s.userRepo.GrantSystemAdmin(ctx, user.ID); err != nil {
			return fmt.Errorf("grant system admin: %w", err)
		}
		user.IsSystemAdmin = true
		res.Admin = "granted"
	case !listed && user.IsSystemAdmin:
		// An empty actor skips the self-revoke guard: the subject is the user signing in.
		_, err := s.userRepo.RevokeSystemAdmin(ctx, user.ID, "")
		switch {
		case errors.Is(err, repository.ErrLastSystemAdmin):
			res.Skipped = append(res.Skipped, "last_admin")
		case errors.Is(err, repository.ErrUserNotSystemAdmin):
			// Revoked concurrently; the in-memory row follows so a later full-struct save cannot re-grant it.
			user.IsSystemAdmin = false
		case err != nil:
			return fmt.Errorf("revoke system admin: %w", err)
		default:
			user.IsSystemAdmin = false
			res.Admin = "revoked"
		}
	}
	return nil
}

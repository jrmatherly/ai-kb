package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
)

const (
	oidcGroupsClaimKey      = "oidc.groups_claim"
	oidcSystemAdminGroupKey = "oidc.system_admin_groups"
	oidcGroupRoleMapKey     = "oidc.group_role_map"
)

// ParseOIDCGroupMap parses "group:workspace:role" entries, splitting from the right so a group name may
// contain ':'; workspace is a slug or a decimal id.
func ParseOIDCGroupMap(entries []string) ([]types.OIDCGroupMapping, error) {
	var out []types.OIDCGroupMapping
	for _, entry := range entries {
		i := strings.LastIndex(entry, ":")
		j := -1
		if i > 0 {
			j = strings.LastIndex(entry[:i], ":")
		}
		if i < 0 || j < 0 {
			return nil, fmt.Errorf("%q expects group:workspace:role", entry)
		}
		group, workspace := strings.TrimSpace(entry[:j]), strings.TrimSpace(entry[j+1:i])
		role := types.TenantRole(strings.TrimSpace(entry[i+1:]))
		if group == "" || workspace == "" || role == "" {
			return nil, fmt.Errorf("%q expects group:workspace:role", entry)
		}
		if !role.IsValid() {
			return nil, fmt.Errorf("%q: unknown role %q", entry, role)
		}
		if _, err := strconv.ParseUint(workspace, 10, 64); err != nil && !types.ValidTenantSlug(workspace) {
			return nil, fmt.Errorf("%q: invalid workspace %q (a slug or a numeric id)", entry, workspace)
		}
		out = append(out, types.OIDCGroupMapping{Group: group, Workspace: workspace, Role: role})
	}
	return out, nil
}

// LoadOIDCGroupSettings resolves the three settings (stored row, then environment, then default).
func LoadOIDCGroupSettings(ctx context.Context, svc interfaces.SystemSettingService) (types.OIDCGroupSettings, error) {
	out := types.OIDCGroupSettings{
		Claim:       svc.GetString(ctx, oidcGroupsClaimKey, "OIDC_AUTH_GROUPS_CLAIM", "groups"),
		AdminGroups: svc.GetStringList(ctx, oidcSystemAdminGroupKey, "OIDC_AUTH_SYSTEM_ADMIN_GROUPS", []string{}),
	}
	entries := svc.GetStringList(ctx, oidcGroupRoleMapKey, "OIDC_AUTH_GROUP_ROLE_MAP", []string{})
	mapping, err := ParseOIDCGroupMap(entries)
	if err != nil {
		// Nothing of a half-read configuration is applied.
		return types.OIDCGroupSettings{}, fmt.Errorf("%s: %w", oidcGroupRoleMapKey, err)
	}
	out.Map = mapping
	return out, nil
}

package types

// OIDCGroupMapping is one "group:workspace:role" entry of oidc.group_role_map.
type OIDCGroupMapping struct {
	Group string
	// Workspace is a workspace slug or a decimal workspace id.
	Workspace string
	Role      TenantRole
}

// OIDCGroupSettings is the resolved group-to-role configuration for one sign-in.
type OIDCGroupSettings struct {
	Claim       string
	AdminGroups []string
	Map         []OIDCGroupMapping
}

// Enabled reports whether a sign-in applies the mapping at all.
func (s OIDCGroupSettings) Enabled() bool { return len(s.Map) > 0 || len(s.AdminGroups) > 0 }

// OIDCGroupSync is what one sign-in changed; the handler audits it.
type OIDCGroupSync struct {
	Added       []uint64 `json:"added"`
	RoleChanged []uint64 `json:"role_changed"`
	Removed     []uint64 `json:"removed"`
	// Admin is "granted", "revoked" or "unchanged".
	Admin string `json:"admin"`
	// Skipped lists what the sync left alone and why: last_owner:<id>, last_admin, unknown_workspace:<key>,
	// error (the sync stopped early; the log has the cause).
	Skipped []string `json:"skipped"`
}

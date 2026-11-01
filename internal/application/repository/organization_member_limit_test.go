package repository

import (
	"context"
	"fmt"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// The member limit is enforced with the insert, not by a separate count the
// caller did earlier, so concurrent joins cannot exceed it.
func TestAddTenantMemberEnforcesLimitWithTheInsert(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE organization_tenant_members (
		id TEXT PRIMARY KEY, organization_id TEXT, tenant_id INTEGER, role TEXT,
		representative_user_id TEXT, joined_at DATETIME, created_at DATETIME, updated_at DATETIME
	)`).Error)
	repo := &organizationRepository{db: db}
	add := func(tenantID uint64, limit int) error {
		return repo.AddTenantMember(context.Background(), &types.OrganizationTenantMember{
			ID: fmt.Sprintf("m-%d", tenantID), OrganizationID: "org", TenantID: tenantID, Role: types.OrgRoleViewer,
		}, limit)
	}

	require.NoError(t, add(1, 2))
	require.NoError(t, add(2, 2))
	require.ErrorIs(t, add(3, 2), ErrOrgMemberLimitReached)
	require.ErrorIs(t, add(2, 0), ErrOrgMemberAlreadyExists)
	require.NoError(t, add(3, 0), "no limit")

	var count int64
	require.NoError(t, db.Model(&types.OrganizationTenantMember{}).Count(&count).Error)
	require.Equal(t, int64(3), count)
}

// Zero is a real value for both fields (no member limit, an invite code that
// never expires), so a create must store it rather than the column default.
func TestCreateOrganizationKeepsZeroLimits(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE organizations (
		id TEXT PRIMARY KEY, name TEXT, description TEXT, avatar TEXT, owner_id TEXT, owner_tenant_id INTEGER,
		invite_code TEXT,
		invite_code_expires_at DATETIME, invite_code_validity_days INTEGER NOT NULL DEFAULT 7,
		require_approval BOOLEAN NOT NULL DEFAULT false, searchable BOOLEAN NOT NULL DEFAULT false,
		member_limit INTEGER NOT NULL DEFAULT 50, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
	)`).Error)
	repo := &organizationRepository{db: db}
	require.NoError(t, repo.Create(context.Background(), &types.Organization{ID: "org", Name: "o"}))

	var got types.Organization
	require.NoError(t, db.First(&got, "id = ?", "org").Error)
	require.Equal(t, 0, got.MemberLimit, "member_limit")
	require.Equal(t, 0, got.InviteCodeValidityDays, "invite_code_validity_days")
}

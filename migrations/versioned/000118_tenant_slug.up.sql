-- A unique human key for a workspace, used by the OIDC group mapping. Soft-deleted workspaces release theirs.
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS slug VARCHAR(64);
CREATE UNIQUE INDEX IF NOT EXISTS idx_tenants_slug ON tenants (slug) WHERE slug IS NOT NULL AND deleted_at IS NULL;

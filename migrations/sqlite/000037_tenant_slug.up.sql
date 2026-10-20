-- Mirrors versioned migration 000118: a unique human key for a workspace, used by the OIDC group mapping.
ALTER TABLE tenants ADD COLUMN slug TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_tenants_slug ON tenants (slug) WHERE slug IS NOT NULL AND deleted_at IS NULL;

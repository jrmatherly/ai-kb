DO $$ BEGIN RAISE NOTICE '[Migration 000065 down] Restoring tenants.api_key...'; END $$;

-- tenant_api_keys holds only hashes, so the restored column starts empty.
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS api_key VARCHAR(256) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_tenants_api_key ON tenants(api_key);
DROP INDEX IF EXISTS idx_tenant_api_keys_revoked_at;
DROP INDEX IF EXISTS idx_tenant_api_keys_tenant;
DROP TABLE IF EXISTS tenant_api_keys;

DO $$ BEGIN RAISE NOTICE '[Migration 000065 down] tenants.api_key restored'; END $$;

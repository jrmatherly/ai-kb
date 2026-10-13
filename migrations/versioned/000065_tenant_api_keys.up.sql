DO $$ BEGIN RAISE NOTICE '[Migration 000065] Creating tenant_api_keys...'; END $$;

CREATE TABLE IF NOT EXISTS tenant_api_keys (
    id BIGSERIAL PRIMARY KEY,
    tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name VARCHAR(128) NOT NULL,
    key_hash VARCHAR(64) NOT NULL UNIQUE,
    -- Masked display value; the token itself is never stored.
    key_hint VARCHAR(16) NOT NULL DEFAULT '',
    full_access BOOLEAN NOT NULL DEFAULT FALSE,
    knowledge_base_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    -- Bounded per-key grants for non-full-access keys. KB allow-list still
    -- constrains which knowledge bases a scoped key may touch.
    capabilities JSONB NOT NULL DEFAULT '[]'::jsonb,
    last_used_at TIMESTAMP,
    expires_at TIMESTAMP,
    revoked_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_tenant_api_keys_tenant
    ON tenant_api_keys(tenant_id);
CREATE INDEX IF NOT EXISTS idx_tenant_api_keys_revoked_at
    ON tenant_api_keys(revoked_at);

DROP INDEX IF EXISTS idx_tenants_api_key;
ALTER TABLE tenants DROP COLUMN IF EXISTS api_key;

DO $$ BEGIN RAISE NOTICE '[Migration 000065] tenant_api_keys ready'; END $$;

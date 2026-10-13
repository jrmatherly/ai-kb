-- Masked display value of an API key. The startup backfill (database.BackfillTenantAPIKeyHints) fills it, writes the
-- real hash for keys copied by 000065, and then clears api_key, so the token itself is no longer stored.
ALTER TABLE tenant_api_keys ADD COLUMN IF NOT EXISTS key_hint VARCHAR(16) NOT NULL DEFAULT '';

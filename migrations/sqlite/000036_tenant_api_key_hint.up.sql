-- Masked display value of an API key; see versioned 000117.
ALTER TABLE tenant_api_keys ADD COLUMN key_hint VARCHAR(16) NOT NULL DEFAULT '';

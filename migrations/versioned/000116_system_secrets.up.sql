-- Server-generated secrets (the per-install JWT secret) that no API reads or lists.
CREATE TABLE IF NOT EXISTS system_secrets (
    name VARCHAR(64) PRIMARY KEY,
    value TEXT NOT NULL
);

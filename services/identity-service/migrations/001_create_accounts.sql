CREATE TABLE IF NOT EXISTS accounts (
    id            UUID PRIMARY KEY,
    handle        TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Unicidade de handle case-insensitive (regra da spec).
CREATE UNIQUE INDEX IF NOT EXISTS accounts_handle_lower_idx ON accounts (lower(handle));

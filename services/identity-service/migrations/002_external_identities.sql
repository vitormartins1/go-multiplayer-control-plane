-- Identidade federada (spec 005-steam-login): vínculo entre um ID de loja
-- (ex.: SteamID64) e a conta do control plane. Uma conta pode ter N vínculos
-- (Steam hoje, GOG/EOS depois); um ID externo aponta para uma conta só.
CREATE TABLE IF NOT EXISTS external_identities (
    provider    TEXT NOT NULL,
    external_id TEXT NOT NULL,
    account_id  UUID NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (provider, external_id)
);

CREATE INDEX IF NOT EXISTS external_identities_account_idx ON external_identities (account_id);

-- Jogos registrados no backend (ADR-0017 — fonte canônica de game_id).
-- Banco próprio do game-registry (database-per-service, ADR-0015).
-- id é um UUID (string) armazenado como TEXT: a chave canônica game_id é o
-- valor, e TEXT evita o cast uuid=text no FindByID (id arbitrário -> not found).
CREATE TABLE games (
    id         TEXT        PRIMARY KEY,
    slug       TEXT        NOT NULL,
    name       TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

-- Unicidade do slug case-insensitive (mesmo padrão do handle no identity).
CREATE UNIQUE INDEX idx_games_slug_lower ON games (lower(slug));

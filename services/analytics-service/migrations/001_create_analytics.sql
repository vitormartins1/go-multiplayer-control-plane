-- 001_create_analytics: rollups pré-agregados por jogo (ADR-0021), alimentados
-- pelo fan-out do resultado das partidas (match-results). Tudo escopado por
-- game_id (multi-tenant, ADR-0017).

-- Idempotência do consumidor: cada partida processada uma única vez (SQS é
-- at-least-once). Marca + agregação na mesma transação (ver spec 006).
CREATE TABLE IF NOT EXISTS processed_matches (
    match_id     TEXT PRIMARY KEY,
    game_id      TEXT NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Rollup incremental por jogo. avgScore = score_sum / participations (no read).
CREATE TABLE IF NOT EXISTS game_rollup (
    game_id        TEXT PRIMARY KEY,
    matches_total  BIGINT NOT NULL DEFAULT 0,
    participations BIGINT NOT NULL DEFAULT 0, -- resultados de jogador somados
    wins           BIGINT NOT NULL DEFAULT 0,
    score_sum      BIGINT NOT NULL DEFAULT 0
);

-- Jogadores distintos vistos em cada jogo (para "jogadores únicos"). A PK
-- composta dedupa; count = únicos por jogo, count(distinct player) = global.
CREATE TABLE IF NOT EXISTS game_player_seen (
    game_id   TEXT NOT NULL,
    player_id TEXT NOT NULL,
    PRIMARY KEY (game_id, player_id)
);

-- Série "partidas por dia" (bucket UTC) por jogo.
CREATE TABLE IF NOT EXISTS game_daily (
    game_id TEXT NOT NULL,
    day     DATE NOT NULL,
    matches BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (game_id, day)
);

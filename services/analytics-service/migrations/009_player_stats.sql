-- 009_player_stats: stats brutos por jogador (spec 011) — os 3 envelopes
-- terminais (win_loss_draw/placement/collective) e o participation_event já
-- carregam `stats` na origem; até aqui o analytics-service descartava.
-- Write-many (uma linha por jogador×partida×stat), sem UNIQUE próprio: a
-- proteção contra reentrega vive na mesma tx do rollup já idempotente
-- (processed_matches / processed_participations).
CREATE TABLE IF NOT EXISTS game_player_stats (
    id          BIGSERIAL PRIMARY KEY,
    game_id     TEXT NOT NULL,
    mode_id     TEXT NOT NULL DEFAULT '',
    match_id    TEXT NOT NULL, -- session_id em host_world_dropin (sem matchId)
    player_id   TEXT NOT NULL,
    stat_key    TEXT NOT NULL,
    stat_value  BIGINT NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_game_player_stats_summary ON game_player_stats (game_id, stat_key);

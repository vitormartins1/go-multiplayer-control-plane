-- 003_participation_rollup: agregação de eventos de participação do
-- host_world_dropin (spec 013/004). Diferente do queue_arena (uma partida = um
-- ReportResult), o mundo host_world reporta vários eventos por sessão; a chave de
-- idempotência é (session_id, player_id, event) — dedupe a reentrega at-least-once
-- do SQS. O rollup conta por (game_id, role, event) — o funil de invasão/co-op.
CREATE TABLE IF NOT EXISTS processed_participations (
    session_id   TEXT NOT NULL,
    player_id    TEXT NOT NULL,
    event        TEXT NOT NULL,
    game_id      TEXT NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (session_id, player_id, event)
);

CREATE TABLE IF NOT EXISTS game_participation_rollup (
    game_id TEXT   NOT NULL,
    role    TEXT   NOT NULL,
    event   TEXT   NOT NULL,
    count   BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (game_id, role, event)
);

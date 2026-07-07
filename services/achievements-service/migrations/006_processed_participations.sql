-- 006_processed_participations: idempotência dos eventos de participação do
-- host_world_dropin (spec 013/009). Diferente do queue_arena (uma partida = um
-- ReportResult), o mundo host_world reporta VÁRIOS eventos por sessão. A chave de
-- idempotência é (session_id, player_id, event): dedupe a reentrega at-least-once
-- do SQS, contando uma vez por tipo de evento por sessão por jogador (decisão da
-- H2 — o produtor não carrega um eventId único por ocorrência).
CREATE TABLE IF NOT EXISTS processed_participations (
    session_id   TEXT NOT NULL,
    player_id    TEXT NOT NULL,
    event        TEXT NOT NULL,
    game_id      TEXT NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (session_id, player_id, event)
);

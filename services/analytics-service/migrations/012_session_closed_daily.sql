-- 012_session_closed_daily: série diária de sessões fechadas (spec 015) —
-- complementa o rollup cumulativo por motivo (game_session_rollup, spec
-- 013) com a evolução no tempo. Mesmo shape das outras 3 séries diárias
-- (game_placement_daily/game_mission_daily/game_participation_daily, spec
-- 010): total por dia, sem quebra por close_reason (o detalhe por motivo
-- já existe no rollup cumulativo).
CREATE TABLE IF NOT EXISTS game_session_daily (
    game_id TEXT NOT NULL,
    mode_id TEXT NOT NULL DEFAULT '',
    day     DATE NOT NULL,
    count   BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (game_id, mode_id, day)
);

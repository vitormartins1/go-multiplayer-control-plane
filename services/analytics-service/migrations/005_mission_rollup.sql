-- 005_mission_rollup: distribuição de desfecho da missão do coop_mission (C3,
-- ADR-0023). O fan-out do session/016 publica kind=collective (um desfecho
-- COLETIVO por sessão: MISSION_SUCCESS/FAILURE); este rollup agrega, por jogo, a
-- contagem de sucessos e falhas (taxa de sucesso derivada no read). Idempotência
-- reusa processed_matches (uma missão = um ReportResult, chave global matchId).
CREATE TABLE IF NOT EXISTS game_mission_rollup (
    game_id   TEXT PRIMARY KEY,
    successes BIGINT NOT NULL DEFAULT 0, -- outcome == MISSION_SUCCESS
    failures  BIGINT NOT NULL DEFAULT 0  -- outcome == MISSION_FAILURE
);

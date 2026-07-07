-- 004_placement_rollup: distribuição de colocação do battle_royale (BR3,
-- ADR-0023). O fan-out do session/015 publica kind=placement (colocação ordinal
-- por participante); este rollup agrega, por jogo, o histograma 1º/top-10/resto
-- e a soma de sobrevivência (avg derivado no read). Idempotência reusa
-- processed_matches (uma partida = um ReportResult, chave global matchId).
CREATE TABLE IF NOT EXISTS game_placement_rollup (
    game_id         TEXT PRIMARY KEY,
    winners         BIGINT NOT NULL DEFAULT 0, -- colocação == 1
    top_n           BIGINT NOT NULL DEFAULT 0, -- 2..threshold (exclui o 1º)
    rest            BIGINT NOT NULL DEFAULT 0, -- > threshold
    entries         BIGINT NOT NULL DEFAULT 0, -- participantes (denominador)
    sum_survived_ms BIGINT NOT NULL DEFAULT 0  -- avg = sum/entries no read
);

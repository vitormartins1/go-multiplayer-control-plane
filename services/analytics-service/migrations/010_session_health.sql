-- 010_session_health: consome o evento session_closed (session-service/028)
-- pra responder "que % das sessões terminam abandonadas?" e "qual a duração
-- média de uma partida/mundo?" (item C da evolução do Analytics). Diferente
-- dos rollups mais antigos (game_rollup etc.), esta tabela nasce COM
-- mode_id — sem a lacuna do item D.

-- Idempotência do consumidor: cada sessão fechada processada uma única vez
-- (SQS at-least-once), mesmo padrão de processed_matches/processed_participations.
CREATE TABLE IF NOT EXISTS processed_sessions (
    session_id   TEXT PRIMARY KEY,
    game_id      TEXT NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Distribuição por motivo de fechamento (spec 013): dado cru (soma/contagem),
-- sem rótulo de negócio embutido (ex.: "isso conta como abandono?" é decisão
-- de apresentação, não deste schema).
CREATE TABLE IF NOT EXISTS game_session_rollup (
    game_id         TEXT NOT NULL,
    mode_id         TEXT NOT NULL DEFAULT '',
    archetype       TEXT NOT NULL DEFAULT '',
    close_reason    TEXT NOT NULL,
    count           BIGINT NOT NULL DEFAULT 0,
    duration_sum_ms BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (game_id, mode_id, close_reason)
);

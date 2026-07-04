-- 004_processed_matches: idempotência do fan-out durável (ADR-0020).
-- O consumidor SQS marca cada partida processada aqui, na mesma transação em que
-- aplica o progresso. SQS entrega at-least-once: reprocessar a mesma match_id é
-- no-op (ON CONFLICT DO NOTHING). match_id é globalmente único (matchmaking).
CREATE TABLE IF NOT EXISTS processed_matches (
    match_id     TEXT PRIMARY KEY,
    game_id      TEXT NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

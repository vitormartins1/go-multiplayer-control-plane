-- 002_outcome_counts: desfecho polimórfico no rollup (ADR-0023, fatia "colher:
-- analytics"). O fan-out virou v2 e carrega `outcome` (win/loss/draw); o rollup
-- passa a contar derrotas e empates separadamente (antes só `wins`, derivado de
-- `won`, colapsava loss e draw). Aditivo: `wins` permanece.

ALTER TABLE game_rollup ADD COLUMN IF NOT EXISTS losses BIGINT NOT NULL DEFAULT 0;
ALTER TABLE game_rollup ADD COLUMN IF NOT EXISTS draws  BIGINT NOT NULL DEFAULT 0;

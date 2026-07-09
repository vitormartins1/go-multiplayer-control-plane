-- 006_participation_event_id_and_outcome: dois follow-ups do host_world_dropin
-- (ADR-0023) na mesma família de tabelas de participação (analytics/007):
--
-- 1) eventId (follow-up #4, espelha achievements/012): processed_participations
--    troca a chave de idempotência de (session_id, player_id, event) pra
--    event_id — sem isso, o mesmo evento repetido na mesma sessão (ex.: boss
--    que respawna) contava só uma vez. Backfill determinístico preserva o
--    comportamento antigo pra linhas já processadas.
--
-- 2) outcome no rollup (follow-up #5): game_participation_rollup agrega por
--    (game_id, role, event) sem distinguir outcome — HOST_KILLED·SUCCESS e
--    HOST_KILLED·FAIL (o boss matando o host sem invasor, spec 004 do Unity)
--    ficavam indistinguíveis. Linhas existentes ficam com outcome='' (bucket
--    "somado antes da dimensão existir" — não reconstituível sem os eventos
--    originais).

ALTER TABLE processed_participations ADD COLUMN event_id TEXT;

UPDATE processed_participations
   SET event_id = session_id || ':' || player_id || ':' || event
 WHERE event_id IS NULL;

ALTER TABLE processed_participations ALTER COLUMN event_id SET NOT NULL;
ALTER TABLE processed_participations DROP CONSTRAINT processed_participations_pkey;
ALTER TABLE processed_participations ADD PRIMARY KEY (event_id);

ALTER TABLE game_participation_rollup ADD COLUMN outcome TEXT NOT NULL DEFAULT '';
ALTER TABLE game_participation_rollup DROP CONSTRAINT game_participation_rollup_pkey;
ALTER TABLE game_participation_rollup ADD PRIMARY KEY (game_id, role, event, outcome);

-- 007_participation_event_id: troca a chave de idempotência de
-- (session_id, player_id, event) para event_id (follow-up #4, spec 021/012).
-- O session-service passa a repassar um eventId único por ocorrência gerado
-- pelo data plane (Unity) — sem isso, o mesmo evento repetido na mesma sessão
-- (ex.: BOSS_DEFEATED de um boss que respawna) contava só uma vez. Backfill
-- determinístico nas linhas existentes preserva o comportamento antigo pra
-- quem já foi processado antes desta migration.
ALTER TABLE processed_participations ADD COLUMN event_id TEXT;

UPDATE processed_participations
   SET event_id = session_id || ':' || player_id || ':' || event
 WHERE event_id IS NULL;

ALTER TABLE processed_participations ALTER COLUMN event_id SET NOT NULL;
ALTER TABLE processed_participations DROP CONSTRAINT processed_participations_pkey;
ALTER TABLE processed_participations ADD PRIMARY KEY (event_id);

-- 007_participation_mode_id: quebra por modo nomeado nas participações do
-- host_world_dropin (spec 009 — revisão de layout do admin-ui). O session-
-- service já publica mode_id no envelope participation_event (ADR-0024) desde
-- a spec 017 daquele serviço; até aqui o analytics-service descartava o
-- campo na desserialização. Dado antigo (sem essa dimensão) cai no bucket
-- mode_id='' ("sem modo") — mesmo padrão já usado pro outcome na migration 006.

ALTER TABLE game_participation_rollup ADD COLUMN mode_id TEXT NOT NULL DEFAULT '';
ALTER TABLE game_participation_rollup DROP CONSTRAINT game_participation_rollup_pkey;
ALTER TABLE game_participation_rollup ADD PRIMARY KEY (game_id, mode_id, role, event, outcome);

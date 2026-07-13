-- 011_mode_scoped_cumulative_rollups: fecha a lacuna do item D da evolução
-- do Analytics — game_rollup/game_daily/game_placement_rollup/
-- game_mission_rollup/game_player_seen são as tabelas mais antigas do
-- serviço (specs 001/002/004/005), anteriores ao ADR-0024, e nunca tiveram
-- mode_id. Mesmo padrão de migration já usado na 007
-- (game_participation_rollup): ADD COLUMN + DROP CONSTRAINT <table>_pkey +
-- ADD PRIMARY KEY com mode_id na chave. Dado legado cai em mode_id=''
-- (bucket "sem modo"), soma corretamente no agregado "todos os modos".

ALTER TABLE game_rollup ADD COLUMN mode_id TEXT NOT NULL DEFAULT '';
ALTER TABLE game_rollup DROP CONSTRAINT game_rollup_pkey;
ALTER TABLE game_rollup ADD PRIMARY KEY (game_id, mode_id);

ALTER TABLE game_daily ADD COLUMN mode_id TEXT NOT NULL DEFAULT '';
ALTER TABLE game_daily DROP CONSTRAINT game_daily_pkey;
ALTER TABLE game_daily ADD PRIMARY KEY (game_id, mode_id, day);

ALTER TABLE game_placement_rollup ADD COLUMN mode_id TEXT NOT NULL DEFAULT '';
ALTER TABLE game_placement_rollup DROP CONSTRAINT game_placement_rollup_pkey;
ALTER TABLE game_placement_rollup ADD PRIMARY KEY (game_id, mode_id);

ALTER TABLE game_mission_rollup ADD COLUMN mode_id TEXT NOT NULL DEFAULT '';
ALTER TABLE game_mission_rollup DROP CONSTRAINT game_mission_rollup_pkey;
ALTER TABLE game_mission_rollup ADD PRIMARY KEY (game_id, mode_id);

ALTER TABLE game_player_seen ADD COLUMN mode_id TEXT NOT NULL DEFAULT '';
ALTER TABLE game_player_seen DROP CONSTRAINT game_player_seen_pkey;
ALTER TABLE game_player_seen ADD PRIMARY KEY (game_id, mode_id, player_id);

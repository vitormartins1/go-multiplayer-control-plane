-- 008_archetype_daily: série temporal pros 3 arquétipos que não tinham
-- (spec 010) — só queue_arena tinha game_daily. Cada tabela já nasce com
-- mode_id na PK (mesma lacuna que a migration 007 corrigiu depois pra
-- participation_rollup; aqui evitamos repetir). Os rollups CUMULATIVOS
-- (game_rollup/game_placement_rollup/game_mission_rollup) não mudam.

CREATE TABLE IF NOT EXISTS game_placement_daily (
    game_id TEXT NOT NULL,
    mode_id TEXT NOT NULL DEFAULT '',
    day     DATE NOT NULL,
    matches BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (game_id, mode_id, day)
);

CREATE TABLE IF NOT EXISTS game_mission_daily (
    game_id TEXT NOT NULL,
    mode_id TEXT NOT NULL DEFAULT '',
    day     DATE NOT NULL,
    matches BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (game_id, mode_id, day)
);

-- host_world_dropin não tem "partida" terminal — a atividade é medida em
-- EVENTOS de participação (JOINED/BOSS_DEFEATED/DIED/...), não sessões.
CREATE TABLE IF NOT EXISTS game_participation_daily (
    game_id TEXT NOT NULL,
    mode_id TEXT NOT NULL DEFAULT '',
    day     DATE NOT NULL,
    events  BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (game_id, mode_id, day)
);

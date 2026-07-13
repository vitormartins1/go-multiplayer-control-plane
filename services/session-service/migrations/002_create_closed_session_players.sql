-- 002_create_closed_session_players: participantes com papel de cada sessão
-- fechada (spec 025) — a base da aba History por jogador. role vazio no
-- queue_arena sem times (FFA); "team:<id>" com times; "host"/"coop"/"invader"
-- no host_world_dropin.
CREATE TABLE closed_session_players (
    session_id TEXT NOT NULL REFERENCES closed_sessions (session_id),
    player_id  TEXT NOT NULL,
    role       TEXT NOT NULL DEFAULT '',
    joined_at  TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (session_id, player_id)
);

CREATE INDEX idx_closed_players_lookup ON closed_session_players (player_id, session_id);

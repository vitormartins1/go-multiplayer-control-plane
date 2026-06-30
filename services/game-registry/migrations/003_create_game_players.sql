-- 003: presença por jogo (feature 002-game-presence; ADR-0019).
-- Quem "está" em cada jogo. Identidade continua global (ADR-0017 #2): aqui só
-- guardamos player_id + handle (desnormalizado do login), NUNCA credenciais.
-- Marcada no login "dentro do jogo" pelo identity (best-effort).
CREATE TABLE game_players (
    game_id    TEXT        NOT NULL REFERENCES games (id),
    player_id  TEXT        NOT NULL,
    handle     TEXT        NOT NULL,
    first_seen TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (game_id, player_id)
);

-- Lista da base do jogo por ordem de chegada (first_seen).
CREATE INDEX idx_game_players_game_seen ON game_players (game_id, first_seen);

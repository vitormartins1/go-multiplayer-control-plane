-- Catálogo de conquistas (definido por seed; ver ADR-0015 — banco próprio).
CREATE TABLE achievements (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT NOT NULL,
    event_type  TEXT NOT NULL
);

CREATE INDEX idx_achievements_event_type ON achievements (event_type);

-- Desbloqueios por jogador. player_id é TEXT (o sub do JWT pode ser um UUID de
-- conta ou um "guest-<uuid>" do login anônimo). A PK composta garante a
-- idempotência do desbloqueio.
CREATE TABLE player_achievements (
    player_id      TEXT        NOT NULL,
    achievement_id TEXT        NOT NULL REFERENCES achievements (id),
    unlocked_at    TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (player_id, achievement_id)
);

CREATE INDEX idx_player_achievements_player ON player_achievements (player_id);

-- Seed do catálogo (conquistas "primeira vez" = desbloqueio direto, MVP 001).
INSERT INTO achievements (id, name, description, event_type) VALUES
    ('first_match', 'Estreia',          'Jogue sua primeira partida',  'match_played'),
    ('first_win',   'Primeira Vitória', 'Vença sua primeira partida',  'match_won'),
    ('first_blood', 'Primeiro Sangue',  'Consiga seu primeiro abate',  'kill');

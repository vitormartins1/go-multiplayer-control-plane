-- 001_create_closed_sessions: histórico persistido de sessões fechadas (spec
-- 025). O session-service é o único serviço stateful-por-natureza sem banco —
-- sem isto, a aba Sessions do jogo e a aba History do player somem no restart
-- (o ring in-memory é bounded e não sobrevive ao processo). Write-once: um
-- INSERT no fechamento (closeSession), nunca UPDATE/DELETE. Cobre TODOS os
-- arquétipos (o fechamento é comum); campos específicos ficam em default
-- vazio nos arquétipos que não os usam.
CREATE TABLE closed_sessions (
    session_id     TEXT PRIMARY KEY,
    match_id       TEXT NOT NULL DEFAULT '',
    game_id        TEXT NOT NULL,
    mode_id        TEXT NOT NULL DEFAULT '',
    archetype      TEXT NOT NULL DEFAULT '',
    origin         TEXT NOT NULL,
    host_player_id TEXT NOT NULL DEFAULT '',
    close_reason   TEXT NOT NULL,
    profile_hash   TEXT NOT NULL DEFAULT '',
    opened_at      TIMESTAMPTZ NOT NULL,
    closed_at      TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_closed_sessions_game ON closed_sessions (game_id, closed_at DESC);

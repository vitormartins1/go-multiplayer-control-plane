-- 003: multi-tenant por jogo (feature 004-game-scope; ADR-0017).
-- O catálogo e o progresso passam a ser por game_id. Os dados existentes
-- (pré-multi-tenant) migram para o jogo "sandbox" (id fixo, seedado no
-- game-registry — DEVE casar com aquele seed).

-- Jogo default para o backfill. O DEFAULT é removido ao final: depois da
-- migração, todo insert precisa de game_id explícito (pega bug de escopo).
ALTER TABLE achievements ADD COLUMN game_id TEXT NOT NULL
    DEFAULT '00000000-0000-0000-0000-000000000000';

ALTER TABLE player_achievements ADD COLUMN game_id TEXT NOT NULL
    DEFAULT '00000000-0000-0000-0000-000000000000';

-- Dropar a FK ANTES de mexer no PK de achievements (a FH depende dele).
ALTER TABLE player_achievements DROP CONSTRAINT player_achievements_achievement_id_fkey;

-- Catálogo: o slug (id) passa a ser único POR JOGO.
ALTER TABLE achievements DROP CONSTRAINT achievements_pkey;
ALTER TABLE achievements ADD PRIMARY KEY (game_id, id);

-- Progresso: refaz a PK e a FK incluindo o game_id.
ALTER TABLE player_achievements DROP CONSTRAINT player_achievements_pkey;
ALTER TABLE player_achievements ADD PRIMARY KEY (game_id, player_id, achievement_id);
ALTER TABLE player_achievements
    ADD FOREIGN KEY (game_id, achievement_id) REFERENCES achievements (game_id, id);

-- index por jogador agora considera o jogo.
DROP INDEX IF EXISTS idx_player_achievements_player;
CREATE INDEX idx_player_achievements_game_player ON player_achievements (game_id, player_id);

-- Backfill concluído: remover o DEFAULT (inserts novos devem informar game_id).
ALTER TABLE achievements ALTER COLUMN game_id DROP DEFAULT;
ALTER TABLE player_achievements ALTER COLUMN game_id DROP DEFAULT;

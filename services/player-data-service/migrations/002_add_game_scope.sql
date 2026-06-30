-- 002: multi-tenant por jogo (feature 002-game-scope; ADR-0017).
-- O dado de jogador passa a ser por game_id. Os dados existentes
-- (pré-multi-tenant) migram para o jogo "sandbox" (id fixo, seedado no
-- game-registry — DEVE casar com aquele seed).

-- Jogo default para o backfill. O DEFAULT é removido ao final: depois da
-- migração, todo insert precisa de game_id explícito (pega bug de escopo).
ALTER TABLE player_data ADD COLUMN game_id TEXT NOT NULL
    DEFAULT '00000000-0000-0000-0000-000000000000';

-- O dado passa a ser único POR JOGO: refaz a PK incluindo o game_id.
ALTER TABLE player_data DROP CONSTRAINT player_data_pkey;
ALTER TABLE player_data ADD PRIMARY KEY (game_id, player_id, namespace, key);

-- index para o admin read (todo o dado de um jogador num jogo) e para o
-- GET por namespace.
CREATE INDEX idx_player_data_game_player ON player_data (game_id, player_id, namespace);

-- Backfill concluído: remover o DEFAULT (inserts novos devem informar game_id).
ALTER TABLE player_data ALTER COLUMN game_id DROP DEFAULT;

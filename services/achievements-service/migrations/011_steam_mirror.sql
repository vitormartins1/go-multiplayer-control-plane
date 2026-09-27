-- 011_steam_mirror (spec 019): espelhar conquistas na Steam.
-- achievements.steam_api_name: o "API Name" da conquista no Steamworks; vazio =
-- não espelha. player_achievements ganha o estado do espelhamento: o pendente
-- é "desbloqueada + conquista com steam_api_name + estado vazio" — os caminhos
-- de desbloqueio não mudam e preencher o nome depois faz o backfill sozinho.
ALTER TABLE achievements ADD COLUMN steam_api_name TEXT NOT NULL DEFAULT '';

ALTER TABLE player_achievements ADD COLUMN steam_sync_state TEXT NOT NULL DEFAULT ''; -- '' | synced | skipped | failed
ALTER TABLE player_achievements ADD COLUMN steam_sync_attempts INT NOT NULL DEFAULT 0;
ALTER TABLE player_achievements ADD COLUMN steam_next_attempt_at TIMESTAMPTZ;         -- NULL = já pode tentar
ALTER TABLE player_achievements ADD COLUMN steam_sync_error TEXT NOT NULL DEFAULT '';
ALTER TABLE player_achievements ADD COLUMN steam_synced_at TIMESTAMPTZ;

-- a fila: desbloqueios ainda não espelhados, por jogo
CREATE INDEX IF NOT EXISTS player_achievements_steam_pending_idx
    ON player_achievements (game_id, steam_next_attempt_at)
    WHERE unlocked_at IS NOT NULL AND steam_sync_state = '';

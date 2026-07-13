-- 003_closed_session_result_and_outcome: fecha gaps encontrados na revisão
-- da spec 025 (spec 027) — Result e TeamByPlayer nunca eram persistidos (o
-- histórico caía num roster achatado sem outcome/placar/times), e o
-- host_world_dropin nunca guardava o último evento/desfecho por participante.
-- Todas as colunas nullable/default vazio: dado legado (sessões fechadas
-- antes desta migration) fica no mesmo "bucket vazio" já usado pelos demais
-- campos opcionais da spec 025.
ALTER TABLE closed_sessions ADD COLUMN result JSONB;
ALTER TABLE closed_sessions ADD COLUMN team_by_player JSONB;

ALTER TABLE closed_session_players ADD COLUMN last_event TEXT NOT NULL DEFAULT '';
ALTER TABLE closed_session_players ADD COLUMN last_outcome TEXT NOT NULL DEFAULT '';

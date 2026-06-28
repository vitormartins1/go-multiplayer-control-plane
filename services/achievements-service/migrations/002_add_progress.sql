-- 002: progresso parcial (feature 002-partial-progress).
-- Generaliza o desbloqueio direto da 001: cada conquista ganha uma meta
-- (target) e a linha de player_achievements passa a guardar o progresso;
-- unlocked_at NULL = em progresso, preenchido = desbloqueada.

ALTER TABLE achievements ADD COLUMN target INT NOT NULL DEFAULT 1;

ALTER TABLE player_achievements ADD COLUMN progress INT NOT NULL DEFAULT 0;
ALTER TABLE player_achievements ALTER COLUMN unlocked_at DROP NOT NULL;

-- Conquista progressiva de exemplo. Mesmo event_type da first_win: um único
-- evento match_won afeta as duas (desbloqueia first_win e progride ten_wins).
INSERT INTO achievements (id, name, description, event_type, target) VALUES
    ('ten_wins', 'Veterano', 'Vença 10 partidas', 'match_won', 10);

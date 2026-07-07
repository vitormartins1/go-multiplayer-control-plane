-- 005_stat_criteria: conquistas por métrica (stat), ADR-0023 / A1.
-- Generaliza o critério: além de "por evento" (event_type + target = ocorrências),
-- uma conquista pode ser "por stat" (stat_key + target = threshold, agregado por
-- stat_mode). Discriminador: stat_key <> '' -> critério por stat.
ALTER TABLE achievements ADD COLUMN stat_key  TEXT NOT NULL DEFAULT '';
ALTER TABLE achievements ADD COLUMN stat_mode TEXT NOT NULL DEFAULT '';

-- Busca por stat_key só faz sentido para as conquistas por stat: índice parcial.
CREATE INDEX idx_achievements_stat_key
    ON achievements (game_id, stat_key) WHERE stat_key <> '';

-- Seed de exemplo no jogo sandbox (id fixo, casa o seed do game-registry).
-- "Franco-atirador": 100 kills acumulados; "Ás": 5 kills numa única partida.
INSERT INTO achievements (game_id, id, name, description, event_type, stat_key, stat_mode, target) VALUES
    ('00000000-0000-0000-0000-000000000000', 'sharpshooter', 'Franco-atirador', 'Acumule 100 abates',        '', 'kills', 'cumulative', 100),
    ('00000000-0000-0000-0000-000000000000', 'ace',          'Ás',              'Consiga 5 abates numa partida', '', 'kills', 'per_match',  5);

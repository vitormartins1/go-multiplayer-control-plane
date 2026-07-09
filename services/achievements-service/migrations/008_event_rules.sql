-- 008_event_rules: mecanismo genérico de regras evento->achievement-event-type,
-- substituindo as 4 funções hardcodadas (ParticipationEventTypes,
-- PlacementEventTypes, CollectiveEventTypes, e a lógica inline de
-- ApplyMatchResult/win_loss_draw). Cada regra casa um subconjunto exato de
-- "facts" normalizados do envelope recebido contra `match`, e some os
-- `event_types` de todas as regras que baterem. game_id NULL = regra global
-- do arquétipo (aplica a todo jogo que usa esse arquétipo); game_id setado =
-- override/adição específica de um jogo.
CREATE TABLE event_rules (
    id          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    game_id     TEXT,              -- NULL = regra global do arquétipo
    archetype   TEXT NOT NULL,     -- queue_arena | host_world_dropin | battle_royale | coop_mission
    match       JSONB NOT NULL,    -- ex.: {"role":"invader","event":"HOST_KILLED","outcome":"SUCCESS"}
    event_types TEXT[] NOT NULL,   -- união de event types emitidos quando a regra bate
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_event_rules_archetype_game ON event_rules (archetype, game_id);

-- Seed: replica EXATAMENTE o comportamento das 4 funções/lógicas hardcoded que
-- existiam antes desta migration (zero regressão pros jogos já rodando).
-- Todos os valores de match/facts são strings (inclusive placement, evita
-- ambiguidade float64/int JSON<->Go); match = '{}' sempre bate (subconjunto
-- vazio de qualquer fact map).

-- host_world_dropin (era domain.ParticipationEventTypes)
INSERT INTO event_rules (game_id, archetype, match, event_types) VALUES
    (NULL, 'host_world_dropin', '{"role":"invader","event":"HOST_KILLED","outcome":"SUCCESS"}', ARRAY['invasion_won']),
    (NULL, 'host_world_dropin', '{"role":"coop","event":"BOSS_DEFEATED","outcome":"SUCCESS"}',   ARRAY['coop_boss_helped']),
    (NULL, 'host_world_dropin', '{"event":"JOINED"}',                                            ARRAY['world_joined']);

-- battle_royale (era domain.PlacementEventTypes) — tier "top_n" derivado na
-- normalização (threshold BattleRoyaleTopNThreshold=10 continua hardcoded lá,
-- não é vocabulário, é parâmetro numérico do arquétipo).
INSERT INTO event_rules (game_id, archetype, match, event_types) VALUES
    (NULL, 'battle_royale', '{}',               ARRAY['battle_royale_played']),
    (NULL, 'battle_royale', '{"placement":"1"}', ARRAY['battle_royale_win']),
    (NULL, 'battle_royale', '{"tier":"top_n"}',  ARRAY['battle_royale_top_n']);

-- coop_mission (era domain.CollectiveEventTypes)
INSERT INTO event_rules (game_id, archetype, match, event_types) VALUES
    (NULL, 'coop_mission', '{}',                            ARRAY['coop_mission_played']),
    (NULL, 'coop_mission', '{"outcome":"MISSION_SUCCESS"}', ARRAY['coop_mission_success']),
    (NULL, 'coop_mission', '{"outcome":"MISSION_FAILURE"}', ARRAY['coop_mission_failed']);

-- queue_arena (era lógica inline em Service.ApplyMatchResult, win_loss_draw)
INSERT INTO event_rules (game_id, archetype, match, event_types) VALUES
    (NULL, 'queue_arena', '{}',                ARRAY['match_played']),
    (NULL, 'queue_arena', '{"outcome":"win"}',  ARRAY['match_won']),
    (NULL, 'queue_arena', '{"outcome":"loss"}', ARRAY['match_lost']),
    (NULL, 'queue_arena', '{"outcome":"draw"}', ARRAY['match_drawn']);

-- 005: perfil de multiplayer por jogo (ADR-0023, spec 006-multiplayer-profile).
-- Relação 1:1 opcional com o jogo: NULL = jogo sem perfil configurado.
-- JSONB guarda { archetype, topology, params } e acomoda os params polimórficos
-- dos arquétipos futuros sem nova migration (queue_arena no MVP).
-- Sem backfill: jogos existentes ficam com perfil NULL (coerente com opcional).
ALTER TABLE games ADD COLUMN multiplayer_profile JSONB;

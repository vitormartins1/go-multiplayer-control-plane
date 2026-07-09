-- 009: comparadores no match das event rules (spec 014, V2a do ADR-0028).
--
-- O valor de cada chave do match passa a aceitar, além da string (igualdade,
-- formato do ADR-0027 — intocado), um objeto {"op": "...", "value": ...} com
-- os operadores lt/lte/gt/gte (numéricos), ne e in.
--
-- Migração de dado: o "top N" do battle_royale deixa de ser o fact derivado
-- `tier` (hardcoded na normalização Go, BattleRoyaleTopNThreshold) e vira a
-- condição EDITÁVEL `placement <= 10` — parâmetro por dado, não por código.
UPDATE event_rules
SET match = (match - 'tier') || '{"placement":{"op":"lte","value":10}}'::jsonb
WHERE match ? 'tier';

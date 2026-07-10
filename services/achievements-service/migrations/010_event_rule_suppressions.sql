-- 010_event_rule_suppressions: V2b do ADR-0028 (supressão de regra global por
-- jogo — a pergunta aberta do ADR-0027). Um jogo pode desligar, para si, uma
-- regra global do arquétipo (event_rules.game_id IS NULL) sem tocar a linha
-- compartilhada: a supressão é modelada como relação própria, não como
-- coluna mutável em event_rules (a linha global permanece IMUTÁVEL — nenhuma
-- escrita nela ocorre pra suprimir/restaurar). Suprimir uma regra de jogo
-- (override) não faz sentido — já é removível via DELETE .../event-rules/{id}.
CREATE TABLE event_rule_suppressions (
    game_id    TEXT NOT NULL,
    rule_id    TEXT NOT NULL REFERENCES event_rules(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (game_id, rule_id)
);

# Plan 017 — Suprimir regra global por jogo (V2b)

## Decisões técnicas

1. **Tabela própria `event_rule_suppressions`** (migration 010), PK
   `(game_id, rule_id)`, `rule_id` com FK `ON DELETE CASCADE` pra
   `event_rules(id)`. Justificativa já registrada na spec — preserva o
   invariante "global é imutável" (nenhuma escrita na linha compartilhada) e
   modela a relação N:M de forma auditável (`created_at`), sem precisar de
   índice extra (a PK já cobre as duas formas de lookup: por
   `game_id` sozinho em `ListEffective`, por `(game_id, rule_id)` em
   `RulesForArchetype`/`Suppress`/`Unsuppress`).
2. **`RulesForArchetype` exclui globais suprimidas na própria query** —
   `AND NOT EXISTS (SELECT 1 FROM event_rule_suppressions s WHERE s.game_id =
   $2 AND s.rule_id = event_rules.id)`. Zero mudança em `Service.ApplyXxx`:
   o caminho de avaliação já usa este método antes de `EvaluateRules`.
3. **`ListEffective` computa `suppressed` via `LEFT JOIN`** —
   `LEFT JOIN event_rule_suppressions s ON s.game_id = $1 AND s.rule_id =
   event_rules.id`, `SELECT ... (s.rule_id IS NOT NULL) AS suppressed`. Só
   este método popula `EventRule.Suppressed`; `RulesForArchetype` nunca
   devolve uma suprimida, então o campo fica no zero-value (`false`) lá — não
   precisa de projeção extra nessa query.
4. **Domain:** `EventRule.Suppressed bool` (novo campo). `EvaluateRules`
   ganha `if r.Suppressed { continue }` no topo do loop — defensivo, barato,
   documentado como cinto-e-suspensório (a fonte de verdade é a query #2).
5. **Repository — `Suppress`:** duas idas ao banco, sem transação (idempotente
   por natureza, corrida entre chamadas concorrentes do MESMO par não corrompe
   nada — pior caso é uma tentativa a mais de `ON CONFLICT DO NOTHING`):
   1. `SELECT EXISTS(SELECT 1 FROM event_rules WHERE id = $1 AND game_id IS
      NULL)` — se `false`, `ErrEventRuleNotFound` (cobre "não existe" e "não é
      global" com a mesma resposta, mesmo espírito do Delete/Update
      existentes).
   2. `INSERT INTO event_rule_suppressions (game_id, rule_id) VALUES ($1, $2)
      ON CONFLICT (game_id, rule_id) DO NOTHING` — idempotente, sem
      `RETURNING` (não precisamos distinguir "criou" de "já existia", os dois
      são sucesso).
   *(Alternativa descartada: um único `INSERT ... SELECT ... WHERE ... ON
   CONFLICT DO NOTHING RETURNING` — `RETURNING` não distingue "conflito
   porque já suprimida" (sucesso) de "SELECT vazio porque regra não é global"
   (erro), ambos devolvem zero linhas. Duas queries simples resolvem sem
   ambiguidade.)*
6. **Repository — `Unsuppress`:** uma query só, mesmo padrão do `DeleteRule`
   existente — `DELETE FROM event_rule_suppressions WHERE game_id = $1 AND
   rule_id = $2`; `RowsAffected() == 0` → `ErrEventRuleNotFound`.
7. **Service:** `SuppressEventRule`/`UnsuppressEventRule` são passthrough
   fino pro repositório (gameID/ruleID já vêm validados como strings do path;
   não há conteúdo de payload pra validar, ao contrário de Create/Update).
8. **Transport:** duas rotas novas sem body
   (`POST`/`DELETE /admin/games/{gameId}/event-rules/{id}/suppress`), `204`
   em sucesso, erro de domínio mapeado como as demais (`writeDomainError`).
   `eventRuleDTO` ganha `Suppressed bool \`json:"suppressed"\`` — populado por
   `toEventRuleDTO` a partir de `r.Suppressed` (já correto pros três
   handlers existentes, que sempre lidam com regras de jogo → `false`).
9. **BFF:** replica o padrão de passthrough das 4 camadas já usadas por
   `UpdateEventRule` (`domain.AdminReader` → `service.Service` →
   `achievements.Client` → `transport.Handler`) — sem lógica nova, só mais
   dois métodos espelhados.

## Arquivos

| Arquivo | Ação |
|---|---|
| `migrations/010_event_rule_suppressions.sql` | novo — tabela + FK |
| `infra/docker/docker-compose.yml` | mount da 010 no initdb do achievements-postgres |
| `internal/domain/event_rule.go` | `Suppressed bool`; guarda em `EvaluateRules`; `Suppress`/`Unsuppress` nas Ports |
| `internal/domain/event_rule_test.go` | teste: `EvaluateRules` ignora regra com `Suppressed: true` |
| `internal/repository/postgres_event_rule.go` | `RulesForArchetype` (NOT EXISTS), `ListEffective` (LEFT JOIN), `Suppress`, `Unsuppress` |
| `internal/repository/postgres_event_rule_test.go` (ou equivalente de integração) | cobre os 4 casos de `Suppress`/`Unsuppress` |
| `internal/service/achievement_service.go` | `SuppressEventRule`/`UnsuppressEventRule` |
| `internal/service/achievement_service_test.go` | idem, mocks do repo |
| `internal/transport/http_handler.go` | rotas + handlers + `eventRuleDTO.Suppressed` |
| `internal/transport/http_handler_test.go` | 204/404 dos dois endpoints |
| admin-bff: `internal/domain/admin.go`, `internal/achievements/client.go`, `internal/service/admin_service.go`, `internal/transport/http_handler.go` | passthrough das 2 rotas novas |

## Riscos / atenção

- Banco em uso: a migration 010 precisa ser aplicada manualmente no container
  já existente (initdb só roda em volume novo — mesmo gotcha das migrations
  009/Grafana).
- `ON DELETE CASCADE` na FK é defensivo (globais não são removíveis pela API
  hoje) — não deve ter efeito observável nesta fatia, só evita registro órfão
  se isso mudar no futuro.
- E2E precisa de DOIS jogos de teste do mesmo arquétipo pra provar que a
  supressão não vaza entre jogos (critério de aceite #5 da spec).

# Plan 014 — Comparadores no match (V2a)

## Decisões técnicas

1. **`MatchCond` com JSON custom no domain:** `EventRule.Match` vira
   `map[string]MatchCond` (`Op string` + `Value any`). `UnmarshalJSON` aceita
   string (→ `eq`) ou objeto `{op, value}`; decode **leniente** (value de tipo
   errado/op desconhecido → `Value nil`, condição nunca casa) + validação
   **estrita** na criação (`ErrInvalidEventRule`) — cobre os dois caminhos
   (payload do admin e linha antiga/futura no banco) sem derrubar avaliação.
   `MarshalJSON` emite string pura pro `eq` (formato compacto preservado no
   JSONB e na API — front antigo leria igual).
2. **Avaliador:** `matchSubset` delega pra `condMatches(cond, factVal,
   present)` — switch por op, puro. Numéricos: `strconv.ParseFloat` no fact;
   value `float64` (JSON number). Op desconhecido → false.
3. **Warning de op desconhecido no banco:** no `scanEventRules` do repository
   (camada que pode logar), `slog.Warn` por regra com cond inválida — o
   domain permanece sem I/O.
4. **Helper `Eq(v)`** no domain pra literais legíveis (seeds de teste e call
   sites: `map[string]MatchCond{"outcome": Eq("win")}`).
5. **Migration 009** (`UPDATE ... WHERE match ? 'tier'`): troca `tier=top_n`
   por `placement lte 10`; `PlacementFacts` perde a derivação e
   `BattleRoyaleTopNThreshold` some. Compose monta a 009 no initdb; pro banco
   JÁ inicializado, aplicar via `psql` manual (initdb só roda em volume novo —
   mesmo gotcha do Grafana provisioning).
6. **Transport:** req/DTO trocam `map[string]string` por
   `map[string]domain.MatchCond` — a serialização custom faz o resto; BFF é
   passthrough (zero mudança).
7. **Admin-ui:** tipo `MatchValue = string | {op, value}` na api; helper
   `condLabel` (ex.: `placement ≤ 10`, `mode in [a, b]`) usado por tabela,
   nós e painéis; linhas do form ganham select de op (`= ≠ < ≤ > ≥ in`),
   value numérico pros ops numéricos e lista por vírgula no `in`; nó de fact
   passa a ser 1 por CONDIÇÃO distinta (id inclui op+value).

## Arquivos

| Arquivo | Ação |
|---|---|
| `internal/domain/event_rule.go` | MatchCond + condMatches + Eq; remove tier/threshold |
| `internal/domain/event_rule_test.go` | testes de op + round-trip JSON; adapta literais |
| `internal/service/achievement_service.go` | validateEventRule cobre conds |
| `internal/repository/postgres_event_rule.go` | warning de cond inválida |
| `migrations/009_match_operators.sql` | novo |
| `infra/docker/docker-compose.yml` | mount da 009 |
| `internal/transport/http_handler.go` | req/DTO com MatchCond |
| admin-ui: `lib/api.ts`, `lib/matchCond.ts` (novo), `lib/rulesGraph.ts`, `components/flow/nodes.tsx`, `pages/GameEventRules.tsx`, `pages/GameEventRulesCanvas.tsx` | render + form de operador |

## Riscos / atenção

- Literais `map[string]string` de Match espalhados em testes — atualização
  mecânica com `Eq()`.
- Banco em uso: aplicar a 009 manualmente no container (initdb não re-roda).
- e2e igual ao da spec 013 (SQS/LocalStack): placement=10 casa top_n, 11 não.

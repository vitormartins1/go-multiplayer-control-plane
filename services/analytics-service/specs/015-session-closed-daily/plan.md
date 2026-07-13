# Plan — 015 série diária de sessões fechadas

## Decisões técnicas

1. **Mesmo padrão de schema/leitura das 3 séries diárias já existentes**
   (`game_placement_daily`/`game_mission_daily`/`game_participation_daily`,
   spec 010) — `(game_id, mode_id, day, count)`, lido via `dailySeries`
   (helper genérico já existente, reaproveitado sem mudança).
2. **UPSERT na mesma tx de `ApplySessionClosed`** — mesma garantia de
   idempotência já usada pro rollup cumulativo (`processed_sessions`
   cobre os dois incrementos, cumulativo e diário, numa tx só).
3. **Total por dia, sem quebra por `closeReason`** — mesmo espírito de
   `game_daily` (não quebra por outcome); quem quiser o detalhe por
   motivo usa o rollup cumulativo (`SessionHealth`), que já tem isso.
   Quebrar a série diária por motivo é uma spec maior (série empilhada),
   fora de escopo até haver demanda real.

## Arquivos por camada

- `migrations/012_session_closed_daily.sql` (novo): `CREATE TABLE
  game_session_daily` (schema do spec.md).
- `internal/domain/analytics.go`: `GameAnalytics.SessionCloseDaily
  []DailyPoint`.
- `internal/repository/postgres_analytics.go`:
  - `ApplySessionClosed`: novo `INSERT ... ON CONFLICT (game_id, mode_id,
    day) DO UPDATE SET count = count + 1` na mesma tx, usando
    `evt.ClosedAt` pra derivar o dia (`(evt.ClosedAt AT TIME ZONE
    'utc')::date` — mesmo padrão de `now() AT TIME ZONE 'utc'` já usado,
    só que a partir do timestamp do evento, não do momento da agregação;
    mais correto pra reprocessamento tardio).
  - `GameAnalytics`: `out.SessionCloseDaily, err = r.dailySeries(ctx,
    "game_session_daily", "count", gameID, days, modeID)`.
- `internal/transport/http_handler.go`: `gameAnalyticsDTO.SessionCloseDaily
  []dailyDTO`.
- Testes: `postgres_analytics_test.go` (integration) — grava e lê a série,
  idempotência, filtro por modo; `http_handler_test.go` — DTO.

## Ordem de implementação

Migration → domínio → repository (write + read) → transport (DTO) →
testes → build/test completo → validação ao vivo (fechar sessões reais em
dias diferentes não é possível de simular sem viajar no tempo — validar
com `ClosedAt` de eventos reais do dia de hoje, 1 ponto na série; conferir
a query com um `UPDATE` manual no Postgres pra simular um dia anterior, se
necessário pra provar a agregação por dia funciona) → admin-bff (028) →
admin-ui (070).

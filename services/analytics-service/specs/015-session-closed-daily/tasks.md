# Tasks — 015 série diária de sessões fechadas

- [x] 1. `migrations/012_session_closed_daily.sql`: `game_session_daily`
- [x] 2. `internal/domain/analytics.go`: `GameAnalytics.SessionCloseDaily`
- [x] 3. `internal/repository/postgres_analytics.go`:
  - [x] 3.1. `ApplySessionClosed`: UPSERT em `game_session_daily` (dia
        derivado de `evt.ClosedAt`, não de `now()`)
  - [x] 3.2. `GameAnalytics`: lê via `dailySeries` reaproveitado
- [x] 4. `internal/transport/http_handler.go`: DTO `sessionCloseDaily`
- [x] 5. Testes:
  - [x] 5.1. Integration: grava + lê a série; idempotência (reentrega não
        dobra); filtro por modo
  - [x] 5.2. Transport: DTO presente/vazio
- [x] 6. `go build ./...` + `go test ./... -race -cover` (unit +
      integration)
- [x] 7. Validação ao vivo: fechar sessão real, conferir
      `sessionCloseDaily` com 1 ponto no dia de hoje
- [x] 8. admin-bff (028): passthrough, validar ao vivo
- [x] 9. admin-ui (070): card pareado com "Saúde das sessões", `raw-stats`
      vira span 12; validar ao vivo (Playwright)

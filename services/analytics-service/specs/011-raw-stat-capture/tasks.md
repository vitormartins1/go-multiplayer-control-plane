# Tasks — 011 captura de stats brutos

## analytics-service

- [x] 1. Migration `009_player_stats.sql`: `game_player_stats` + índice.
- [x] 2. `domain/analytics.go`: `PlayerResult.Stats`,
      `PlacementEntry.{PlayerID,Stats}`, `CollectiveResult.Players
      []CollectivePlayerStat`, `ParticipationEvent.Stats`,
      `GameAnalytics.Stats []StatSummary`.
- [x] 3. `consumer.go`: os 4 parsers extraem `playerId`/`stats` (collective
      passa a ler `players[]` inteiro).
- [x] 4. `postgres_analytics.go`: os 4 `Apply*` gravam `game_player_stats`
      (condicional a stats não-vazio); `GameAnalytics` lê o resumo agregado.
- [x] 5. `http_handler.go`: `gameAnalyticsDTO.stats`.
- [x] 6. Testes: consumer, repository (INSERT condicional + agregação +
      idempotência), http handler. `go test ./services/analytics-service/... -race`.
- [x] 7. Build completo do monorepo + suíte inteira (unit + integration).
- [x] 8. Validação ao vivo: reportar stats nos 4 kinds (win_loss_draw,
      placement, collective, participation_event via grpcurl), conferir
      `stats[]` agregado no `GET .../analytics`.

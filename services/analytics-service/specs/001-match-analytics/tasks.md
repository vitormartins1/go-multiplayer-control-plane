# Tasks: 001-match-analytics

**Plan:** specs/001-match-analytics/plan.md
**Data:** 2026-07-04

- [x] `migrations/001_create_analytics.sql` — processed_matches, game_rollup, game_player_seen, game_daily
- [x] `internal/domain/analytics.go` — tipos (MatchResult/Overview/GameAnalytics/...) + Ports (MatchRepository, Ingestor)
- [x] `internal/repository/postgres_analytics.go` — `ApplyMatch` (tx idempotente), `Overview`, `GameAnalytics` (+ querier)
- [x] teste de integração do repo — agregação, idempotência por matchId, jogadores únicos, overview ordenado
- [x] `internal/service/pool.go` — worker pool (jobs/done channels, Start/Submit/Stop, WaitGroup, métricas)
- [x] teste do pool `-race` — N submits concorrentes agregam certo; Stop drena e encerra
- [x] `internal/service/reader.go` — Overview/GameAnalytics (avgScore)
- [x] `internal/consumer/consumer.go` + `internal/awsx/awsx.go` — SQS long-poll, Submit, delete-após-sucesso, DLQ
- [x] `internal/transport/http_handler.go` — GET /admin/analytics/overview + /admin/games/{gameId}/analytics (+ httptest)
- [x] `cmd/server/main.go` + `Dockerfile` — wire pool+consumer+HTTP; shutdown ordenado (consumer → pool.Stop)
- [x] infra — Terraform (fila match-results-analytics + DLQ + assinatura raw); compose (analytics-postgres + exporter + analytics-service); prometheus job
- [x] `go test ./services/analytics-service/... -race` + `go build ./...` + observabilidade (métricas RED + workers)

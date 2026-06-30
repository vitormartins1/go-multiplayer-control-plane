# Tasks: 003-leaderboard-fanout
**Plan:** specs/003-leaderboard-fanout/plan.md
**Data:** 2026-06-30

- [x] `internal/publisher/leaderboard_publisher.go` (+ httptest) — score por jogador, best-effort, internal key
- [x] `internal/publisher/composite.go` (+ test) — fan-out N sinks; loga falhas, não aborta
- [x] `cmd/server/main.go` — `buildPublisher` monta lista (ACHIEVEMENTS_URL/LEADERBOARD_URL) + Composite; client único
- [x] `infra/docker/docker-compose.yml` — session `LEADERBOARD_URL` + depends_on
- [x] `go test ./... -race` + `go build ./...`; validar no Docker (ReportResult -> leaderboard top-N do jogo)

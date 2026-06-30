# Tasks: 003-leaderboard-fanout
**Plan:** specs/003-leaderboard-fanout/plan.md
**Data:** 2026-06-30

- [ ] `internal/publisher/leaderboard_publisher.go` (+ httptest) — score por jogador, best-effort, internal key
- [ ] `internal/publisher/composite.go` (+ test) — fan-out N sinks; loga falhas, não aborta
- [ ] `cmd/server/main.go` — `buildPublisher` monta lista (ACHIEVEMENTS_URL/LEADERBOARD_URL) + Composite; client único
- [ ] `infra/docker/docker-compose.yml` — session `LEADERBOARD_URL` + depends_on
- [ ] `go test ./... -race` + `go build ./...`; validar no Docker (ReportResult -> leaderboard top-N do jogo)

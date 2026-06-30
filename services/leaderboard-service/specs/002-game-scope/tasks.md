# Tasks: 002-game-scope
**Plan:** specs/002-game-scope/plan.md
**Data:** 2026-06-30

- [ ] `internal/domain/leaderboard.go` — gameID em Service (Submit/Top/Around) e Store (Submit/Rank/Top/Range)
- [ ] `internal/storage/redis_store.go` — key por jogo (`prefix:gameID`) + test (isolamento, testcontainers)
- [ ] `internal/service/leaderboard_service.go` — repassa gameID; `Top(ctx, gameID, limit)` + test
- [ ] `internal/transport/http_handler.go` — player X-Game-Id (vazio->400); admin scores/leaderboard + test
- [ ] `cmd/server/main.go` — guard `RequireInternalKey(key, "/admin")` + key prefix; compose `INTERNAL_API_KEY`; `.http`
- [ ] `go test ./... -race` + `go build ./...`; validar no Docker (isolamento por jogo; admin com/sem key)

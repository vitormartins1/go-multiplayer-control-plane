# Tasks: 002-game-scope
**Plan:** specs/002-game-scope/plan.md
**Data:** 2026-06-30

- [ ] `internal/domain` — gameID nas Ports; `LoadFor`
- [ ] `internal/storage/s3_store.go` — `objectKey(gameID, playerID)`; Put/Get com key composta (+ teste isolamento)
- [ ] `internal/service` — repassa gameID; `LoadFor`
- [ ] `internal/transport` — player X-Game-Id (vazio->400); admin `GET /admin/games/{gameId}/players/{playerID}/save` (404)
- [ ] `cmd/server/main.go` — guard internal key; compose (INTERNAL_API_KEY); `.http`
- [ ] `go test ./... -race` + `go build ./...`; validar no Docker

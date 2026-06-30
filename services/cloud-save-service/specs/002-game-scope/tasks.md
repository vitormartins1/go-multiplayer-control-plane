# Tasks: 002-game-scope
**Plan:** specs/002-game-scope/plan.md
**Data:** 2026-06-30

- [x] `internal/domain` — gameID nas Ports; `LoadFor`
- [x] `internal/storage/s3_store.go` — `objectKey(gameID, playerID)`; Put/Get com key composta (+ teste isolamento)
- [x] `internal/service` — repassa gameID; `LoadFor`
- [x] `internal/transport` — player X-Game-Id (vazio->400); admin `GET /admin/games/{gameId}/players/{playerID}/save` (404)
- [x] `cmd/server/main.go` — guard internal key; compose (INTERNAL_API_KEY); `.http`
- [x] `go test ./... -race` + `go build ./...`; validar no Docker

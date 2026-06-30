# Tasks: 002-game-scope
**Plan:** specs/002-game-scope/plan.md
**Data:** 2026-06-30

- [ ] `migrations/002_add_game_scope.sql` — coluna game_id (default sandbox) + PK nova + índice
- [ ] `internal/domain` — gameID nas Ports; `ListForPlayer` + `Entry`
- [ ] `internal/repository` — queries com game_id; `ListForPlayer` (+ teste integração: isolamento)
- [ ] `internal/service` — repassa gameID; `ListForPlayer` (+ unit)
- [ ] `internal/transport` — player X-Game-Id (vazio->400); admin `GET /admin/games/{gameId}/players/{playerID}/data` (+ unit)
- [ ] `cmd/server/main.go` — guard internal key; compose (INTERNAL_API_KEY + volume migration + `down -v`); `.http`
- [ ] `go test ./... -race` + `go build ./...`; validar no Docker

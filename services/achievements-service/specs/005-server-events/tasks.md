# Tasks: 005-server-events
**Plan:** specs/005-server-events/plan.md
**Data:** 2026-06-29

- [ ] `internal/transport/http_handler.go` — rota `POST /admin/games/{gameId}/players/{playerID}/events` + handler `adminRecordEvent`
- [ ] `internal/transport/http_handler_test.go` — 200 (unlocked); type vazio -> 400; usa path
- [ ] `api/achievements.http` — exemplo com X-Internal-Key
- [ ] `go test ./... -race` + `go build ./...`; validar no Docker (com chave)

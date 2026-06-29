# Tasks: 005-server-events
**Plan:** specs/005-server-events/plan.md
**Data:** 2026-06-29

- [x] `internal/transport/http_handler.go` — rota `POST /admin/games/{gameId}/players/{playerID}/events` + handler `adminRecordEvent`
- [x] `internal/transport/http_handler_test.go` — 200 (unlocked); type vazio -> 400; usa path
- [x] `api/achievements.http` — exemplo com X-Internal-Key
- [x] `go test ./... -race` + `go build ./...`; validar no Docker (com chave)

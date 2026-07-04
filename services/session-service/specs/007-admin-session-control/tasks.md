# Tasks: 007-admin-session-control

**Plan:** specs/007-admin-session-control/plan.md
**Data:** 2026-07-04

- [x] `internal/domain/session.go` — `CloseAdmin CloseReason = "admin"`
- [x] `internal/service/session_service.go` — `AllSessions()` (ativas + histórico, ordenado) (+ test)
- [x] `internal/service/session_service.go` — `CloseByAdmin(ctx, matchID)` via `closeSession(...,CloseAdmin)` (+ test: fecha/libera; inexistente → ErrSessionNotFound)
- [x] `internal/transport/http_handler.go` — `SessionDirectory` +`AllSessions`/`CloseByAdmin`; `gameId` no `sessionDTO`
- [x] `internal/transport/http_handler.go` — `GET /admin/sessions` (filtro state) + `POST /admin/sessions/{matchId}/close` (200 DTO | 404)
- [x] `internal/transport/http_handler_test.go` — testes dos dois endpoints (httptest)
- [x] `go test ./services/session-service/... -race` + `go build ./...`

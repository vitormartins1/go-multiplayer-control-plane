# Tasks: 008-rooms

**Plan:** specs/008-rooms/plan.md
**Data:** 2026-07-04

- [x] `internal/domain/sessions.go` — `GameID`+`GameName` no `GameSession`; `SessionsReader` +`AllSessions`/`CloseSession`
- [x] `internal/domain/admin.go` — tipo `FanoutHealth`
- [x] `internal/session/client.go` — `gameId` no DTO+map; `AllSessions`; `CloseSession` (404→UpstreamError) (+ httptest)
- [x] `internal/service/admin_service.go` — `gameNamesByID`; `Rooms`; `CloseRoom`; `FanoutHealth` (+ testes com fakes)
- [x] `internal/transport/http_handler.go` — `AdminService` +3; rotas `/api/rooms`, `/api/rooms/{matchId}/close`, `/api/health/fanout` (+ testes)
- [x] `go test ./services/admin-bff/... -race` + `go build ./...`

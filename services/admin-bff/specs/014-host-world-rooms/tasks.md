# Tasks: 014-host-world-rooms

Fatia H3 (BFF) — passthrough dos campos host_world no Rooms + endpoint per-game analytics. Ver `plan.md`.

Baseline: `go test ./services/admin-bff/... -race` verde antes.

- [x] **T1 — Rooms fields.** `domain/sessions.go`: `GameSession` += `Origin`, `HostPlayerID` (omitempty), `RoleSlots []RoleSlot` (omitempty); `RoleSlot{Role, Capacity, Occupied}`. `session/client.go`: `sessionDTO` += campos; `toGameSessions` copia. Teste do adapter (fake session server).
- [x] **T2 — Analytics per-game.** `domain/analytics.go`: `GameAnalyticsDetail{AnalyticsGameStat; Daily; RoleParticipations}`, `ParticipationStat{Role, Event, Count}`, `AnalyticsReader.GameAnalytics`. `analytics/client.go`: `GameAnalytics` (GET `/admin/games/{id}/analytics`). Teste httptest (roleParticipations + 502).
- [x] **T3 — service + transport.** `admin_service.go`: `GameAnalytics(gameID, days)` + injeta gameName. `http_handler.go`: `GET /api/analytics/games/{gameId}` (`?days`, default 30). Testes service (fakes) + transport (rota).
- [x] **T4 — verde + lint.** `go test ./services/admin-bff/... -race -cover`; `golangci-lint run`. Rooms/overview sem regressão.

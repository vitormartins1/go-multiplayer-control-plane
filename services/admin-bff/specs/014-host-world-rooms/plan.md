# Plan: 014-host-world-rooms

**Serviço:** admin-bff · **Spec:** `spec.md` (Aprovada) · **ADR:** 0023 (H3, BFF)

Passthrough. Duas peças independentes: (1) campos host_world no Rooms; (2) endpoint per-game analytics.

## Decisões

- **D1 — GameSession += origin/hostPlayerId/roleSlots.** `session/client.go` `sessionDTO` ganha os campos; `toGameSessions` copia (roleSlots é `[]RoleSlot{Role,Capacity,Occupied}`). Zero lógica.
- **D2 — AnalyticsReader += GameAnalytics(gameID, days).** Novo `GameAnalyticsDetail` (resumo + daily + roleParticipations). `analytics/client.go` faz GET `/admin/games/{id}/analytics` e decodifica; adapter mapeia. Erro → `ErrAnalyticsUnavailable` (502).
- **D3 — Service.GameAnalytics** lê e injeta `gameName` (reusa o game-registry client, como o overview faz por jogo). **D4 — Transport** `GET /api/analytics/games/{gameId}` (`?days`, default 30).

## Arquivos

| Arquivo | Mudança |
|---|---|
| `internal/domain/sessions.go` | `GameSession` += campos; `RoleSlot` |
| `internal/session/client.go` | `sessionDTO` += campos; `toGameSessions` |
| `internal/domain/analytics.go` | `AnalyticsReader.GameAnalytics`; `GameAnalyticsDetail`, `ParticipationStat` |
| `internal/analytics/client.go` | `GameAnalytics` (GET per-game) |
| `internal/service/admin_service.go` | `GameAnalytics` (+ gameName) |
| `internal/transport/http_handler.go` | `GET /api/analytics/games/{gameId}` |
| respectivos `_test.go` | adapters (httptest) + service (fakes) + transport |

## Testes

- Baseline verde. Rooms repassa os campos (fake session server). `GameAnalytics`: decodifica roleParticipations, injeta gameName, 502 quando analytics fora. Transport: rota + days default. `-race`.

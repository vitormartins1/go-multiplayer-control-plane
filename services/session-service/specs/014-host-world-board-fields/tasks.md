# Tasks: 014-host-world-board-fields

Fatia H3 (lado session) — expõe `origin`/`hostPlayerId`/`roleSlots` no board admin. Aditivo. Ver `plan.md`.

Baseline: `go test ./services/session-service/... -race` verde antes.

- [x] **T1 — DTO + mapeamento.** `internal/transport/http_handler.go`: `sessionDTO` += `origin`, `hostPlayerId` (omitempty), `roleSlots` (omitempty); `roleSlotDTO{Role, Capacity, Occupied}`; `roleSlotsDTO(map)` ordenado por role; `toSessionDTO` preenche.
- [x] **T2 — testes + verde.** `http_handler_test.go`: host_world (slots ordenados, occupied correto) + queue_arena (origin=match, sem slots). `go test ./services/session-service/... -race`; lint.

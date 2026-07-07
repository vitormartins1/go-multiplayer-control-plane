# Plan: 014-host-world-board-fields

**Serviço:** session-service · **Spec:** `spec.md` (Aprovada) · **ADR:** 0023 (H3, lado session)

Aditivo de serialização no `sessionDTO`. Sem domínio, sem gRPC, sem migration.

## Decisões

- **D1 — `sessionDTO` += `origin`, `hostPlayerId`, `roleSlots`.** `origin` sempre; `hostPlayerId`/`roleSlots` `omitempty`. Novo tipo `roleSlotDTO{Role string; Capacity, Occupied int}`.
- **D2 — `toSessionDTO` mapeia.** `origin = string(s.Origin)`; se `host_world`: `hostPlayerId = s.HostPlayerID` e monta `roleSlots` de `s.RoleSlots` (map → slice), `occupied = len(Occupants)`, **ordenado por role** (coop, invader) para JSON determinístico.
- **D3 — helper `roleSlotsDTO(map) []roleSlotDTO`** ordenado; devolve `nil` (omitido) quando o mapa é vazio (queue_arena).

## Arquivos

| Arquivo | Mudança |
|---|---|
| `internal/transport/http_handler.go` | `sessionDTO` += campos; `roleSlotDTO`; `toSessionDTO` + `roleSlotsDTO` |
| `internal/transport/http_handler_test.go` | host_world com slots; queue_arena sem |

## Testes

- Baseline verde. host_world: `origin`/`hostPlayerId`/`roleSlots` corretos e ordenados. queue_arena: `origin="match"`, sem os demais. `-race`.

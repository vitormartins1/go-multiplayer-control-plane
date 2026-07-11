# Spec: 018-game-delete-passthrough

**Serviço:** admin-bff
**Status:** Done
**Data:** 2026-07-08
**ADR:** 0025 (remoção de jogo — soft delete no registry)
**Companheira de:** `game-registry/010-game-soft-delete`

> Fatia pequena/aditiva (passthrough sem decisão técnica): spec curta, sem plan/tasks.

## Comportamento

- WHEN `DELETE /api/games/{gameId}` → THE SYSTEM SHALL repassar `DELETE /games/{id}` ao game-registry (internal key, via `upstream`) e devolver o código do upstream **intacto**: `204` (deletado), `404` (inexistente/já deletado), `502` (upstream fora).
- Sem corpo de resposta no 204; erros no envelope padrão do BFF (`upstream.ErrorCode`).

## Mudanças

- `internal/gameregistry/client.go`: `DeleteGame(ctx, id string) error` (mesmo padrão do `DeleteMultiplayerMode` — 204 esperado, erro classificado pelo `upstream`).
- `internal/transport/http_handler.go`: rota `DELETE /api/games/{gameId}` → `h.deleteGame`.

## Critérios de aceite

- [ ] (client) `DeleteGame` → nil no 204; `ErrNotFound` no 404; erro classificado em 5xx
- [ ] (handler) `DELETE /api/games/{gameId}` → 204/404/502 conforme upstream
- [ ] Barra verde: `go test ./... -race`, lint 0 issues

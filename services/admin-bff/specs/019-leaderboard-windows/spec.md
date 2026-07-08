# Spec: 019-leaderboard-windows

**Serviço:** admin-bff
**Status:** Rascunho
**Data:** 2026-07-08
**Companheira de:** `leaderboard/007-time-windows`

> Fatia pequena/aditiva (repasse de query params): spec curta, sem plan/tasks.

## Comportamento

- WHEN `GET /api/games/{gameId}/leaderboard` com `?window=` e/ou `?offset=` → THE SYSTEM SHALL repassar os params ao `GET /admin/games/{id}/leaderboard` do leaderboard-service e devolver a resposta como hoje; `400` do upstream (window/board inválido) propagado.
- Params ausentes → comportamento atual intacto (all-time, offset 0).

## Mudanças

- `internal/leaderboard/client.go` (adapter): `GameLeaderboard` ganha `window string, offset int` (vazio/0 = omite da query).
- `internal/domain` + `internal/service` + `internal/transport`: assinatura repassada; handler lê `?window`/`?offset` cru.

## Critérios de aceite

- [ ] window/offset propagados na query do upstream; ausentes → query igual à atual
- [ ] 400 do upstream propagado com código
- [ ] Barra verde: `go test ./... -race`, lint 0

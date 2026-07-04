# Plan: 008-rooms

**Spec:** specs/008-rooms/spec.md
**Data:** 2026-07-04

## Abordagem técnica

Espelha o padrão da aba Sessions (007-sessions-panel): adapter fino no session + enrich no service. Reusa `handlesByID`/enrich (handles) e o `scalar`/PromQL (fan-out health, sem tocar serviços — o adapter Prometheus já tem `Query`). O nome do jogo sai de um `gameNamesByID` (irmão do `handlesByID`, via `registry.ListGames`).

## Arquivos

- `internal/domain/sessions.go` — `GameSession` ganha `GameID` + `GameName`; `SessionsReader` ganha `AllSessions(ctx, state)` e `CloseSession(ctx, matchID) (GameSession, error)`.
- `internal/domain/admin.go` — tipo `FanoutHealth` (ponteiros nil = métrica ausente, como `LiveMetrics`).
- `internal/session/client.go` — `gameId` no `sessionDTO` + no mapeamento; `AllSessions` (`GET /admin/sessions`); `CloseSession` (`POST /admin/sessions/{m}/close` → 1 `sessionDTO`; não-2xx → `UpstreamError{Status}` p/ propagar o 404).
- `internal/service/admin_service.go`:
  - `Rooms(ctx, state)` — `AllSessions` + enrich (handle por player + `GameName` via `gameNamesByID`).
  - `CloseRoom(ctx, matchID)` — delega ao adapter.
  - `FanoutHealth(ctx)` — `errgroup` de `scalar` sobre `sum(fanout_publish_total{result=...})` e `sum(fanout_consume_total{result=...})`.
  - `gameNamesByID(ctx)` — mapa id→nome (best-effort; registry fora → mapa vazio).
- `internal/transport/http_handler.go` — `AdminService` +3 métodos; rotas `GET /api/rooms`, `POST /api/rooms/{matchId}/close`, `GET /api/health/fanout`; handlers (`writeUpstream` no close, `metrics_unavailable`→503 no health).
- Testes: adapter (httptest do session), service (fakes de sessions/metrics/registry), transport.

## Decisões

- **Enrich tolerante** (best-effort): identity/registry fora → sala sem handle/nome, não falha (igual `GameSessions`).
- **Close propaga o status** do session via `UpstreamError` (404 → 404), padrão dos outros adapters.
- **FanoutHealth** usa `sum by (result)` agregando as instâncias dos consumers/produtor; métrica ausente → campo nil (não zero).
- **Board = ativas por default**; `state=closed` traz histórico (mesmo contrato do session).

## Ordem

1. domain (tipos + Port). 2. adapter session (+ httptest). 3. service (Rooms/CloseRoom/FanoutHealth + gameNamesByID) (+ testes com fakes). 4. transport (rotas + interface) (+ testes). 5. `go test -race` + build.

## Riscos

- **N+1 de enrich**: `handlesByID` e `gameNamesByID` são O(páginas), chamados uma vez por request (não por sessão) — ok. Rodar os dois em paralelo se latência incomodar (não no MVP).
- **PromQL divergente**: se os nomes das métricas mudarem, o health degrada para nil silenciosamente — aceitável (mesma semântica do LiveMetrics).

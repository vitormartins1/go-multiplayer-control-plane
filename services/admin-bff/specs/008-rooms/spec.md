# Spec: 008-rooms

**Serviço:** admin-bff
**Status:** Draft
**Data:** 2026-07-04
**Parte de:** Rooms Console (Onda 1) — pares: `session-service/007-admin-session-control`, `admin-ui/009-rooms-console`

## Contexto

Agrega para o painel as capacidades novas do session-service (spec 007): a **visão global de salas ao vivo** e o **force-close**. Também expõe a **saúde do fan-out durável** (ADR-0020) via PromQL — o adapter Prometheus já tem `Query` genérico, então é só mais consultas, sem tocar os serviços.

Hoje o BFF só tem `GET /api/games/{gameId}/sessions` (por-jogo, spec 007-sessions-panel) e `LiveMetrics` com 4 contadores. Esta spec adiciona o board global, a ação de encerrar e os sinais do fan-out.

## Comportamento

### Rooms (board global + ação)

- WHEN `GET /api/rooms[?state=open|closed]` -> THE SYSTEM SHALL devolver as sessões de **todos os jogos** (via `GET /admin/sessions` do session), **enriquecidas**: `playerHandle` por player (reuso do enrich existente do identity) e `gameName` (via game-registry). Default: só ativas (o board é "ao vivo"); `state=closed` traz o histórico recente.
- WHEN `POST /api/rooms/{matchId}/close` -> THE SYSTEM SHALL chamar `POST /admin/sessions/{matchId}/close` no session (com a internal key) e devolver a sessão encerrada. `404` vira `404` (sessão não ativa).

### Saúde do fan-out (ADR-0020)

- WHEN `GET /api/health/fanout` -> THE SYSTEM SHALL devolver, via PromQL instant queries (best-effort, `errgroup`): publicações OK/erro (`fanout_publish_total{result}`), consumos por resultado (`sum by (result) (fanout_consume_total)`) e, se um sink está sem consumir/erra, sinalizar. Métrica ausente -> campo nulo (não derruba a resposta), como o `LiveMetrics` já faz.

## Contratos de Interface

- **Domain/Port `SessionsReader`:** ganha `AllSessions(ctx, state) (GameSessions, error)` e `CloseSession(ctx, matchID) (Session, error)`.
- **Adapter `session/client.go`:** implementa os dois — `GET /admin/sessions` e `POST /admin/sessions/{m}/close` (internal key).
- **Service:** `Rooms(ctx, state)` (lista global + enrich de handle/gameName), `CloseRoom(ctx, matchID)`, `FanoutHealth(ctx)` (PromQL em paralelo).
- **Transport:** `GET /api/rooms`, `POST /api/rooms/{matchId}/close`, `GET /api/health/fanout`.
- Erros do upstream mapeados como nos outros adapters (sentinela → status).

## Casos de Teste Obrigatórios

- [ ] `GET /api/rooms` agrega sessões de vários jogos com `playerHandle` + `gameName`
- [ ] `GET /api/rooms?state=open` -> só ativas
- [ ] `POST /api/rooms/{m}/close` -> proxy com internal key; 404 do session vira 404
- [ ] enrich tolerante: identity/game-registry fora -> devolve a sala sem o handle/nome (não falha)
- [ ] `GET /api/health/fanout` agrega as métricas; métrica ausente -> campo nulo

## Fora do escopo

- WebSocket/push (o board usa polling no cliente, como a aba Sessions atual).
- Profundidade real da DLQ (`ApproximateNumberOfMessages`) — precisa de um exporter de métricas do SQS; fica como refinamento (aqui só publish/consume do Prometheus).

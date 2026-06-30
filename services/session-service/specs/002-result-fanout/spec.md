# Spec: 002-result-fanout

**Serviço:** session-service
**Status:** Approved
**Data:** 2026-06-30

## Contexto

Fecha o gameplay loop: substitui o **publisher stub** (que só loga) por um **fan-out real** do resultado da partida (ADR-0010). Quando o game server chama `ReportResult`, o session olha a sessão (tem `game_id` e players — peça 3) e, para cada resultado, chama o achievements server-side (peça 2, `005-server-events`) **injetando a internal key** (ADR-0018), traduzindo o resultado em eventos de gameplay.

## Comportamento

- WHEN `ReportResult{matchId, results[]}` -> libera o game server, encerra a sessão e dispara o **fan-out**: para cada `result`, envia ao achievements os eventos do jogo da sessão.
- THE SYSTEM SHALL traduzir cada resultado em eventos: **`match_played`** para todos os jogadores; **`match_won`** para os que venceram (`won=true`).
- THE SYSTEM SHALL usar o `game_id` guardado na sessão no fan-out (`POST /admin/games/{gameId}/players/{playerID}/events`, com `X-Internal-Key`).
- THE SYSTEM SHALL **liberar o server e encerrar a sessão independentemente** do fan-out (uma falha de fan-out não vaza server nem falha o report — best-effort, com log).
- IF não existir sessão para o `matchId` -> `NotFound` (inalterado).

## Contratos de Interface

- `ResultPublisher.Publish(ctx, gameID string, results []PlayerResult) error` — passa a receber o `gameID` (era `matchID`); o adapter real chama o achievements.
- Novo adapter `internal/publisher/achievements_publisher.go` (HTTP) que satisfaz `ResultPublisher`. O `LogPublisher` segue como fallback (sem `ACHIEVEMENTS_URL`).

## Casos de Teste Obrigatórios

- [ ] `ReportResult` -> para cada jogador envia `match_played`; para `won=true` envia também `match_won`
- [ ] o fan-out usa o `game_id` da sessão e o `X-Internal-Key`
- [ ] achievements indisponível -> o server é liberado e a sessão encerra mesmo assim (best-effort)
- [ ] `matchId` inexistente -> `NotFound`

## Fora do escopo

- **Leaderboard** no fan-out — feature futura (mesmo padrão)
- Retry/outbox/garantia de entrega — best-effort no MVP
- Mapeamento rico de eventos (kills, score) — só `match_played`/`match_won` no MVP

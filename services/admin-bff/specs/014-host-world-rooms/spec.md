# Spec: 014-host-world-rooms

**Serviço:** admin-bff
**Status:** Aprovada (2026-07-07)
**Data:** 2026-07-07
**ADR:** 0023 — **arquétipo `host_world_dropin`, fatia H3** (surfacing — lado BFF)
**Par de:** `session-service/014-host-world-board-fields`, `admin-ui/020-host-world-console`
**Depende de:** `session/014` (board expõe origin/host/slots), `analytics/004` (`roleParticipations` no detalhe por jogo).

## Contexto

O BFF é **passthrough** (ADR: agrega/enriquece, não decide). A H3 precisa de duas coisas dele:

1. **Rooms por papel:** o board de Rooms (`admin-bff/008`) repassa o `sessionDTO` do session; agora que o session/014 carrega `origin`/`hostPlayerId`/`roleSlots`, o BFF precisa **passá-los adiante** no `GameSession`.
2. **Analytics por jogo:** o console mostra o detalhe de analytics por jogo **filtrando o overview** no cliente — não há endpoint per-game no BFF. Para exibir `roleParticipations` (que vive só no detalhe `GameAnalytics` do analytics-service), o BFF ganha um **passthrough per-game**: `GET /api/analytics/games/{gameId}` → `GET /admin/games/{id}/analytics` do analytics.

## Comportamento

### Rooms — campos host_world (passthrough + enrich existente)

- WHEN `GET /api/rooms` / `GET /api/games/{id}/sessions` → THE SYSTEM SHALL incluir em cada sessão `origin`, `hostPlayerId` e `roleSlots` (do session/014), além do enrich já existente (`playerHandle`, `gameName`). `queue_arena` → `origin="match"`, sem os demais (omitidos).

### Analytics por jogo (novo passthrough)

- WHEN `GET /api/analytics/games/{gameId}` → THE SYSTEM SHALL ler `GET /admin/games/{gameId}/analytics` do analytics-service e devolver o detalhe do jogo — resumo (matches/wins/…), série diária e **`roleParticipations`** (por papel/evento; vazio no queue_arena) — enriquecido com `gameName` (game-registry). Analytics fora → `502` (`ErrAnalyticsUnavailable`), como o overview.

## Contratos de Interface

- **`domain.GameSession`** += `Origin string`, `HostPlayerID string (omitempty)`, `RoleSlots []RoleSlot (omitempty)` com `RoleSlot{Role string; Capacity, Occupied int}`.
- **`session/client.go`** `sessionDTO` += os campos; `toGameSessions` mapeia.
- **`domain.AnalyticsReader`** += `GameAnalytics(ctx, gameID string, days int) (GameAnalyticsDetail, error)`; novo tipo `GameAnalyticsDetail{ AnalyticsGameStat; Daily []AnalyticsDaily; RoleParticipations []ParticipationStat }` com `ParticipationStat{Role, Event string; Count int64}`.
- **`analytics/client.go`** += `GameAnalytics` (GET `/admin/games/{id}/analytics`).
- **Service** += `GameAnalytics(ctx, gameID, days)` (lê + injeta gameName).
- **Transport** += `GET /api/analytics/games/{gameId}` (`?days=`, default 30).

## Fora de escopo

- **Quadro de sinais ativo** — follow-up (sem leitura no matchmaking).
- Mudança no analytics-service (já expõe `roleParticipations`).

## Critérios de Aceite

- [ ] Rooms/sessions repassa `origin`/`hostPlayerId`/`roleSlots`; queue_arena sem os host_world.
- [ ] `GET /api/analytics/games/{id}` devolve resumo + daily + `roleParticipations` + `gameName`; analytics fora → 502.
- [ ] Testes (httptest dos adapters + service com fakes); `-race`; sem regressão do overview/rooms.

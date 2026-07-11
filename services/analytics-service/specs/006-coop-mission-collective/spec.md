# Spec: 006-coop-mission-collective

**Serviço:** analytics-service
**Status:** Done
**Data:** 2026-07-07
**ADR:** 0023 — arquétipo `coop_mission`
**Roadmap:** `docs/game-models/coop_mission-roadmap.md` — fatia **C3** (sink)
**Depende de:** C2 (`session/016` — envelope `kind=collective`)

## Contexto

Sink do `coop_mission` (C3). O consumer de analytics já despacha por `kind` (specs 003/004/005) e agrega em rollups Postgres via worker pool, idempotente por `matchId` (`processed_matches`). Esta fatia adiciona o ramo `collective`: agrega, por jogo, a **distribuição de desfecho da missão** (sucessos/falhas → taxa de sucesso). Espelha a BR3 (`analytics/005`, `game_placement_rollup`).

## Comportamento

- WHEN chega um envelope `kind=collective` (`{matchId, gameId, outcome, ...}`) → THE SYSTEM SHALL agregar, por `gameId`, **+1 em `successes`** se `outcome == MISSION_SUCCESS` ou **+1 em `failures`** se `MISSION_FAILURE`, idempotente por `matchId` (uma missão = um `ReportResult`). O desfecho é **da sessão** (uma contagem por partida, não por jogador — o analytics ignora `players`).
- WHEN o `matchId` já foi processado → no-op (dup).
- WHEN `GET /admin/games/{id}` (analytics do jogo) → THE SYSTEM SHALL incluir `mission` com `{successes, failures, successRate}` quando houver missões; ausente (nil) fora do `coop_mission`.
- THE SYSTEM SHALL manter `win_loss_draw`/`participation_event`/`placement` **sem regressão**.

## Contratos de Interface

### Migration — `005_mission_rollup.sql`

- `game_mission_rollup(game_id TEXT PRIMARY KEY, successes BIGINT DEFAULT 0, failures BIGINT DEFAULT 0)`. Idempotência reusa `processed_matches`. **Montar no docker-compose** (`analytics-postgres`) — junto das 002–004 que estavam faltando o mount (correção do gap).

### Domínio

- `CollectiveResult{ MatchID, GameID, Outcome string }` (o analytics só precisa do desfecho da sessão).
- `MissionDistribution{ Successes, Failures int64; SuccessRate float64 }` (SuccessRate derivado no read). `GameAnalytics` ganha `Mission *MissionDistribution` (nil fora do coop_mission).
- `MatchRepository` += `ApplyCollective(ctx, CollectiveResult) (dup, err)`; `Ingestor` += `SubmitCollective(ctx, CollectiveResult) <-chan error`.

### Pool + Consumer

- `job` ganha `collective *CollectiveResult`; `process` despacha; `SubmitCollective` enfileira.
- `consumer.go`: `case kind == "collective"` → `parseCollective` → `pool.SubmitCollective`.

### HTTP (read)

- `gameAnalyticsDTO` ganha `mission *missionDTO` (`successes`/`failures`/`successRate`), `omitempty`. Render no console = C4; aqui só o campo no JSON.

## Casos de Teste Obrigatórios

- [ ] (repo, integração) `ApplyCollective` SUCCESS → successes+1; FAILURE → failures+1; reprocessar matchId → no-op; `GameAnalytics.Mission` devolve successRate = successes/(successes+failures)
- [ ] (consumer) `parseCollective` lê `{matchId, gameId, outcome}`; `kind=collective` → `SubmitCollective`
- [ ] regressão: outros kinds seguem

## Fora do escopo

- Render no console (C4, admin-ui + bff). `teamId`/stats agregados (follow-up).

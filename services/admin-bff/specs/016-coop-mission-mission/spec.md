# Spec: 016-coop-mission-mission

**Serviço:** admin-bff
**Status:** Aprovado (batch C1–C4)
**Data:** 2026-07-07
**ADR:** 0023 — arquétipo `coop_mission`
**Roadmap:** `docs/game-models/coop_mission-roadmap.md` — fatia **C4** (console)
**Depende de:** C3 (`analytics/006` — campo `mission` no analytics do jogo)

## Contexto

Fatia console (C4) do `coop_mission`, lado BFF. Espelha a BR4 (`admin-bff/015`, que propagou `placement`). O BFF é **passthrough** da read API do analytics: adiciona só o campo `mission` no `GameAnalyticsDetail`. O perfil de multiplayer já flui cru (`json.RawMessage`, achado da B1/BR4) — `coop_mission` não precisa de tipagem no BFF.

## Comportamento

- WHEN o BFF lê `GET /admin/games/{id}/analytics` do analytics-service e a resposta traz `mission` (`{successes, failures, successRate}`) → THE SYSTEM SHALL **propagar** esse campo no `GameAnalyticsDetail` do `GET /api/analytics/games/{id}`; ausente (nil) fora do `coop_mission`.
- THE SYSTEM SHALL manter `placement`/`roleParticipations`/resumo **sem regressão**.

## Contratos de Interface

- `domain.GameAnalyticsDetail` ganha `Mission *MissionDistribution` (`json:"mission,omitempty"`); `MissionDistribution{Successes, Failures int64; SuccessRate float64}`. O analytics client decodifica o campo; o handler já escreve a struct direto (passthrough).

## Casos de Teste Obrigatórios

- [ ] (analytics client) upstream com `mission` → `GameAnalyticsDetail.Mission` populado; sem `mission` → nil
- [ ] regressão: `placement` segue

## Fora do escopo

- Perfil `coop_mission` (já flui cru). Render (admin-ui). `teamId`/stats agregados.

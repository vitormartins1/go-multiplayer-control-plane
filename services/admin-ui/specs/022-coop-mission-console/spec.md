# Spec: 022-coop-mission-console

**Serviço:** admin-ui
**Status:** Aprovado (batch C1–C4)
**Data:** 2026-07-07
**ADR:** 0023 — arquétipo `coop_mission`
**Roadmap:** `docs/game-models/coop_mission-roadmap.md` — fatia **C4** (console)
**Depende de:** C4/BFF (`admin-bff/016` — campo `mission`), C0-UI (form já feito na C0)

## Contexto

Fatia console (C4) do `coop_mission`, lado UI. Espelha a BR4 (`admin-ui/021`). O **form de perfil** `coop_mission` já entrou na C0 (`MultiplayerSettings.tsx` + union em `api.ts`) — aqui só o **surfacing**: a distribuição de desfecho da missão na Analytics e o rótulo co-op no Rooms.

## Comportamento

- WHEN a Analytics de um jogo traz `mission` (`{successes, failures, successRate}`) → THE SYSTEM SHALL mostrar um bloco condicional "Desfecho das missões" com missões cumpridas, falhas e taxa de sucesso (%). Ausente fora do `coop_mission`.
- WHEN uma sala (Rooms) é de um jogo `coop_mission` → THE SYSTEM SHALL mostrar um badge `co-op <jogadores>/<teamSize>` (perfil por `gameId`, cacheado — reusa o mecanismo do badge BR).
- THE SYSTEM SHALL manter a distribuição de colocação (BR) e os demais surfacings **sem regressão**.

## Contratos de Interface

- `api.ts`: `GameAnalyticsDetail` ganha `mission?: MissionDistribution`; interface `MissionDistribution{successes, failures, successRate}`.
- `GameAnalytics.tsx`: bloco condicional `data?.mission`.
- `Rooms.tsx`: `coopInfo(session)` (perfil `coop_mission` → `{teamSize}`) + badge `co-op N/teamSize`.

## Casos de Teste Obrigatórios

- [ ] `npm run build` (tsc --noEmit && vite build) verde com os tipos novos.
- [ ] (manual) Analytics de um jogo coop mostra o bloco; Rooms mostra o badge co-op.

## Fora do escopo

- Missão/dificuldade ao vivo na sessão (Abertura). Backend (C0–C3).

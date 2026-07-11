# Spec: 006-coop-mission-collective

**Serviço:** leaderboard-service
**Status:** Done
**Data:** 2026-07-07
**ADR:** 0023 — arquétipo `coop_mission`
**Roadmap:** `docs/game-models/coop_mission-roadmap.md` — fatia **C3** (sink)
**Depende de:** C2 (`session/016` — envelope `kind=collective`)

## Contexto

Sink do `coop_mission` (C3). O consumer de leaderboard já despacha por `kind` (specs 003/004/005): `win_loss_draw` e `placement` alimentam boards; kinds sem ranking são descartados. Esta fatia adiciona o ramo `collective`: as métricas por jogador viram highscore boards + boards cumulativos, e o sucesso da missão vira um board cumulativo `missions_completed`. Zero infra nova (Redis, sem migration); reusa `Submit` (highscore ZADD GT) e `ApplyCumulative` (ZINCRBY idempotente por matchID, script Lua atômico).

## Comportamento

- WHEN chega um envelope `kind=collective` (`{matchId, gameId, outcome, players[]}`) → THE SYSTEM SHALL, para **cada** jogador:
  - highscore: um board por `stat` presente (`ValidBoard`), via `Submit` (melhor de uma partida);
  - cumulative: `<stat>_total` (soma na temporada);
  - cumulative `missions_completed` (+1) **se** `outcome == MISSION_SUCCESS` — o desfecho é da sessão, então **todos** os jogadores ganham +1 numa missão cumprida.
  - as parcelas cumulativas vão numa única `ApplyCumulative` (idempotente por `matchId`).
- WHEN `outcome == MISSION_FAILURE` → nenhum `missions_completed` (mas os stats highscore/total ainda contam — performance vale mesmo na falha).
- THE SYSTEM SHALL manter `win_loss_draw`/`placement` **sem regressão**; kind não rankeável (participation_event) segue descartado; `matchId` reprocessado → no-op.

## Contratos de Interface

### Consumer (cmd/consumer/main.go)

- Novo `case kind == "collective"` → `handleCollective`, que desserializa `{matchId, gameId, outcome, players[]{playerId, stats}}`.

### Domínio

- const `BoardMissionsCompleted = "missions_completed"` (board cumulativo do coop_mission).

## Casos de Teste Obrigatórios

- [ ] (consumer/handler) `collective` SUCCESS → cada jogador: Submit por stat + `<stat>_total` + `missions_completed`+1; ApplyCumulative uma vez por matchId
- [ ] (handler) `collective` FAILURE → stats contam, `missions_completed` **não** incrementa
- [ ] (handler) reprocessar mesmo `matchId` → dup (no-op)
- [ ] regressão: `win_loss_draw`/`placement` seguem; `participation_event` descartado

## Fora do escopo

- Console/seletor de board (C4). Migration (Redis). `teamId` (follow-up).

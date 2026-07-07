# Spec: 011-coop-mission-collective

**Serviço:** achievements-service
**Status:** Aprovado (batch C1–C4)
**Data:** 2026-07-07
**ADR:** 0023 — arquétipo `coop_mission`
**Roadmap:** `docs/game-models/coop_mission-roadmap.md` — fatia **C3** (sink)
**Depende de:** C2 (`session/016` — envelope `kind=collective`)

## Contexto

Sink do `coop_mission` (C3). O consumer de achievements já despacha o envelope do fan-out por `kind` (specs 007/009/010): `win_loss_draw` → `ApplyMatchResult`, `participation_event` → `ApplyParticipationEvent`, `placement` → `ApplyPlacementResult`. Esta fatia adiciona o ramo `collective`: o desfecho coletivo da missão vira event types de gameplay que o catálogo de conquistas casa. Zero infra nova (o SNS já faz fan-out para a fila; o dispatch por kind já existe) — só mais um ramo, espelhando a BR3 (`010`).

## Comportamento

- WHEN chega um envelope `kind=collective` (`{matchId, gameId, outcome, players[]}`) → THE SYSTEM SHALL, para **cada** jogador, gerar os event types `coop_mission_played` (todos) **+** `coop_mission_success` (se `outcome == MISSION_SUCCESS`) **ou** `coop_mission_failed` (se `outcome == MISSION_FAILURE`), passar os `stats` do jogador, e aplicar de forma **idempotente por `matchId`** (reusa `processed_matches`, sem migration).
- O desfecho é **da sessão** — todos os jogadores recebem o mesmo par played+success/failed; os `stats` seguem por jogador (critério por-métrica da spec 008 reusado: kills/revives/etc.).
- WHEN o `outcome` não estiver no vocabulário → só `coop_mission_played` (não falha o processamento; o session já validou, isto é rede de segurança).
- THE SYSTEM SHALL manter `win_loss_draw`/`participation_event`/`placement` **sem regressão**; `matchId` reprocessado → no-op (dup).

## Contratos de Interface

### Consumer (cmd/consumer/main.go)

- Novo `case head.Kind == "collective"` → `handleCollective`, que desserializa `{matchId, gameId, outcome, players[]{playerId, stats}}` e chama `svc.ApplyCollectiveResult`.

### Domínio — novo `collective.go`

- consts `EventCoopMissionPlayed = "coop_mission_played"`, `EventCoopMissionSuccess = "coop_mission_success"`, `EventCoopMissionFailed = "coop_mission_failed"`; consts `MissionSuccess`/`MissionFailure`.
- `CollectivePlayer{ PlayerID string; Stats map[string]int64 }`.
- `CollectiveEventTypes(outcome string) []string` (função pura; espelha `PlacementEventTypes`): `played` sempre; `+success` ou `+failed` conforme o desfecho da sessão.

### Serviço

- `ApplyCollectiveResult(ctx, gameID, matchID, outcome string, players []domain.CollectivePlayer) (bool, error)`: monta `PlayerMatchInput{PlayerID, EventTypes: CollectiveEventTypes(outcome), Stats}` para cada jogador e delega ao `repo.ApplyMatchResult` (idempotente por matchID). Espelha `ApplyPlacementResult`.

## Casos de Teste Obrigatórios

- [ ] (domain) `CollectiveEventTypes(MISSION_SUCCESS)` → `[played, success]`; `(MISSION_FAILURE)` → `[played, failed]`; desconhecido → `[played]`
- [ ] (service) `ApplyCollectiveResult` SUCCESS → cada jogador recebe played+success, stats repassados; delega ao repo
- [ ] (service) reprocessar mesmo `matchId` → dup (no-op)
- [ ] (consumer) envelope `kind=collective` → `handleCollective`; regressão dos outros kinds

## Fora do escopo

- Console (C4). Migration (reusa `processed_matches`). Conquistas por `teamId` (follow-up, como na BR3).

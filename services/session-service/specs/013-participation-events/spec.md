# Spec: 013-participation-events

**Serviço:** session-service
**Status:** Done
**Data:** 2026-07-07
**ADR:** 0023 (perfil de multiplayer por jogo) — **arquétipo `host_world_dropin`, fatia H2** (lado session: resultado por evento/papel)
**Roadmap:** `docs/game-models/host_world_dropin-roadmap.md` → H2
**Depende de:** `session/011` (mundo host-cêntrico, slots por papel) e o fan-out do `queue_arena` (ADR-0020: tópico `match-results`, envelope versionado).

## Contexto

No `queue_arena` a partida tem **um** resultado terminal: o server chama `ReportResult(matchID, outcome)` **uma vez**, o session publica o `MatchOutcome` no tópico `match-results` e **encerra** a sessão. É o modelo `win_loss_draw` (ADR-0023).

O `host_world_dropin` (ficha 01, §5) **quebra isso**: não há placar de partida. O mundo é persistente e cada participante tem um **desfecho por papel**, reportado **ao longo da vida** da sessão — o co-op ajudou a matar o boss, o invasor matou o host, um phantom morreu ou saiu. São **eventos por participante**, não um outcome único no fim.

Esta fatia (H2, lado session) entrega o **produtor**: uma nova operação `ReportParticipationEvent` pela qual o server do mundo reporta cada evento; o session aplica o **efeito no slot** quando o evento o exige e **publica** o evento no mesmo tópico `match-results`, com um **envelope discriminado por `kind`** (decisão #3 do roadmap) — retrocompatível com os consumers do `queue_arena`. Os **sinks** que reagem a esses eventos (achievements por papel, analytics por participação) são as **sub-fatias seguintes** da H2 (specs próprias em achievements/analytics), que consomem o envelope definido aqui.

> **Escopo:** só o produtor (session). O `ReportResult` terminal (`queue_arena`) fica **inalterado** — é outro caminho. Nada de placar/ranking de invasões aqui (leaderboard é cauda opcional da H2).

## Comportamento

### Reportar um evento de participação

- WHEN o server do mundo chama `ReportParticipationEvent(sessionID, {playerID, role, event, outcome})` para uma **sessão host_world aberta** → THE SYSTEM SHALL (1) **publicar** o evento no tópico `match-results` com `kind=participation_event` (envelope abaixo) e, **só após a publicação bem-sucedida**, (2) aplicar o **efeito no slot** correspondente ao `event`. A ordem (publica → muta) espelha o `ReportResult` (ADR-0020): se a publicação falha, o estado **não** muda e o server refaz o report (retry) — o evento não se perde.
- THE SYSTEM SHALL aceitar os eventos `event ∈ {JOINED, BOSS_DEFEATED, HOST_KILLED, DIED, LEFT}` e os desfechos `outcome ∈ {SUCCESS, FAIL, NEUTRAL}`. `event`/`outcome` fora desses conjuntos → **ErrInvalidParticipation** (`InvalidArgument`). O `role` viaja como reportado (o server o lê do claim `role` do connect token, H0) — o session não o revalida contra o slot.
- IF não existe **sessão host_world aberta** com o `sessionID` (nunca existiu, ou já foi encerrada) → THE SYSTEM SHALL responder **ErrWorldNotFound** (`NotFound`). É também o que o server recebe ao reenviar um evento para um mundo já encerrado (ex.: segundo `HOST_KILLED`) — sinal para parar o retry.

### Efeito de cada evento no mundo

- THE SYSTEM SHALL aplicar o efeito estrutural **por tipo de evento**, deixando a interpretação de "quem venceu" para os sinks (o desfecho por papel não é decisão do session):

| `event` | Efeito no mundo | Observação |
|---|---|---|
| `JOINED` | nenhum | o slot já foi ocupado no `JoinWorld` (H0); `JOINED` é a confirmação do server. |
| `BOSS_DEFEATED` | nenhum | sinal de progresso (co-op ajudou); o phantom pode sair depois com `LEFT`. |
| `DIED` | **libera o slot** do `playerID` | phantom morto (invasor abatido / co-op morto) — igual ao `LeaveWorld` do phantom. |
| `LEFT` | **libera o slot** do `playerID` | saída voluntária do phantom; o mundo segue aberto. |
| `HOST_KILLED` | **encerra o mundo** (`CloseReason=host_killed`) | o host morreu — encerra a participação de todos os phantoms, libera o server. |

- WHEN `event=DIED` ou `event=LEFT` para um `playerID` que **não** ocupa slot (evento duplicado/atrasado) → THE SYSTEM SHALL tratar como **no-op** idempotente (libera nada, não erra) — o efeito de slot é tolerante a replay.
- WHEN `event=HOST_KILLED` → THE SYSTEM SHALL encerrar o mundo com o novo motivo `host_killed` (distinto de `host_left`, que é a saída voluntária da H0), reusando o fechamento unificado (libera server, entra no histórico).

### Retrocompatibilidade do tópico (decisão #3)

- THE SYSTEM SHALL publicar os eventos de participação no **mesmo** tópico `match-results`, com `kind=participation_event`, para **não** criar um segundo tópico/infra. O envelope do `queue_arena` (`kind=win_loss_draw`) fica **inalterado**.
- OBSERVAÇÃO (contrato para os consumers): como o SNS faz fan-out para **todas** as filas assinantes, os consumers atuais (achievements/analytics/leaderboard) passam a **receber** também os `participation_event`. Eles hoje **não inspecionam `kind`** — então **as sub-fatias de consumer da H2 adicionam o dispatch por `kind`** (ignorar o que não sabem tratar; reagir ao que sabem). Esta spec **define o envelope**; a robustez dos sinks é entregue nessas sub-fatias, na ordem da dependência (o envelope assenta aqui primeiro).

## Contratos de Interface

### Envelope publicado (`kind=participation_event`)

```json
{
  "schemaVersion": 2,
  "kind": "participation_event",
  "gameId": "<jogo>",
  "sessionId": "<mundo>",
  "playerId": "<participante>",
  "role": "host | coop | invader",
  "event": "JOINED | BOSS_DEFEATED | HOST_KILLED | DIED | LEFT",
  "outcome": "SUCCESS | FAIL | NEUTRAL"
}
```

Sem `matchId`/`results` (campos do `win_loss_draw`). Um consumer que despacha por `kind` lê só o que o `kind` dele exige.

### Protobuf (`session/v1/session.proto`)

- Novo RPC **unary** `ReportParticipationEvent(ReportParticipationEventRequest{ string session_id; string player_id; string role; string event; string outcome }) → ReportParticipationEventResponse{}`.
- `ReportResult`/`CreateSession`/`OpenWorld`/`JoinWorld`/`LeaveWorld`/`CloseWorld`/`ListInvadableWorlds` — **inalterados**. Regenerar com `buf`.

### Ports (domínio)

- **Novo** `ParticipationEvent{ PlayerID, Role, Event, Outcome string }` + constantes dos vocabulários (`EventJoined`/`EventBossDefeated`/`EventHostKilled`/`EventDied`/`EventLeft`; `OutcomeSuccess`/`OutcomeFail`/`OutcomeNeutral`).
- **`SessionService`** ganha `ReportParticipationEvent(ctx, sessionID string, evt ParticipationEvent) error`.
- Novo `CloseReason` `host_killed`.
- **Publicação:** decisão do plan — nova Port `ParticipationPublisher` (`PublishParticipation(ctx, gameID, sessionID string, evt ParticipationEvent) error`) implementada pelo **mesmo** adapter SNS (e um Log fallback), OU um método a mais no `ResultPublisher`. O `ResultPublisher.Publish` (queue_arena) fica intacto.

### Erros de domínio (novos)

- `ErrInvalidParticipation` → `InvalidArgument` (event/outcome fora do vocabulário). Reusa `ErrWorldNotFound`.

## Modelo de Dados

Sessão continua **em memória** (H0). Nenhum estado novo persistente: um evento de participação é **publicado e (talvez) muta um slot**, não é armazenado. A idempotência ponta a ponta vive **no consumer**, por `(sessionID, playerID, event)` (decisão #3, entregue nas sub-fatias de consumer). **Sem migration.**

## Observabilidade

- `participation_events_total{event}` (counter) — eventos por tipo, o pulso do modo host_world. (Detalhe/onde registrar: plan.)
- Reusa o `fanout_publish_total{result}` do publisher para ok/erro da publicação.
- Tracing: `ReportParticipationEvent` no otelgrpc global; o publish herda o span.

## Fora de escopo

- **Consumers** (achievements por papel, analytics por participação) — sub-fatias próprias da H2, consomem este envelope.
- **Leaderboard de invasões/co-ops** — cauda opcional da H2.
- **Persistência dos eventos** no session — não; o histórico por participante, se necessário, é responsabilidade dos sinks (analytics).
- **Revalidar `role`/`outcome` contra a semântica de papel** ("invasor não pode reportar BOSS_DEFEATED") — o server do mundo é autoritativo sobre o que aconteceu; o session valida só o vocabulário.
- **Contrato do data plane** (`host_world_dropin-dataplane-contract.md`) — doc na cauda da H2 (o que o server Unity chama/emite), depois que o envelope e os consumers assentarem.

## Critérios de Aceite

- [ ] `ReportParticipationEvent` num mundo aberto publica o envelope `kind=participation_event` (com sessionId/playerId/role/event/outcome) **antes** de mutar o slot.
- [ ] `DIED`/`LEFT` de um phantom liberam o slot dele; o mundo segue aberto; `DIED`/`LEFT` de quem não está no mundo → no-op.
- [ ] `HOST_KILLED` encerra o mundo (`CloseReason=host_killed`), libera o server, entra no histórico.
- [ ] `JOINED`/`BOSS_DEFEATED` publicam sem mexer no slot.
- [ ] `event`/`outcome` fora do vocabulário → `ErrInvalidParticipation`; `sessionID` inexistente/fechado → `ErrWorldNotFound`.
- [ ] Falha de publicação → erro, **sem** mutar o slot (o server refaz o report).
- [ ] `queue_arena` (`ReportResult`, 001–010) e H0/H1a (011/012) **sem regressão**; testes com `-race`, ≥80% em `service/`/`domain/`.

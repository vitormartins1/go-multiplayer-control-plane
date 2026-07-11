# Spec: 009-participation-consumer

**Serviço:** achievements-service
**Status:** Superseded by ADR-0027 (motor genérico de event_rules, migration `008_event_rules.sql`) — a fatia H2 (dispatch por kind, idempotência por sessionId/playerId/event, sink `participation_event`→`ApplyParticipationEvent`) está implementada e viva, mas a função pura hardcoded `ParticipationEventTypes` desta spec foi substituída por `ParticipationFacts`+`EvaluateRules` (regra data-driven, ver spec `013-mode-fact`)
**Data:** 2026-07-07
**ADR:** 0023 (perfil de multiplayer por jogo) — **arquétipo `host_world_dropin`, fatia H2** (sink: conquistas por papel/evento)
**Roadmap:** `docs/game-models/host_world_dropin-roadmap.md` → H2
**Depende de:** `session/013` (produtor — envelope `kind=participation_event` no tópico `match-results`) e a A1/008 (critério por evento/stat, reaproveitado).

## Contexto

O achievements consome hoje **um** tipo de mensagem do tópico `match-results`: o `win_loss_draw` (`queue_arena`). O consumer SQS (`cmd/consumer`) desserializa `{matchId, gameId, results[]}` e chama `ApplyMatchResult`, idempotente por `matchId` (tabela `processed_matches`), traduzindo o resultado em eventos de gameplay (`match_played`/`match_won`/…) que o catálogo casa por `event_type`.

A H2 (session/013) passou a publicar, **no mesmo tópico**, um segundo `kind`: `participation_event` (`{sessionId, playerId, role, event, outcome}`), sem `matchId`/`results`. Como o SNS faz fan-out para **todas** as filas, a fila do achievements **já recebe** esses envelopes — e o consumer atual, que não olha o `kind`, os trataria como um `win_loss_draw` vazio (`matchId=""`), poluindo `processed_matches`. Esta fatia conserta isso e **dá sentido** ao novo evento: conquistas **por papel** no `host_world_dropin` — "venceu como invasor", "ajudou a matar o boss como co-op".

> **Escopo:** só o achievements (sink). O analytics é sub-fatia irmã (`analytics/004`). O leaderboard de invasões/co-ops é cauda.

## Comportamento

### Dispatch por `kind` (retrocompat)

- WHEN chega uma mensagem no tópico → THE SYSTEM SHALL ler o campo `kind` do envelope e **despachar**: `win_loss_draw` → o caminho atual (`ApplyMatchResult`, inalterado); `participation_event` → o novo caminho (`ApplyParticipationEvent`); **qualquer outro/ausente** → **ignorar** (no-op, sem erro, sem ir à DLQ). É a robustez que a decisão #3 do roadmap pressupõe.

### Aplicar um evento de participação

- WHEN chega um `participation_event` `{gameId, sessionId, playerId, role, event, outcome}` → THE SYSTEM SHALL traduzi-lo em zero ou mais **event types de gameplay** (por papel; mapa abaixo) e aplicar o progresso das conquistas que casam por `event_type` (reusa o critério da A1), de forma **idempotente** pela chave `(sessionId, playerId, event)`.
- THE SYSTEM SHALL registrar a marca de idempotência `(sessionId, playerId, event)` e o progresso na **mesma transação** (como `ApplyMatchResult` faz com `processed_matches`): reprocessar (entrega at-least-once do SQS) é **no-op**; um crash no meio não deixa o evento "processado" sem efeito.
- WHEN o mapeamento não produz nenhum event type (evento sem conquista associada, ex.: `LEFT`) → THE SYSTEM SHALL **marcar como processado** e não aplicar nada (no-op idempotente) — o evento não volta à fila.

### Mapa papel × evento → event types (a política do sink)

Função **pura e testável** `ParticipationEventTypes(role, event, outcome) []string`:

| `role` | `event` | `outcome` | event types emitidos |
|---|---|---|---|
| `invader` | `HOST_KILLED` | `SUCCESS` | `invasion_won` |
| `coop` | `BOSS_DEFEATED` | `SUCCESS` | `coop_boss_helped` |
| qualquer | `JOINED` | qualquer | `world_joined` |
| qualquer | (demais) | — | (nenhum) |

- THE SYSTEM SHALL emitir cada event type **uma vez por ocorrência** (`+1`, como o critério por evento da A1). Como a idempotência dedupe a reentrega por `(sessionId, playerId, event)`, cada `(sessão, jogador, evento)` distinto contribui uma vez.
- OBSERVAÇÃO (consequência da chave, decisão do roadmap): a idempotência por `(sessionId, playerId, event)` faz a contagem ser **uma por tipo de evento por sessão**. "Ajudou N bosses como co-op" = **N sessões** com `BOSS_DEFEATED` (não N bosses numa mesma sessão). É a simplificação aceita da H2 (o produtor não carrega um `eventId` único por ocorrência); refino futuro se necessário.

### Isolamento

- THE SYSTEM SHALL manter o caminho `win_loss_draw` (`ApplyMatchResult`, idempotência por `matchId`) **inalterado**. As duas idempotências são tabelas distintas.

## Contratos de Interface

### Consumer (`cmd/consumer`)

- O `makeHandler` passa a decodificar `{ kind }` primeiro e despachar. `win_loss_draw`/ausente-legado com `matchId` → `ApplyMatchResult`; `participation_event` → decodifica os campos de participação e chama `ApplyParticipationEvent`; `kind` desconhecido → log + `return dup=false, nil` (descarta).

### Service / domínio

- **Novo** `Service.ApplyParticipationEvent(ctx, gameID, sessionID, playerID, role, event, outcome string) (dup bool, err error)`.
- **Nova** função de domínio `ParticipationEventTypes(role, event, outcome string) []string` (mapa acima) + constantes dos event types (`EventInvasionWon`/`EventCoopBossHelped`/`EventWorldJoined`).
- **Novo** método no repo: `ApplyParticipationEvent(ctx, gameID, sessionID, playerID, event string, eventTypes []string, at time.Time) (dup bool, err error)` — marca `processed_participations` + aplica progresso por event type na mesma tx. Reusa `findByEventType`/`recordProgress`.

### Erros

- Sem erros novos de domínio. `kind` desconhecido não é erro (é descarte). Falha de banco propaga (à DLQ, como hoje).

## Modelo de Dados

- **Nova migration** `006_processed_participations.sql`:

```sql
CREATE TABLE IF NOT EXISTS processed_participations (
    session_id   TEXT NOT NULL,
    player_id    TEXT NOT NULL,
    event        TEXT NOT NULL,
    game_id      TEXT NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (session_id, player_id, event)
);
```

Idempotência por `(session_id, player_id, event)` (a chave da H2), na mesma tx do progresso. `player_achievements` (progresso) **inalterado** — o critério por `event_type` já existe (A1).

## Observabilidade

- `participation_events_consumed_total{event}` (counter) — eventos de participação processados por tipo. Ou reusa o contador de mensagens do consumer com um label de `kind`. (Detalhe: plan.)
- Reusa o dashboard/RED do achievements consumer.

## Fora de escopo

- **Critério por stat** para participação (ex.: "cause N mortes como invasor") — só `event_type` nesta fatia; o produtor não manda stats no `participation_event`.
- **`eventId` único por ocorrência** no produtor (contar N bosses na mesma sessão) — refino futuro; a chave `(sessão, jogador, evento)` é o combinado da H2.
- **Analytics** por participação — `analytics/004` (sub-fatia irmã).
- **Catálogo semente** com conquistas de invasão/co-op — o operador cadastra pelo console (as conquistas com `event_type` `invasion_won`/`coop_boss_helped` já funcionam por serem data-driven); um seed de exemplo pode virar cauda.

## Critérios de Aceite

- [ ] Envelope `kind=win_loss_draw` → caminho atual intacto (idempotência por `matchId`).
- [ ] Envelope `kind=participation_event` → traduz por papel e aplica progresso por `event_type`, idempotente por `(sessionId, playerId, event)`.
- [ ] `HOST_KILLED`+`invader`+`SUCCESS` → `invasion_won`; `BOSS_DEFEATED`+`coop`+`SUCCESS` → `coop_boss_helped`; `JOINED` → `world_joined`; `LEFT`/`DIED` → nenhum event type (marca processado, no-op).
- [ ] Reprocessar o mesmo `(sessionId, playerId, event)` → `dup=true`, sem duplicar progresso.
- [ ] `kind` desconhecido/ausente-sem-matchId → descartado (no-op, não vai à DLQ, não polui `processed_matches`).
- [ ] `ParticipationEventTypes` testada isolada (tabela); testes de repo com `-race`; ≥80% em `service/`/`domain/`.

# Spec: 004-participation-analytics

**Serviço:** analytics-service
**Status:** Aprovada (2026-07-07)
**Data:** 2026-07-07
**ADR:** 0023 (perfil de multiplayer por jogo) — **arquétipo `host_world_dropin`, fatia H2** (sink: analytics por participação)
**Roadmap:** `docs/game-models/host_world_dropin-roadmap.md` → H2
**Depende de:** `session/013` (envelope `participation_event`) e o rollup do `queue_arena` (specs 001–003, reusa o padrão de idempotência + upsert).

## Contexto

O analytics consome hoje **um** `kind` do tópico `match-results`: o `win_loss_draw`. O consumer parseia `{matchId, gameId, results[]}`, submete ao worker pool e o repo faz `ApplyMatch` — idempotente por `matchId` (`processed_matches`), agregando `game_rollup` (matches/participações/wins/losses/draws/score) e `game_daily`.

A H2 (session/013) passou a publicar `participation_event` no **mesmo** tópico. Como o SNS faz fan-out para todas as filas, a fila do analytics **já recebe** esses envelopes — e o parse atual, que não olha o `kind`, monta um `MatchResult` vazio (`matchId=""`) e o agrega como uma "partida" fantasma. Esta fatia conserta isso e **agrega participações por papel/evento por jogo** — o material do funil de invasão/co-op no console (H3).

> **Escopo:** só o analytics (sink). Espelha a `achievements/009` (o outro sink): dispatch por `kind` + agregação idempotente por `(sessionId, playerId, event)`. O surfacing no console é H3.

## Comportamento

### Dispatch por `kind` (retrocompat)

- WHEN chega uma mensagem → THE SYSTEM SHALL ler o `kind` e despachar: `win_loss_draw`/ausente-com-`matchId` → o caminho atual (`ApplyMatch`, inalterado); `participation_event` → o novo caminho (agregação de participação); qualquer outro → **ignorar** (não deleta como erro nem trava; descarta silenciosamente). Mensagem malformada segue indo à DLQ (comportamento atual).

### Agregar uma participação

- WHEN chega um `participation_event` `{gameId, sessionId, playerId, role, event, outcome}` → THE SYSTEM SHALL agregar **+1** na contagem de `(gameId, role, event)` de forma **idempotente** por `(sessionId, playerId, event)`: reprocessar (entrega at-least-once do SQS) é **no-op**.
- THE SYSTEM SHALL registrar a marca de idempotência e o incremento do rollup na **mesma transação** (como o `ApplyMatch` faz com `processed_matches`).
- THE SYSTEM SHALL agregar **todos** os eventos do vocabulário (JOINED/BOSS_DEFEATED/HOST_KILLED/DIED/LEFT), por `(role, event)` — o analytics é descritivo (conta tudo), diferente do achievements (que só reage aos eventos com conquista). O `outcome` pode entrar como dimensão adicional se necessário; nesta fatia agrega por `(role, event)` (o outcome fica fora para conter a cardinalidade — decisão do plan).

### Ler as participações de um jogo

- WHEN o painel pede o detalhe de um jogo (`GameAnalytics`) → THE SYSTEM SHALL incluir, **de forma aditiva**, a quebra de participações por papel/evento (`[]ParticipationStat{Role, Event, Count}`). Jogo sem participações → lista vazia. O `queue_arena` (sem host_world) tem a lista vazia — nada muda no JSON existente além do campo novo.

### Isolamento

- THE SYSTEM SHALL manter o caminho `win_loss_draw` (`ApplyMatch`, rollup por `matchId`) **inalterado**. As duas idempotências (por `matchId` / por `(sessionId, playerId, event)`) são tabelas distintas; os rollups são tabelas distintas.

## Contratos de Interface

### Consumer + Ingestor

- O consumer passa a **despachar por `kind`** antes de submeter. O `Ingestor` ganha um segundo caminho: `SubmitParticipation(ctx, ParticipationEvent) <-chan error` (ou o `job` do pool vira polimórfico — decisão do plan). O worker chama `repo.ApplyParticipation`.
- **Novo** `domain.ParticipationEvent{GameID, SessionID, PlayerID, Role, Event, Outcome string}`.

### Repositório

- **Novo** `MatchRepository.ApplyParticipation(ctx, evt ParticipationEvent) (dup bool, err error)` — tx idempotente por `(sessionId, playerId, event)` (tabela `processed_participations`) + upsert `game_participation_rollup (game_id, role, event, count)`.
- **`GameAnalytics`** ganha `Participations []ParticipationStat` (novo tipo `ParticipationStat{Role, Event string; Count int64}`), lido do rollup. `Overview` **inalterado** nesta fatia (participações no detalhe do jogo, não no cross-game — evita cardinalidade no overview).

### Erros

- Sem erros novos. `kind` desconhecido → descarte (não é erro). Falha de banco propaga (mensagem volta à fila).

## Modelo de Dados

- **Migration `003_participation_rollup.sql`**:

```sql
CREATE TABLE IF NOT EXISTS processed_participations (
    session_id   TEXT NOT NULL,
    player_id    TEXT NOT NULL,
    event        TEXT NOT NULL,
    game_id      TEXT NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (session_id, player_id, event)
);

CREATE TABLE IF NOT EXISTS game_participation_rollup (
    game_id TEXT   NOT NULL,
    role    TEXT   NOT NULL,
    event   TEXT   NOT NULL,
    count   BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (game_id, role, event)
);
```

Idempotência + rollup na mesma tx (o padrão do `ApplyMatch`). Rollups existentes (`game_rollup`/`game_daily`) **inalterados**.

## Observabilidade

- Reusa as métricas RED do consumer/pool. Opcional `participations_ingested_total{event}` — decisão do plan; pode ficar para H3 (funil no dashboard).

## Fora de escopo

- **Surfacing no console** (aba/coluna de participações por papel) — H3 (admin-bff/admin-ui consomem o `Participations` novo).
- **Participações no `Overview`** cross-game — só no detalhe do jogo nesta fatia.
- **Dimensão `outcome`** na agregação — agrega por `(role, event)`; refino se o funil pedir.
- **Achievements** por participação — `achievements/009` (sink irmão, já entregue).

## Critérios de Aceite

- [ ] Envelope `kind=win_loss_draw` → `ApplyMatch` intacto (rollup por `matchId`).
- [ ] Envelope `kind=participation_event` → `+1` em `(gameId, role, event)`, idempotente por `(sessionId, playerId, event)`; reprocessar → `dup`, sem dobrar.
- [ ] `kind` desconhecido → descartado (não trava a fila, não vira partida fantasma).
- [ ] `GameAnalytics` devolve `Participations` (por papel/evento) de forma aditiva; jogo sem participação → lista vazia; `queue_arena` sem mudança de comportamento.
- [ ] Testes com `-race`; agregação testada (integração de repo, gated); ≥80% em `service/`/`domain/`.

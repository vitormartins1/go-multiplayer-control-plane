# Spec: 005-battle-royale-placement

**Serviço:** analytics-service
**Status:** Done
**Data:** 2026-07-07
**ADR:** 0023 (perfil de multiplayer por jogo) — arquétipo `battle_royale`
**Roadmap:** `docs/game-models/battle_royale-roadmap.md` — fatia **BR3** (sinks), parte analytics
**Depende de:** BR2 (`session/015` — envelope `kind=placement`) · reusa o dispatch por `kind` do consumer, o worker pool (fan-in por channel) e a idempotência por `matchID` (`processed_matches`) das specs `003`/`004`

## Contexto

BR2 publica `kind=placement`. Hoje o consumer de analytics despacha `win_loss_draw` e `participation_event`; um `placement` cai no `default` e é **descartado** (deletado, não é nosso). Esta fatia adiciona o ramo `placement`: agrega a **distribuição de colocações** por jogo (histograma 1º / top-10 / resto) e a **sobrevivência média**, de forma idempotente por `matchID`.

Espelha o caminho de participação (spec `004`): novo método no `Ingestor` (`SubmitPlacement`), novo parse no consumer, novo rollup no repo (tabela nova + migration), campo aditivo no read model do jogo. **Fronteira:** só agregação; sem validar a corrida.

## Comportamento

### Consumir o envelope `placement`

- WHEN o consumer recebe `kind=placement` → THE SYSTEM SHALL desserializar `{ matchId, gameId, entries[]{ placement, survivedMs } }`, submeter ao worker pool (`SubmitPlacement`) e deletar a mensagem só no sucesso (mesmo fan-in dos outros kinds).
- THE SYSTEM SHALL agregar idempotente por `matchID` (reusa `processed_matches`): reprocessar a mesma partida é no-op (`dup=true`).
- IF envelope malformado → não deleta → retry/DLQ. THE SYSTEM SHALL manter `win_loss_draw`/`participation_event` **sem regressão**; kind desconhecido segue descartado.

### Agregação: distribuição de colocação + sobrevivência

Por partida `placement`, o rollup do jogo (`game_placement_rollup`, uma linha por `game_id`) soma:

| Campo | Regra |
|---|---|
| `winners` | +1 por entrada com `placement == 1` |
| `top_n` | +1 por entrada com `2 <= placement <= TopN` |
| `rest` | +1 por entrada com `placement > TopN` |
| `entries` | +1 por entrada (denominador) |
| `sum_survived_ms` | += `survivedMs` da entrada |

- **Partição (histograma):** `winners + top_n + rest == entries`. Diferente do `top_n` **inclusivo** de achievements/leaderboard (que conta o 1º lugar) — aqui os baldes são **mutuamente exclusivos** (o 1º lugar vai só para `winners`), porque o objetivo é a distribuição. Isso é intencional e documentado.
- **Limiar Top-N:** constante do sink `battle_royale_top_n_threshold = 10` (consistente com os outros sinks).
- `matchId` já é a chave global de idempotência (uma partida = um `ReportResult`); reusa `processed_matches` numa transação (INSERT idempotente → se novo, atualiza o rollup; se conflito, `dup`).

### Read model (aditivo)

- WHEN o painel lê o detalhe do jogo (`GameAnalytics`) → THE SYSTEM SHALL incluir, aditivo, a distribuição de colocação (`winners`/`top_n`/`rest`) e a **sobrevivência média** (`sum_survived_ms / entries`, derivada no read; 0 se `entries == 0`). Vazio/zerado em jogos sem `battle_royale` (sem regressão no queue_arena/host_world). O **surfacing no console** (render) é **BR4**; aqui só expomos no JSON.

## Contratos de Interface

### Consumer (`internal/consumer/consumer.go`)

- `handleBatch` ganha `case kind == "placement":` → `parsePlacement(body)` → `pool.SubmitPlacement(ctx, pr)`.
- `parsePlacement` desserializa `{ matchId, gameId, entries[]{ placement, survivedMs } }` → `domain.PlacementResult`.

### Domínio (`domain/`)

- `PlacementResult{ GameID, MatchID string; Entries []PlacementEntry }`, `PlacementEntry{ Placement int; SurvivedMs int64 }`.
- `PlacementDistribution{ Winners, TopN, Rest, Entries int64; AvgSurvivedMs float64 }` (read model).
- `Ingestor` ganha `SubmitPlacement(ctx, PlacementResult) <-chan error`; `MatchRepository` ganha `ApplyPlacement(ctx, PlacementResult) (dup, error)`; `GameAnalytics` ganha o campo `Placement *PlacementDistribution` (nil/zerado fora do BR).
- const `BattleRoyaleTopNThreshold = 10`.

### Persistência (migration nova)

- `migrations/004_placement_rollup.sql`: `game_placement_rollup(game_id PK, winners, top_n, rest, entries, sum_survived_ms BIGINT default 0)`. Reusa `processed_matches` para idempotência (sem tabela de dedupe nova).

### Pool (`service/pool.go`)

- Novo caminho `SubmitPlacement` no worker pool, pelo mesmo mecanismo de fan-in (channel de retorno) do `Submit`/`SubmitParticipation`.

## Casos de Teste Obrigatórios

- [ ] (consumer) `kind=placement` → `parsePlacement` correto → `SubmitPlacement`; `win_loss_draw`/`participation_event` sem regressão; malformado → DLQ; kind desconhecido → descarta
- [ ] (repo, integração testcontainers) `ApplyPlacement` soma `winners`/`top_n`/`rest`/`entries`/`sum_survived_ms` corretamente; partição fecha (`winners+top_n+rest == entries`); idempotente por `matchID` (2ª aplicação = `dup`, rollup inalterado)
- [ ] (repo) `GameAnalytics` devolve a distribuição + `AvgSurvivedMs` derivada; jogo sem placement → `Placement` nil/zerado (sem regressão)
- [ ] (pool) `SubmitPlacement` devolve erro no channel em falha do repo (mensagem volta à fila)

## Fora do escopo

- **Surfacing no console** (render do histograma/sobrevivência) — BR4.
- **Distribuição temporal** (colocação por dia) e **unique players** do BR — follow-up; aqui só o rollup agregado por jogo.
- **Achievements/leaderboard** de colocação — fatias irmãs.

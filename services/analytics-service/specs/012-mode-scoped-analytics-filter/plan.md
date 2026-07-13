# Plan — 012 filtro de modo no Analytics

## Decisões técnicas

1. **`modeID string` como parâmetro posicional novo em `GameAnalytics`**,
   não um struct de opções — a assinatura já tem 2 parâmetros primitivos
   (`gameID`, `days`); um terceiro mantém o padrão do resto do repositório
   sem introduzir um novo tipo só pra isso.
2. **`modeID == ""` = sem filtro (comportamento atual)** — não é um valor
   mágico novo, é literalmente ausência de filtro, mesmo espírito do já
   usado em `ParticipationEvent.ModeID`/`ProfileSnapshot` pro "modo
   default"/"sem modo".
3. **Só as 5 séries que já têm `mode_id` na tabela ganham o filtro**
   (`game_placement_daily`, `game_mission_daily`, `game_participation_daily`,
   `game_player_stats`, `game_participation_rollup`) — as 3 tabelas
   cumulativas sem a coluna (`game_rollup`, `game_placement_rollup`,
   `game_mission_rollup`) ficam de fora por não terem a dimensão (item D,
   spec separada).
4. **`ModeID` ecoado no `GameAnalytics`** (não um bool `Filtered`) — o valor
   já diz tudo (vazio = sem filtro, não-vazio = qual filtro), sem precisar
   de um segundo campo.
5. **Filtro aplicado com `AND ($N = '' OR mode_id = $N)`** (mesmo padrão já
   usado no schema pra "bucket vazio") — evita duas queries distintas
   (com/sem filtro) ou SQL dinâmico por concatenação.
6. **`MatchResult.ModeID` (achado adicional, trazido pro escopo)** — mesmo
   tratamento dos "primos" (`PlacementResult`/`CollectiveResult`/
   `ParticipationEvent`, spec 010): campo novo, extraído no parser, sem
   migration de backfill pro dado legado (fica `''`, mesmo bucket já aceito
   no resto do projeto).

## Arquivos por camada

**analytics-service:**
- `internal/domain/analytics.go`:
  - `MatchRepository.GameAnalytics` ganha o parâmetro `modeID string`.
  - `GameAnalytics` struct ganha `ModeID string`.
- `internal/repository/postgres_analytics.go`:
  - `GameAnalytics(ctx, gameID, days, modeID)`: seta `out.ModeID = modeID`;
    repassa `modeID` pras 3 chamadas de `dailySeries` (ganha parâmetro
    novo), pra query de `RoleParticipations` e pra query de `Stats`.
  - `dailySeries(ctx, table, valueCol, gameID, days, modeID string)`: WHERE
    ganha `AND ($4 = '' OR mode_id = $4)`, `GROUP BY day` inalterado (o
    filtro não muda a forma de agregação por dia, só reduz o universo).
  - Query de `RoleParticipations`: `WHERE game_id = $1 AND ($3 = '' OR
    mode_id = $3)`.
  - Query de `Stats`: `WHERE game_id = $1 AND ($3 = '' OR mode_id = $3)
    GROUP BY stat_key` (inalterado no agrupamento — só reduz o universo
    quando filtrado).
- `internal/consumer/consumer.go`:
  - `domain.MatchResult` ganha `ModeID string` (achado adicional).
  - `parse()` (kind=win_loss_draw): struct anônima do envelope ganha
    `ModeID string \`json:"modeId"\``, propagado pro `domain.MatchResult`
    retornado — mesmo padrão de `parsePlacement`/`parseCollective`/
    `parseParticipation`.
- `internal/repository/postgres_analytics.go` (achado adicional):
  - `ApplyMatch`: `insertPlayerStats(ctx, tx, m.GameID, m.ModeID,
    m.MatchID, ...)` — troca o `""` hardcoded por `m.ModeID`.
- Confirmado: não há camada de `service` entre `transport` e
  `repository` pro caminho de leitura (`http_handler` chama
  `MatchRepository` direto via a Port `Reader`); `internal/service/pool.go`
  só cuida do caminho de escrita (consumo do fan-out), sem mudança aqui
  além do teste (`pool_test.go`, ver Testes).
- `internal/transport/http_handler.go`:
  - Handler do endpoint `/admin/games/{gameId}/analytics` lê
    `r.URL.Query().Get("modeId")`, repassa pra `GameAnalytics`.
  - DTO de resposta ganha `ModeID string \`json:"modeId,omitempty"\`` (eco).
- Testes: `postgres_analytics_test.go` (ou integration) — filtro reduz
  corretamente as 5 séries, campos cumulativos permanecem globais;
  `http_handler_test.go` — query param lido e ecoado.

**admin-bff:** spec-irmã `026-mode-scoped-analytics-passthrough` —
`Client.GameAnalytics`/`Service.GameAnalytics` ganham `modeID string`,
handler HTTP lê `modeId` da query e repassa; `GameAnalyticsDetail` ganha
`ModeID`.

## Ordem de implementação

Domínio (assinatura + `ModeID`) → repository (5 queries) → transport
(query param + DTO) → testes → build/test completo → validação ao vivo
(jogo com 2+ modos reais, comparar resposta com/sem `modeId`) → admin-bff
(passthrough) → validação ao vivo do admin-bff.

# Spec: 010-mode-scoped-time-series

**Serviço:** analytics-service
**Status:** Done (2026-07-13) — validado ao vivo (grpcurl: battle_royale,
coop_mission e host_world_dropin, jogos neon-breach e Achievements Test)
**Data:** 2026-07-13
**Parte de:** "Analytics Fase 2/3" (docs/NEXT-STEPS.md, item 6).

## Contexto

Só `queue_arena` tem série temporal ("partidas por dia", `game_daily`,
renderizada no admin-ui desde a spec `admin-ui/054`). `battle_royale`
(`game_placement_rollup`), `coop_mission` (`game_mission_rollup`) e
`host_world_dropin` (`game_participation_rollup`) são só contadores
cumulativos, sem dimensão de tempo — o gráfico "X por dia" desses 3
arquétipos não existe.

Além disso, `PlacementResult` e `CollectiveResult` (domínio do
analytics-service) não carregam `mode_id` — o envelope do fan-out já publica
`modeId` pra esses dois kinds (`placementEnvelope`/`collectiveEnvelope` em
`session-service/internal/publisher/sns_publisher.go`) desde o ADR-0024, mas o
`analytics-consumer` descarta o campo na desserialização (`parsePlacement`/
`parseCollective`). É a MESMA lacuna que a spec 009 fechou pra
`participation_event` — aqui fechamos criando o schema novo já com `mode_id`
desde o início (não repete o erro).

## Comportamento

- WHEN uma partida `battle_royale` fecha (`kind=placement`) -> THE SYSTEM
  SHALL, além do rollup cumulativo existente (`game_placement_rollup`,
  inalterado), incrementar um contador diário por `(game_id, mode_id, day)`.
- WHEN uma missão `coop_mission` fecha (`kind=collective`) -> THE SYSTEM SHALL,
  além do rollup cumulativo existente (`game_mission_rollup`, inalterado),
  incrementar um contador diário por `(game_id, mode_id, day)`.
- WHEN um evento de participação `host_world_dropin` é reportado
  (`kind=participation_event`) -> THE SYSTEM SHALL, além do rollup cumulativo
  existente (`game_participation_rollup`, inalterado), incrementar um contador
  diário de EVENTOS por `(game_id, mode_id, day)` (não há "partida" terminal
  em host_world_dropin — a atividade é medida em eventos de participação).
- `game_daily` (queue_arena) e a leitura/escrita do `game_rollup`/
  `game_placement_rollup`/`game_mission_rollup` cumulativos NÃO mudam nesta
  spec — só os 3 arquétipos que não tinham série temporal ganham uma.
- `GET /admin/games/{gameId}/analytics` SHALL devolver três séries novas,
  aditivas, cada uma vazia/ausente fora do arquétipo correspondente (mesmo
  padrão de `Placement`/`Mission` hoje): `placementDaily`, `missionDaily`,
  `participationDaily` — cada uma uma lista de `{day, count}` para os últimos
  `days` (mesmo parâmetro `?days=` já existente).
- Idempotência: os contadores diários entram NA MESMA transação do rollup
  cumulativo já idempotente (`processed_matches`/`processed_participations`)
  — reprocessar uma mensagem (SQS at-least-once) não infla o dia duas vezes.

## Contratos de Interface

### Domínio (analytics-service)
```go
type PlacementResult struct {
    MatchID string
    GameID  string
    ModeID  string // NOVO — hoje descartado no parse
    Entries []PlacementEntry
}
type CollectiveResult struct {
    MatchID string
    GameID  string
    ModeID  string // NOVO — hoje descartado no parse
    Outcome string
}
// GameAnalytics ganha:
PlacementDaily     []DailyPoint // battle_royale; vazio fora do BR
MissionDaily       []DailyPoint // coop_mission; vazio fora do coop
ParticipationDaily []DailyPoint // host_world_dropin; vazio fora do HWD
```

### Schema (migration 008)
```sql
CREATE TABLE game_placement_daily (
    game_id TEXT NOT NULL, mode_id TEXT NOT NULL DEFAULT '', day DATE NOT NULL,
    matches BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (game_id, mode_id, day)
);
CREATE TABLE game_mission_daily (
    game_id TEXT NOT NULL, mode_id TEXT NOT NULL DEFAULT '', day DATE NOT NULL,
    matches BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (game_id, mode_id, day)
);
CREATE TABLE game_participation_daily (
    game_id TEXT NOT NULL, mode_id TEXT NOT NULL DEFAULT '', day DATE NOT NULL,
    events BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (game_id, mode_id, day)
);
```

### HTTP admin (`GET /admin/games/{gameId}/analytics`, aditivo)
```json
{
  "...campos existentes inalterados...": "...",
  "placementDaily": [{"day": "2026-07-13", "matches": 3}],
  "missionDaily": [{"day": "2026-07-13", "matches": 1}],
  "participationDaily": [{"day": "2026-07-13", "matches": 42}]
}
```
`participationDaily[].matches` é, na verdade, a contagem de EVENTOS de
participação daquele dia (reaproveita o mesmo shape `dailyDTO` das outras
séries — host_world_dropin não tem "partida" terminal, ver plan.md).

## Critérios de Aceite

1. Partida `battle_royale` fechada -> linha em `game_placement_daily` pro dia
   UTC atual; `game_placement_rollup` (cumulativo) segue funcionando igual.
2. Missão `coop_mission` fechada -> linha em `game_mission_daily`.
3. Evento de participação `host_world_dropin` -> linha em
   `game_participation_daily` (contagem de EVENTOS, não de sessões).
4. Reprocessar a mesma mensagem (SQS) não duplica o contador diário
   (idempotência herdada da mesma tx do rollup cumulativo).
5. `mode_id` vazio (dado legado, antes do ADR-0024) cai no bucket `''` —
   mesmo padrão já usado em `game_participation_rollup`.
6. `GET .../analytics` devolve `placementDaily`/`missionDaily`/
   `participationDaily` vazios (não erro) pra jogos sem esse arquétipo.
7. `queue_arena` (`game_daily`, `game_rollup`) inalterado — nenhuma regressão.

## Fora de escopo (v1)

- Filtro por modo nas séries diárias (a query soma todos os modos do jogo,
  mesmo padrão do resto do endpoint hoje — só `roleParticipations` tem
  seletor de modo na UI). Pode virar follow-up se um jogo real pedir.
- Adicionar `mode_id` aos rollups CUMULATIVOS (`game_rollup`,
  `game_placement_rollup`, `game_mission_rollup`) — não pedido nesta spec,
  tabelas estáveis, sem dor hoje; só as tabelas NOVAS (diárias) nascem com
  `mode_id`.

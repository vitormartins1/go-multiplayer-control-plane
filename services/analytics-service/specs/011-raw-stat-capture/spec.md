# Spec: 011-raw-stat-capture

**Serviço:** analytics-service
**Status:** Done (2026-07-13) — validado ao vivo nos 4 kinds (win_loss_draw,
placement, collective, participation_event via grpcurl, jogos neon-breach e
Achievements Test); soma/contagem/média agregadas conferidas.
**Data:** 2026-07-13
**Parte de:** "Analytics Fase 2/3" (docs/NEXT-STEPS.md, item 6).

## Contexto

Os três envelopes terminais do fan-out (`win_loss_draw`, `placement`,
`collective`) já carregam `stats map[string]int64` por jogador desde a
origem (`domain.PlayerOutcome`/`PlacementEntry`/`CollectivePlayer` no
session-service — usado por achievements/leaderboard). O analytics-service
desserializa esses envelopes hoje mas **descarta o campo `stats`** — só usa
pra contadores agregados (wins/losses, colocação, sucesso/falha). Não existe
NENHUMA tabela de stats brutos por jogador; a aba Analytics não tem como
mostrar "kills totais" ou "média de assists por partida".

`participation_event` (host_world_dropin) **depende de
`session-service/026-participation-event-stats`** (achado ao redigir esta
spec: o envelope desse kind não carregava `stats` — gap real do
session-service, distinto do `achievements-service/018`, que só implementou o
lado de RECEBER; nunca chegava dado). Esta spec assume a 026 já implementada
— os 4 kinds (win_loss_draw/placement/collective/participation_event) ficam
no mesmo pé.

## Comportamento

- WHEN uma partida `win_loss_draw` (queue_arena) fecha com `stats` não-vazio
  em algum jogador -> THE SYSTEM SHALL persistir uma linha por
  (jogador, stat_key) com o valor bruto.
- WHEN uma partida `placement` (battle_royale) fecha com `stats` não-vazio em
  alguma entrada -> THE SYSTEM SHALL persistir do mesmo jeito (o `playerId` da
  entrada, hoje descartado no parse, passa a ser lido).
- WHEN uma missão `collective` (coop_mission) fecha com `stats` não-vazio em
  algum jogador -> THE SYSTEM SHALL persistir do mesmo jeito (a lista de
  `players[]` do envelope, hoje descartada por completo — `CollectiveResult`
  só tem `Outcome` — passa a ser lida).
- WHEN um evento `participation_event` (host_world_dropin) chega com `stats`
  não-vazio -> THE SYSTEM SHALL persistir do mesmo jeito, por
  (jogador, stat_key), usando `match_id = session_id` (host_world não tem
  matchId — mesma convenção já usada no resto do session-service pra esse
  arquétipo).
- IF `stats` vier vazio/ausente (jogo que não declara `scoringStats`) -> THE
  SYSTEM SHALL seguir funcionando sem gravar nada (aditivo, sem regressão).
- Idempotência: os INSERTs de stats entram NA MESMA transação do rollup já
  idempotente por `matchID` (`processed_matches`) — reprocessar não duplica.
- `GET /admin/games/{gameId}/analytics` SHALL devolver um resumo agregado por
  `stat_key` (soma, contagem de ocorrências, média) — não a lista bruta por
  jogador (isso é papel do leaderboard-service, que já ranqueia por stat; o
  analytics só mostra o panorama agregado do jogo).

## Contratos de Interface

### Domínio (analytics-service)
```go
type PlayerResult struct {
    PlayerID string
    Outcome  string
    Won      bool
    Score    int64
    Stats    map[string]int64 // NOVO — hoje descartado no parse
}
type PlacementEntry struct {
    PlayerID   string           // NOVO — hoje descartado (analytics só agregava o histograma)
    Placement  int
    SurvivedMs int64
    Stats      map[string]int64 // NOVO
}
type CollectiveResult struct {
    MatchID string
    GameID  string
    ModeID  string
    Outcome string
    Players []CollectivePlayerStat // NOVO — hoje CollectiveResult não tem lista de jogadores
}
type CollectivePlayerStat struct {
    PlayerID string
    Stats    map[string]int64
}
type ParticipationEvent struct {
    // ...campos existentes (GameID/SessionID/PlayerID/Role/Event/Outcome/EventID/ModeID)...
    Stats map[string]int64 // NOVO — depende de session-service/026
}

// GameAnalytics ganha:
Stats []StatSummary // agregado por stat_key; vazio se o jogo não declara stats
type StatSummary struct {
    Key   string
    Sum   int64
    Count int64   // ocorrências (jogador×partida) com esse stat
    Avg   float64 // Sum/Count
}
```

### Schema (migration 009)
```sql
CREATE TABLE game_player_stats (
    id          BIGSERIAL PRIMARY KEY,
    game_id     TEXT NOT NULL,
    mode_id     TEXT NOT NULL DEFAULT '',
    match_id    TEXT NOT NULL,
    player_id   TEXT NOT NULL,
    stat_key    TEXT NOT NULL,
    stat_value  BIGINT NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_game_player_stats_summary ON game_player_stats (game_id, stat_key);
```
Sem UNIQUE/idempotência própria: a proteção contra reentrega vive na mesma tx
do `processed_matches` (win_loss_draw/placement/collective) ou
`processed_participations` (participation_event) — se o evento já foi
processado, a tx nem chega nos INSERTs de stats.

### HTTP admin (aditivo)
```json
{ "stats": [{"key": "kills", "sum": 1234, "count": 300, "avg": 4.11}] }
```

## Critérios de Aceite

1. Partida queue_arena com `stats: {"kills": 7}` reportado -> linha em
   `game_player_stats`; `GET .../analytics` mostra `kills` no resumo agregado.
2. Idem pra `placement` (battle_royale) e `collective` (coop_mission).
3. Jogo sem `scoringStats` no perfil (nunca reporta `stats`) -> `stats: []`,
   sem erro, sem regressão nos outros campos.
4. Reprocessar a mesma mensagem SQS não duplica as linhas de stats.
5. Evento `participation_event` (host_world_dropin) com `stats` -> linha em
   `game_player_stats` com `match_id = session_id`; sem `stats` -> segue
   funcionando sem gravar nada.

## Fora de escopo (v1)

- Read por jogador ("stats de fulano nesta partida") — cabe ao
  leaderboard-service (boards por stat já existem, ADR-0023). O analytics só
  agrega o panorama do JOGO.
- Retenção/purga de `game_player_stats` (cresce ~1 linha por
  jogador×partida×stat — sem sinal de volume real ainda para decidir política).

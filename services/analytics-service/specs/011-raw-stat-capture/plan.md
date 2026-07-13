# Plan — 011 captura de stats brutos

## Decisões técnicas

1. **Tabela única `game_player_stats`**, greenfield, uma linha por
   (jogador, partida/sessão, stat_key). Sem UNIQUE/idempotência própria — a
   proteção contra reentrega vive na MESMA tx do rollup já idempotente
   (`processed_matches` pros 3 kinds terminais; `processed_participations`
   pro participation_event).
2. **`match_id` = `session_id` em host_world_dropin** — esse arquétipo não
   tem matchId; mesma convenção já usada no resto do session-service.
3. **`domain.PlayerResult`/`PlacementEntry` ganham `Stats`** (já existiam no
   envelope, descartados no parse — mesmo padrão de mode_id na 010).
   `CollectiveResult` ganha `Players []CollectivePlayerStat` (hoje só tem
   `Outcome`, sem lista de jogadores nenhuma). `ParticipationEvent` ganha
   `Stats` (destravado por `session-service/026`).
4. **Leitura agregada, não por jogador**: `GameAnalytics` ganha
   `Stats []StatSummary{Key, Sum, Count, Avg}`, um `GROUP BY stat_key` sobre
   `game_player_stats` do jogo. Não expõe o valor por jogador — isso é papel
   do leaderboard-service (já ranqueia por stat, ADR-0023).
5. **Sem stat -> sem gravação** (jogo sem `scoringStats` no perfil nunca
   manda `stats`; o INSERT é condicional a `len(stats) > 0`, evitando linhas
   vazias/ruído).

## Arquivos por camada

**analytics-service:**
- `migrations/009_player_stats.sql` (novo) — `game_player_stats` + índice
  `(game_id, stat_key)`.
- `internal/domain/analytics.go`:
  - `PlayerResult.Stats map[string]int64` (novo).
  - `PlacementEntry.{PlayerID, Stats}` (novo — hoje só Placement/SurvivedMs).
  - `CollectiveResult.Players []CollectivePlayerStat` (novo tipo).
  - `ParticipationEvent.Stats map[string]int64` (novo).
  - `GameAnalytics.Stats []StatSummary` (novo); `StatSummary{Key, Sum, Count, Avg}`.
- `internal/consumer/consumer.go`:
  - `parse` (win_loss_draw): lê `playerId`+`stats` de cada entrada de `results`.
  - `parsePlacement`: lê `playerId`+`stats` de cada `entries`.
  - `parseCollective`: lê `players[]` inteiro (hoje ignorado por completo).
  - `parseParticipation`: lê `stats`.
- `internal/repository/postgres_analytics.go`:
  - `ApplyMatch`/`ApplyPlacement`/`ApplyCollective`/`ApplyParticipation`:
    depois do rollup+daily, um loop de INSERT em `game_player_stats` por
    (jogador, stat) não-vazio, na MESMA tx.
  - `GameAnalytics`: query nova `SELECT stat_key, SUM(stat_value),
    COUNT(*), AVG(stat_value) FROM game_player_stats WHERE game_id=$1 GROUP
    BY stat_key ORDER BY stat_key`.
- `internal/transport/http_handler.go` — `gameAnalyticsDTO.Stats
  []statSummaryDTO`.
- Testes: consumer (stats em cada parse), repository (INSERT condicional,
  agregação, idempotência herdada), http handler (DTO).

**admin-bff / admin-ui:** spec-irmãs 024 / 062 (não tocados aqui).

## Ordem de implementação

Migration → domain → consumer (parse) → repository (Apply* + leitura) →
http_handler (DTO) → testes → build/test completo → validação ao vivo
(reportar stats nos 4 kinds — já dá pra reusar as sessões abertas na
validação da 010 + a stats-smoke-coop da 026 — e conferir o resumo agregado).

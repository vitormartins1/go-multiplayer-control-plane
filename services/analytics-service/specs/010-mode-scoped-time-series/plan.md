# Plan — 010 série temporal por arquétipo

## Decisões técnicas

1. **3 tabelas novas, uma por arquétipo sem série hoje** — `game_daily`
   (queue_arena) fica intocado; `game_placement_daily`/`game_mission_daily`/
   `game_participation_daily` nascem já com `mode_id` na PK (mesmo padrão da
   migration 007), evitando repetir a lacuna que a spec 009 teve que corrigir
   depois.
2. **`ModeID` novo em `PlacementResult`/`CollectiveResult`** — o envelope já
   publica `modeId` pra esses dois kinds (`sns_publisher.go`); só o
   analytics-service descartava. `ParticipationEvent.ModeID` já existe
   (spec 009) — reaproveitado.
3. **Contador diário = 1 por evento agregado**, não uma métrica composta:
   `game_placement_daily`/`game_mission_daily` incrementam `matches` (1 por
   `ReportResult`, como `game_daily`); `game_participation_daily` incrementa
   `events` (1 por `ReportParticipationEvent` — não há "partida" terminal em
   host_world_dropin, a atividade é medida em eventos).
4. **Idempotência herdada**: os UPSERTs diários entram na MESMA tx do rollup
   cumulativo já idempotente (`processed_matches`/`processed_participations`)
   — sem chave própria.
5. **Sem mudança nos rollups cumulativos** (`game_rollup`,
   `game_placement_rollup`, `game_mission_rollup`) — ficam `(game_id)` como
   estão; só as tabelas novas (diárias) ganham `mode_id`. Fora de escopo
   documentado na spec.
6. **Leitura**: `GameAnalytics(gameID, days)` soma todos os modos (mesmo
   padrão de `game_daily` hoje — sem seletor de modo nas séries diárias
   nesta v1; a spec já documenta isso como fora de escopo).

## Arquivos por camada

**analytics-service:**
- `migrations/008_archetype_daily.sql` (novo) — as 3 tabelas.
- `internal/domain/analytics.go`:
  - `PlacementResult.ModeID string` (novo).
  - `CollectiveResult.ModeID string` (novo).
  - `GameAnalytics` ganha `PlacementDaily`, `MissionDaily`,
    `ParticipationDaily []DailyPoint`.
- `internal/consumer/consumer.go`:
  - `parsePlacement`: struct anônimo ganha `ModeID string \`json:"modeId"\``.
  - `parseCollective`: idem.
- `internal/repository/postgres_analytics.go`:
  - `ApplyPlacement`: upsert extra em `game_placement_daily` (dentro da
    mesma tx, depois do rollup cumulativo).
  - `ApplyCollective`: upsert extra em `game_mission_daily`.
  - `ApplyParticipation`: upsert extra em `game_participation_daily`
    (incrementa `events`, não `matches`).
  - `GameAnalytics`: 3 queries novas (mesmo padrão da leitura de
    `game_daily`), somando por dia entre modos.
- `internal/transport/http_handler.go` — `gameAnalyticsDTO` ganha
  `placementDaily`, `missionDaily`, `participationDaily []dailyDTO`
  (reaproveita o tipo `dailyDTO` já existente).
- Testes: `consumer_test.go` (modeId em placement/collective),
  `postgres_analytics_test.go` (upsert diário + leitura, idempotência),
  `http_handler_test.go` (DTO expõe os 3 campos).

**admin-bff / admin-ui:** ficam pra spec-irmã 024 / 062 (não tocados aqui).

## Ordem de implementação

Migration → domain → consumer (parse) → repository (Apply* + leitura) →
http_handler (DTO) → testes → build/test completo → validação ao vivo
(grpcurl: fechar 1 partida de cada arquétipo restante, conferir a série
diária no endpoint).

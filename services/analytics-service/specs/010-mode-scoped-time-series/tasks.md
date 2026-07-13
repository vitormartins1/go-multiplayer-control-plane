# Tasks — 010 série temporal por arquétipo

## analytics-service

- [x] 1. Migration `008_archetype_daily.sql`: `game_placement_daily`,
      `game_mission_daily`, `game_participation_daily` (schema do plan).
- [x] 2. `domain/analytics.go`: `PlacementResult.ModeID`,
      `CollectiveResult.ModeID`, `GameAnalytics.{PlacementDaily,MissionDaily,
      ParticipationDaily}`.
- [x] 3. `consumer.go`: `parsePlacement`/`parseCollective` extraem `modeId`.
- [x] 4. `postgres_analytics.go`: `ApplyPlacement`/`ApplyCollective`/
      `ApplyParticipation` gravam a linha diária correspondente; leitura de
      `GameAnalytics` inclui as 3 séries novas.
- [x] 5. `http_handler.go`: `gameAnalyticsDTO` expõe `placementDaily`/
      `missionDaily`/`participationDaily`.
- [x] 6. Testes: consumer (modeId em placement/collective), repository
      (upsert diário + idempotência + leitura), http handler (DTO). Rodar
      `go test ./services/analytics-service/... -race`.
- [x] 7. Build completo do monorepo + suíte inteira.
- [x] 8. Validação ao vivo: rebuild do container, fechar 1 partida
      battle_royale, 1 coop_mission e reportar 1 evento host_world_dropin
      (grpcurl), conferir as 3 séries diárias no `GET .../analytics`.

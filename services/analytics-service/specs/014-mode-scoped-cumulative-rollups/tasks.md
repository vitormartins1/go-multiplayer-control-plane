# Tasks — 014 mode_id nos rollups cumulativos

- [x] 1. `migrations/011_mode_scoped_cumulative_rollups.sql`: 5
      `ALTER TABLE` (game_rollup, game_daily, game_placement_rollup,
      game_mission_rollup, game_player_seen)
- [x] 2. `internal/repository/postgres_analytics.go` — writers:
  - [x] 2.1. `ApplyMatch`: `game_rollup`/`game_daily`/`game_player_seen`
        gravam `m.ModeID`, `ON CONFLICT` atualizado
  - [x] 2.2. `ApplyPlacement`: `game_placement_rollup` grava `pr.ModeID`
  - [x] 2.3. `ApplyCollective`: `game_mission_rollup` grava `cr.ModeID`
- [x] 3. `internal/repository/postgres_analytics.go` — readers:
  - [x] 3.1. `GameAnalytics`: `game_rollup` vira agregação com filtro
  - [x] 3.2. `GameAnalytics`: `game_daily` passa a usar `dailySeries`
  - [x] 3.3. `GameAnalytics`: `game_placement_rollup`/
        `game_mission_rollup` viram agregação com filtro + `COUNT(*)` pra
        distinguir "sem dado" de "zero"
  - [x] 3.4. `GameAnalytics`: subquery de `game_player_seen` ganha
        `DISTINCT` + filtro por modo
  - [x] 3.5. `Overview`: `GROUP BY r.game_id` + `SUM(...)`; subquery de
        `game_player_seen` ganha `DISTINCT`
- [x] 4. Testes (integration):
  - [x] 4.1. Sem `?modeId=` -> regressão zero (31 testes existentes
        continuam passando)
  - [x] 4.2. `TestGameAnalytics_ModeID_IsolaRollupCumulativo`: 2 modos
        queue_arena, filtro isola matches/wins/losses/daily/uniquePlayers
  - [x] 4.3. `TestGameAnalytics_ModeID_IsolaPlacementEMission`: 2 modos
        battle_royale + 2 modos coop_mission, filtro isola
  - [x] 4.4. `TestOverview_JogoComDoisModos_NaoDuplica`: achado crítico
        confirmado em teste — sem o GROUP BY, duplicaria
  - [x] 4.5. `TestGameAnalytics_ModeID_JogadorEmDoisModos_UniquePlayersNaoDuplica`:
        jogador em 2 modos conta 1 no global, 1 em cada modo
- [x] 5. `go build ./...` + `go test ./... -race -cover` (39 unit + 35
      integration, todos passando)
- [x] 6. Validação ao vivo: Packet Arena real (dado acumulado de todo o
      item A/B/C desta sessão, 24 matches legados em `mode_id=''` +
      partidas novas pós-deploy) — sem filtro soma 26; `?modeId=default`
      isola exatamente a partida nova daquele modo (1 match, 2 jogadores);
      `?modeId=teams-2v2` idem; `Overview` continua 1 linha só pro jogo
      (26 matches, 56 jogadores únicos, sem duplicar)
- [x] 7. admin-ui (spec-irmã 066): remove código morto (próxima fatia)

# Tasks: 021-battle-royale-console

**Serviço:** admin-ui · **Spec:** [spec.md](./spec.md)

- [x] **AU1** — `src/lib/api.ts`: `BattleRoyaleParams`/`RespawnMode`, `battle_royale` na union `MultiplayerProfile`; `PlacementDistribution` + `placement?` em `GameAnalyticsDetail`.
- [x] **AU2** — `MultiplayerSettings.tsx`: `BR_DEFAULTS`, `buildBattleRoyaleProfile`, `BattleRoyaleForm` (lobbySize/teamSize/respawnMode/scoringStats + tooltips); archetype state/Select/load/save + validação (lobbySize ≥ 2×teamSize, stats).
- [x] **AU3** — `GameAnalytics.tsx`: bloco condicional "Distribuição de colocação" (winners/topN/rest/entries + sobrevivência média via formatDuration).
- [x] **AU4** — `Rooms.tsx`: cache de perfis por gameId + badge "lobby N/lobbySize · modo" nas salas de jogos battle_royale.
- [x] **AU5** — `npm run build` (tsc --noEmit && vite build) verde; sem regressão em queue_arena/host_world_dropin.

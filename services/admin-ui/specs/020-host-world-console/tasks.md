# Tasks: 020-host-world-console

Fatia H3 (UI) — Rooms por papel + participações no analytics do jogo. Ver `spec.md`.

- [x] **T1 — tipos + api.** `lib/api.ts`: `GameSession` += origin/hostPlayerId/roleSlots; `RoleSlot`, `ParticipationStat`, `GameAnalyticsDetail`; `api.gameAnalytics(gameId)`.
- [x] **T2 — Rooms.** `pages/Rooms.tsx`: componente `RoleSlots` (badges co-op/invasor occupied/capacity); coluna Players condicional por origin; host + slots no detalhe expandido.
- [x] **T3 — GameAnalytics.** `pages/GameAnalytics.tsx`: usa `api.gameAnalytics`; tabela de `roleParticipations` (rótulos pt-BR); some no queue_arena.
- [x] **T4 — verde.** `tsc --noEmit` + `npm run build`.

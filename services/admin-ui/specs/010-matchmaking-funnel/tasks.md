# Tasks: 010-matchmaking-funnel

**Plan:** specs/010-matchmaking-funnel/plan.md
**Data:** 2026-07-04

- [x] `src/lib/api.ts` — tipo `LiveMetrics` + `api.liveMetrics()`
- [x] `src/pages/Overview.tsx` — extrai `StatCard`; bloco **Matchmaking** (4 cards); FanoutCards passa a usar `StatCard`
- [x] `src/pages/Overview.tsx` — `useAsync(api.liveMetrics)` + polling ~10s (Atualizar + intervalo)
- [x] `npm run build` (typecheck limpo)
- [ ] verificação visual dos cards — pendente (stack de pé)

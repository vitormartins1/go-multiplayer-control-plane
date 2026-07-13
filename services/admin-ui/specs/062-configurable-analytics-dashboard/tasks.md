# Tasks — 062 dashboard configurável de Analytics

## admin-ui

- [x] 1. `npm install react-grid-layout` (+ import do CSS
      `react-grid-layout/css/styles.css` e `react-resizable/css/styles.css`).
- [x] 2. `api.ts`: `GameAnalyticsDetail.{placementDaily,missionDaily,
      participationDaily}: DailyPoint[]`, `stats: StatSummary[]`.
- [x] 3. `src/lib/dashboardLayout.ts`: tipos + `loadLayout`/`saveLayout`/
      `defaultLayout` (localStorage por `gameId`).
- [x] 4. `src/components/DashboardGrid.tsx`: wrapper de react-grid-layout
      (modo leitura vs. edição, cabeçalho por widget com esconder).
- [x] 5. `GameAnalytics.tsx`: monta o catálogo de widgets (existentes +
      `placement-daily`/`mission-daily`/`participation-daily`/`raw-stats`
      novos); integra `DashboardGrid` + `useDashboardLayout`; toggle "Editar
      layout" + "Restaurar padrão".
- [x] 6. `npm run build` limpo (typecheck + vite build).
- [x] 7. Rebuild do container `admin-ui`.
- [x] 8. Verificação em browser real (Playwright ad-hoc): aba renderiza com
      layout padrão; modo de edição permite arrastar/redimensionar/esconder;
      recarregar preserva; "Restaurar padrão" reverte; console sem erros
      novos. Screenshots.

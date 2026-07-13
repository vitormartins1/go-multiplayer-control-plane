# Tasks — 065 widget de saúde das sessões

- [x] 1. `lib/api.ts`: `GameAnalyticsDetail.sessionHealth`, tipo
      `SessionCloseStat`
- [x] 2. `pages/GameAnalytics.tsx`:
  - [x] 2.1. `CLOSE_REASON_LABELS` + `CLOSE_REASON_COLORS`
  - [x] 2.2. Widget `session-health`: donut + tabela (Motivo/Contagem/
        Duração média), coluna "Modo" condicional a >1 modo distinto nos
        dados recebidos
  - [x] 2.3. Sem ícone fixo, sem legenda de aviso (cross-arquétipo,
        já filtrável de verdade)
  - [x] 2.4. Filtro do catálogo por arquétipo (spec 064) ganha
        `session-health` como sempre-elegível, junto de `raw-stats`
- [x] 3. `npx tsc --noEmit` + `npm run build` — sem erros
- [x] 4. Validação ao vivo (Playwright ad-hoc): Achievements Test, 2
      motivos de fechamento reais (`host_killed`/`host_left`) -> donut de
      2 fatias + tabela corretas; widget continua visível ao filtrar por
      `mundo-vivo`; zero erros de console

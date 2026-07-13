# Tasks — 066 remoção do aviso obsoleto

- [x] 1. `pages/GameAnalytics.tsx`:
  - [x] 1.1. Remove `UNSCOPED_WIDGETS`, `UnscopedNotice`,
        `showsUnscopedNotice`, `archetypeModeCount`
  - [x] 1.2. Remove as 5 chamadas `<UnscopedNotice .../>` (summary,
        queue-daily, outcomes-donut, placement-distribution,
        mission-distribution); simplifica `queue-daily`/`outcomes-donut`
        de volta pra `render` sem `<Stack>` desnecessário (só um filho)
  - [x] 1.3. Atualiza comentários que citavam "item D pendente"
- [x] 2. `npx tsc --noEmit` + `npm run build` — sem erros, sem referências
      soltas
- [x] 3. Validação ao vivo (Playwright ad-hoc): Packet Arena filtrado por
      `default` -> "Resumo"/"Partidas por dia"/"Vitórias × derrotas ×
      empates" mostram os números JÁ filtrados (1 partida, 1 vitória, 1
      derrota), sem nenhum aviso; zero erros de console

# Tasks — 067 layout estático e estruturado do Analytics

- [x] 1. `pages/GameAnalytics.tsx`:
  - [x] 1.1. Nova estrutura `CardDef`/`Section` (tipos locais)
  - [x] 1.2. Reagrupa o `useMemo` do catálogo em `sections: Section[]`
        (mesmo gating por dado, mesma filtragem por modo/arquétipo já
        existente)
  - [x] 1.3. Substitui a renderização final: `Stack` de seções, cada uma
        com cabeçalho (ícone + label, se houver arquétipo) + `Grid`
        (12 colunas) dos cards com `span` fixo
  - [x] 1.4. Remove o import/uso de `DashboardGrid`
- [x] 2. Remove `components/DashboardGrid.tsx`
- [x] 3. Remove `lib/dashboardLayout.ts`
- [x] 4. Remove `react-grid-layout` de `package.json` (+ `npm install`,
      5 pacotes removidos)
- [x] 5. `npx tsc --noEmit` + `npm run build` — sem erros, sem
      referências soltas; bundle do GameDetail encolheu de 189KB pra
      134KB
- [x] 6. Validação ao vivo (Playwright ad-hoc):
  - [x] 6.1. Achievements Test (2 arquétipos, mas só host_world_dropin
        tinha dado real): seção "Host World Drop-in" com cabeçalho +
        ícone, cards certos, seção final (raw-stats/session-health)
        pareada
  - [x] 6.2. Mesmo jogo em viewport mobile (390px) -> cards empilhados
        (`base: 12`), sem quebra visual
  - [x] 6.3. Sem "Editar layout"/"Restaurar padrão" — confirmado ausente
  - [x] 6.4. Packet Arena (achado bônus: tem dado real dos 3 arquétipos
        queue_arena/battle_royale/coop_mission) -> 3 seções corretas,
        cada uma com o card certo; filtro por modo "default" -> só a
        seção "Queue Arena" aparece, números corretos (1 partida, 1
        vitória, 1 derrota)
  - [x] 6.5. Zero erros de console em todos os passos

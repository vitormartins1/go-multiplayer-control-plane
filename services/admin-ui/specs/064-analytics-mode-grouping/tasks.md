# Tasks — 064 agrupamento por arquétipo/modo no Analytics

- [x] 1. `lib/api.ts`:
  - [x] 1.1. `gameAnalytics` ganha `opts?: { modeId?: string }`
        (URLSearchParams no call site, mesmo padrão de `gameLeaderboard`)
  - [x] 1.2. `GameAnalyticsDetail` ganha `modeId?: string`
- [x] 2. `components/DashboardGrid.tsx`:
  - [x] 2.1. `WidgetDef` ganha `icon?: Icon`
  - [x] 2.2. Render do cabeçalho do card mostra o ícone (se houver) ao lado
        do título
- [x] 3. `pages/GameAnalytics.tsx`:
  - [x] 3.1. `WIDGET_ARCHETYPE` (tabela estática widget id -> arquétipo)
  - [x] 3.2. `UNSCOPED_WIDGETS` (Set dos 5 widgets cumulativos não
        filtráveis — incluindo `queue-daily`, achado durante a validação:
        faltava o aviso nesse widget, corrigido)
  - [x] 3.3. `selectedMode` entra nas deps do `useAsync`; fetch passa
        `modeId` quando != ALL_MODES
  - [x] 3.4. Seletor "Modo" acima do `DashboardGrid`, só se `modes.length
        > 1` (sentinela NO_MODE do widget antigo removido — backend não
        distingue "filtro vazio" de "sem filtro")
  - [x] 3.5. Cada `push` no array de widgets ganha `icon` via
        `WIDGET_ARCHETYPE`
  - [x] 3.6. Filtragem final do array de widgets por arquétipo quando
        `selectedMode !== ALL_MODES` (+ `raw-stats` sempre elegível)
  - [x] 3.7. Legenda de aviso (`showsUnscopedNotice`) nos 5 widgets de
        `UNSCOPED_WIDGETS` quando filtro ativo e o jogo tem >1 modo daquele
        arquétipo
  - [x] 3.8. `participations-role`: `Select` interno + filtragem
        client-side removidos; usa `data.roleParticipations` direto; coluna
        "Modo" só quando >1 modo distinto nos dados recebidos
- [x] 4. `npx tsc --noEmit` — sem erros
- [x] 5. Build do admin-ui (`npm run build`) — sem erros
- [x] 6. Validação ao vivo (Playwright ad-hoc):
  - [x] 6.1. Achievements Test (2 arquétipos): ícones corretos nos cards
        sem filtro; seletor de modo aparece; escolher `mundo-vivo` esconde
        os widgets de `queue_arena` (confirmado via screenshot)
  - [x] 6.2. Packet Arena (2 modos `queue_arena`, `orbs` em ambos):
        escolher `teams-2v2` -> `raw-stats` isola `152` (mesmo número do
        item A); `summary`/`queue-daily`/`outcomes-donut` mostram a
        legenda de aviso (achado durante a validação: `queue-daily`
        faltava o aviso, corrigido antes de fechar)
  - [x] 6.3. Sandbox (0 modos) -> seletor não aparece
  - [x] 6.4. Packet Arena: filtra `default` -> volta pra "Todos os modos"
        -> catálogo completo restaurado (Resumo reaparece)
  - [x] 6.5. Layout persistido por `gameId` (localStorage), independente
        do modo — troca de modo só muda `visibleItems` (filtro existente
        de `DashboardGrid` já ignora ids fora do catálogo atual sem apagar
        posição salva); nenhuma mudança na lógica de persistência
  - [x] 6.6. Zero erros de console em todos os passos acima

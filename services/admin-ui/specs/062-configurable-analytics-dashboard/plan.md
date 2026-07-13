# Plan — 062 dashboard configurável de Analytics

## Decisões técnicas

1. **Biblioteca:** `react-grid-layout` (justificativa no spec.md). Usa o
   `WidthProvider(GridLayout)` (padrão da lib pra grid responsivo sem breakpoints
   múltiplos — a aba Analytics não precisa de breakpoints por device, é um
   painel admin desktop-first).
2. **Catálogo de widgets** — um array de definições (`id`, `title`,
   `defaultLayout: {x,y,w,h}`, `available(data): boolean`, `render(ctx):
   ReactNode`) em vez do JSX condicional atual. `GameAnalytics.tsx` monta o
   catálogo com o `data` já carregado e passa pro componente de grid.
3. **Estado do layout**: `{ items: {i, x, y, w, h}[], hidden: string[] }`,
   uma entrada de `localStorage` por jogo (`analytics-dashboard:{gameId}`).
   Migração de schema não é necessária (v1). Merge com o catálogo disponível
   no load: widgets do catálogo ausentes no layout salvo (ex.: usuário nunca
   viu esse widget, ou era indisponível antes) entram no fim com o
   `defaultLayout`; entradas do layout salvo cujo widget não existe mais no
   catálogo (não deveria acontecer, mas defensivo) são ignoradas.
4. **Modo de edição**: toggle local (`useState`, não persistido) que alterna
   `isDraggable`/`isResizable` do `GridLayout` e mostra o botão "Restaurar
   padrão". Fora do modo de edição, a grid é só leitura (sem handles visíveis
   — `isDraggable={false}`).
5. **Reaproveita os widgets existentes tal como são** — cada `FormSection`
   com gráfico vira o `render()` de um widget; NENHUMA lógica de gráfico é
   reescrita, só a casca de layout muda.

## Arquivos por camada

**admin-ui:**
- `package.json` — `react-grid-layout` (+ `@types/react-grid-layout` se não
  vier tipado; a lib é TS-first na v2, então provavelmente não precisa).
- `src/lib/dashboardLayout.ts` (novo) — tipos `WidgetLayoutItem`/
  `DashboardLayoutState`; `loadLayout(gameId, catalogIds)`,
  `saveLayout(gameId, state)`, `defaultLayout(catalog)` (localStorage,
  chave `analytics-dashboard:{gameId}`).
- `src/components/DashboardGrid.tsx` (novo) — wrapper de
  `react-grid-layout`: recebe `widgets: WidgetDef[]`, `layout state`,
  `editing: boolean`, `onLayoutChange`; renderiza cada widget disponível
  numa célula com um cabeçalho (título + olho pra esconder, se `editing`).
- `src/pages/GameAnalytics.tsx` — refatorado: monta o catálogo (`summary`,
  `queue-daily`, `outcomes-donut`, `participations-role`,
  `placement-distribution`, `placement-daily`, `mission-distribution`,
  `mission-daily`, `participation-daily`, `raw-stats`) a partir de `data`;
  cada entrada filtra por `available(data)` (mesmas condições que hoje são
  `if (data.placement) ...`); usa `DashboardGrid` + `useDashboardLayout(gameId,
  catalogIds)`; toggle "Editar layout" + botão "Restaurar padrão" no cabeçalho.
- Widget novo `raw-stats`: tabela/StatCards com `data.stats` (key/sum/count/avg).
- Widgets novos `placement-daily`/`mission-daily`/`participation-daily`:
  `LineChart`, mesmo visual do `queue-daily` existente.
- `src/lib/api.ts` — `GameAnalyticsDetail` ganha `placementDaily`/
  `missionDaily`/`participationDaily: DailyPoint[]` e `stats: StatSummary[]`.

## Ordem de implementação

`api.ts` (tipos) → `dashboardLayout.ts` (persistência) → `DashboardGrid.tsx`
(casca) → `GameAnalytics.tsx` (catálogo + integração) → `npm run build` →
rebuild do container → verificação em browser real (Playwright ad-hoc, mesmo
padrão da spec 025): abrir a aba, arrastar/redimensionar um widget, recarregar
a página (layout persiste), esconder um widget, restaurar padrão.

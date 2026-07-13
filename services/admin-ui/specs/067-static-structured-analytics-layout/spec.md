# Spec: 067-static-structured-analytics-layout

**Serviço:** admin-ui
**Status:** Done
**Data:** 2026-07-13
**Parte de:** feedback direto do usuário após uso real da página de
Analytics (pós evolução A→B→C→D) — reverte a spec 062 (dashboard
configurável) por um layout fixo, responsivo e bem estruturado.

## Contexto

O usuário usou a página de Analytics de verdade (não só validação técnica)
e achou o modelo de dashboard editável (`DashboardGrid`, spec 062:
drag/resize/hide de widgets, botões "Editar layout"/"Restaurar padrão",
persistido em localStorage) confuso. Pedido explícito: layout DESENHADO
por nós — fixo, responsivo, bem organizado — em vez de delegado ao usuário
editar. "Dinâmico" no pedido do usuário significa "reage aos dados
disponíveis" (widget só aparece se há dado — isso já existia e continua),
não "editável".

`DashboardGrid`/`dashboardLayout.ts`/a dependência `react-grid-layout` são
usados SÓ por `GameAnalytics.tsx` (confirmado por busca no repo) — remover
é seguro, sem efeito colateral em outras telas.

## Comportamento

- A página SHALL organizar os cards em SEÇÕES por arquétipo (ordem fixa,
  mesma do `ARCHETYPE_META`: `queue_arena` → `host_world_dropin` →
  `battle_royale` → `coop_mission`), cada seção com cabeçalho (ícone +
  nome do arquétipo, reaproveitando `ARCHETYPE_META`). Seção SHALL só
  aparecer se tiver pelo menos um card com dado (mesmo gating por
  presença de dado já existente, só reorganizado).
- Dentro de cada seção, os cards SHALL seguir um layout responsivo fixo
  (Mantine `Grid`, 12 colunas): o card de "resumo/distribuição" pareado
  lado a lado com o card "por dia" (`span 6` cada em telas médias/grandes,
  `span 12`/empilhado em mobile) — mesmo par visual nas 3 seções que têm
  esse padrão (queue_arena, battle_royale, coop_mission). A seção
  `host_world_dropin` (participações por papel + eventos por dia) segue o
  mesmo par.
- Uma seção final, SEM cabeçalho de arquétipo ("Geral" ou sem título),
  SHALL agrupar os 2 widgets cross-arquétipo (`raw-stats`/
  `session-health`) lado a lado (`span 6` cada), quando presentes.
- `DashboardGrid`/`dashboardLayout.ts`/a dependência `react-grid-layout`
  SHALL ser removidos — sem edição de layout pelo usuário, sem
  persistência de arranjo customizado.
- O seletor de modo (spec 064) e a filtragem de widgets por arquétipo
  selecionado (spec 064/065/066) SHALL continuar exatamente como estão —
  esta spec é só sobre COMO os widgets são posicionados na tela, não sobre
  QUAIS aparecem.

## Contratos de Interface

Sem mudança de API/tipos — é reorganização pura de apresentação em cima do
`GameAnalyticsDetail` já existente. `WidgetDef`/`DashboardGrid` deixam de
existir; o catálogo de widgets vira uma estrutura de SEÇÕES (arquétipo +
lista de cards), montada no mesmo `useMemo` que já filtra por dado/modo.

## Critérios de Aceite

1. Jogo com 2+ arquétipos (Achievements Test) -> 2 seções visíveis, cada
   uma com cabeçalho de arquétipo e os cards certos.
2. Cada seção com o card de resumo/distribuição ao lado do card "por dia"
   em desktop; empilhados em mobile (viewport estreito).
3. Sem botão "Editar layout"/"Restaurar padrão" — layout sempre fixo.
4. Filtro de modo (spec 064) continua funcionando idêntico — escolher um
   modo mostra só a seção daquele arquétipo.
5. `raw-stats`/`session-health` aparecem numa seção final, lado a lado,
   sem cabeçalho de arquétipo.
6. `react-grid-layout` removido de `package.json`; `DashboardGrid.tsx`/
   `dashboardLayout.ts` removidos do repo (sem uso órfão).

## Fora de escopo

- Qualquer nova métrica/widget — só reorganização visual dos que já
  existem.
- Filtro de modo/arquétipo — inalterado (specs 064/065/066).

# Plan — 067 layout estático e estruturado do Analytics

## Decisões técnicas

1. **Seções por arquétipo, ordem fixa igual a `ARCHETYPE_META`**
   (`queue_arena → host_world_dropin → battle_royale → coop_mission`) —
   mesma ordem canônica já usada noutros lugares do app (badges do header
   do GameDetail), sem inventar uma ordem nova.
2. **`Grid` (12 colunas) em vez de `SimpleGrid`/`react-grid-layout`** —
   Mantine `Grid` já dá responsividade por breakpoint com spans
   declarativos (`span={{ base: 12, md: 6 }}`), sem precisar de posição
   x/y calculada (react-grid-layout) nem de reflow manual.
3. **Par fixo "resumo/distribuição" + "por dia" em cada seção que tem os
   dois** — replica o padrão visual que já existia informalmente (os dois
   cards relacionados ficavam próximos mesmo no layout livre); agora é
   estrutural, não um acidente de onde o usuário arrastou.
4. **Estrutura de dados: `Section[]` em vez de `WidgetDef[]`** —
   ```ts
   interface CardDef { id: string; title: string; span: number; render: () => ReactNode }
   interface Section { archetype?: MultiplayerProfile["archetype"]; title?: string; cards: CardDef[] }
   ```
   `archetype` ausente = seção final cross-arquétipo (raw-stats/
   session-health), sem cabeçalho de ícone.
5. **Remoção completa de `DashboardGrid`/`dashboardLayout.ts`/
   `react-grid-layout`** — confirmado via busca que são usados só por
   `GameAnalytics.tsx`; remover é seguro. `localStorage` de layouts
   antigos (`analytics-dashboard-layout:{gameId}`) fica órfão no browser
   dos usuários que já usaram — inofensivo (chave nunca mais lida),
   não precisa de migração/limpeza ativa.

## Arquivos por camada

- `package.json`: remove `react-grid-layout` (dependência da spec 062).
- `components/DashboardGrid.tsx`: removido.
- `lib/dashboardLayout.ts`: removido.
- `pages/GameAnalytics.tsx` (reescrita da montagem final, lógica de
  filtro/gating por dado/modo inalterada):
  - Cada `render()` de card continua igual (o JSX interno dos cards não
    muda — só como são agrupados/posicionados).
  - Cada card ganha um `span` fixo (6 pros pares resumo+diário, 6 pros 2
    cards da seção final, 12 se algum widget não tiver par — ex.:
    `participations-role`, que é maior/mais denso, fica `span 12` sozinho
    acima do `participation-daily`, que aí sim pareia `span 6` ao lado de
    nada — reavaliar durante a implementação se o par não fizer sentido
    visual pro host_world_dropin, dado que `participations-role` é mais
    alto que os outros).
  - Monta `sections: Section[]` a partir do mesmo catálogo condicional já
    existente, agrupando por `WIDGET_ARCHETYPE` (reaproveitado) em vez de
    filtrar uma lista plana.
  - Renderiza cada seção: `<Stack>` com `Group` de cabeçalho (ícone +
    label do arquétipo, se houver) + `Grid` dos cards.
  - Remove o botão "Editar layout"/"Restaurar padrão" (não existe mais
    `DashboardGrid` pra prover isso).

## Ordem de implementação

Reestrutura o `useMemo` (catálogo -> seções) → substitui a renderização
final (`DashboardGrid` -> `Grid`/`Stack` por seção) → remove
`DashboardGrid.tsx`/`dashboardLayout.ts`/dependência → `tsc --noEmit` +
build → validação ao vivo (Playwright: desktop e mobile viewport,
Achievements Test com 2 arquétipos, confirmar responsividade e ausência
dos botões de edição).

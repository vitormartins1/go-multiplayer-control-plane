# Tasks 043 — Editor visual de Event Rules

- [x] T1. `lib/rulesGraph.ts` — tipos dos nós, build fact/regra/type/achievement,
      layout em camadas (baricentro) e `pathHighlight` (BFS bidirecional).
- [x] T2. `components/flow/` — `flow.css` (cards por camada, handles, faded/active),
      `nodes.tsx` (FactNode/RuleNode/EventTypeNode/AchievementNode),
      `FlowCanvas.tsx` (ReactFlow dark + Background/Controls/MiniMap tematizados).
- [x] T3. `pages/GameEventRulesCanvas.tsx` — seletor de arquétipo, paleta com
      drag-and-drop, painel de propriedades (detalhe/criação/remoção via
      endpoints existentes), destaque de caminho na seleção.
- [x] T4. `pages/GameEventRules.tsx` — toggle Canvas|Tabela (default Canvas),
      fetch de catalog, tabela existente preservada.
- [x] T5. Build verde (`npm run build`: tsc --noEmit + vite) e smoke visual
      (Playwright: screenshots do grafo e do destaque de caminho no Neon Breach;
      ajustadas alturas estimadas e `tt="none"` nos badges após o primeiro smoke).

# Plan 043 — Editor visual de Event Rules

## Decisões técnicas

1. **Lib do canvas:** `@xyflow/react` (React Flow 12, já dependência — ADR-0028).
   `colorMode="dark"` tematiza Controls/MiniMap nativamente.
2. **Auto-layout:** **layout próprio em camadas** (sem dagre/elkjs). O grafo é
   estritamente 4-camadas (fact → regra → type → achievement), então um layout
   de colunas fixas em X + empilhamento em Y com ordenação por baricentro
   (média dos índices dos vizinhos da camada anterior, minimiza cruzamentos) é
   ~50 linhas, determinístico e evita dependência nova. Se um grafo futuro do
   kit não for em camadas, aí sim entra dagre/elkjs (decisão adiada com causa).
3. **Kit reutilizável** em `src/components/flow/` (R7): `FlowCanvas.tsx`
   (wrapper ReactFlow + Background/Controls/MiniMap tematizados) + nós custom
   (`FactNode`, `RuleNode`, `EventTypeNode`, `AchievementNode`) + `flow.css`.
   Nada ali importa `lib/api` nem conhece a página — os nós recebem dados via
   `node.data`.
4. **Grafo como função pura** em `src/lib/rulesGraph.ts`: `buildRulesGraph`
   (rules+catalog → nodes/edges com posições) e `pathHighlight` (BFS
   upstream+downstream a partir do nó selecionado → sets de nós/edges ativos).
   Espelha o padrão do avaliador Go (função pura, testável isoladamente).
5. **Página:** `GameEventRulesCanvas.tsx` (nova) = paleta esquerda (Nova regra
   + facts do arquétipo, draggable) | canvas | painel direito de propriedades
   (detalhe do nó selecionado; form de criação com Autocomplete das
   chaves/valores e TagsInput com sugestões — substitui o modal no modo
   canvas). `GameEventRules.tsx` ganha SegmentedControl **Canvas | Tabela**
   (default Canvas) e passa `rules`/`catalog`/`reload` pra baixo — a tabela
   existente permanece intacta como segunda view.
6. **Seleção destaca o caminho** (R2): nós fora do caminho ganham classe
   `is-faded` (opacity), edges do caminho ficam `animated` + stroke destacado.
7. **Achievements (camada 4):** `api.gameCatalog`; entram só os com
   `eventType` presente no grafo (stat-criteria, `statKey`, fica de fora — não
   é disparado por event type).
8. **Criação via drop:** arrastar um fact da paleta pro canvas abre o painel
   de criação com aquele par pré-preenchido (`dataTransfer` custom, padrão do
   exemplo drag-and-drop do React Flow). Sem posicionamento manual — a posição
   vem sempre do layout (posições não persistem, spec).

## Arquivos

| Arquivo | Ação |
|---|---|
| `src/lib/rulesGraph.ts` | novo — build do grafo + layout + highlight (puro) |
| `src/components/flow/FlowCanvas.tsx` | novo — wrapper tematizado |
| `src/components/flow/nodes.tsx` | novo — 4 nós custom |
| `src/components/flow/flow.css` | novo — tema dos cards/handles/edges |
| `src/pages/GameEventRulesCanvas.tsx` | novo — view canvas (paleta+painel) |
| `src/pages/GameEventRules.tsx` | editar — toggle Canvas/Tabela, lift de catalog |

## Riscos / atenção

- `nodeTypes` precisa de referência estável (fora do componente) — senão o
  React Flow re-monta os nós a cada render.
- Alturas dos nós são estimadas no layout (conteúdo variável do RuleNode) —
  estimativa conservadora por número de pares do match.
- Jogo sem modos: mesma degradação da tabela (fallback de arquétipos).

# Plan 048 — Fontes unificadas + criação in-canvas

## Decisões técnicas

1. **Grafo por FONTE**: `rulesGraph` reescrito — camada 1 vira um nó por
   CHAVE (`src:<key>`; kinds structural/mode/stat), condições viram `label`
   das edges fonte→regra (`condShort` novo no matchCond: só op+valor).
   Critério por métrica = edge `src:stat.<k>` → achievement com label
   (`acumulada ≥ N`). Toggle Stats OFF esconde fontes de métrica SEM uso em
   regras + achievements por métrica (fonte usada em regra permanece, senão
   a edge da regra ficaria órfã).
2. **Draft in-canvas via node.data com callbacks**: o build posiciona os nós
   `ruleDraft`/`eventTypeDraft` (altura estimada por nº de linhas); a página
   injeta data (linhas, sugestões, onChange/onSave/onCancel) num map pós-build.
   Inputs dentro do nó com `className="nodrag"` + `dragHandle` do React Flow
   restrito ao header (padrão oficial de custom nodes com form).
3. **Operador como chip + Popover** (7 ops) — cabe na largura do nó; value
   Autocomplete condicional ao op (igual ao drawer que sai).
4. **Event types do draft**: `linkedTypes` (clique num type existente liga/
   desliga) + `newTypes` ({id, value} editáveis inline, criados pelo "+" do
   rodapé/handle). Payload = linked + new não-vazios.
5. **Busca**: TextInput na toolbar; casa contra texto derivado por tipo de nó
   (page-side); casados = is-active, resto is-faded (prioridade sobre a
   seleção enquanto houver termo).
6. **Hint de redundância**: página computa os event types emitidos por regras
   de match VAZIO do arquétipo; painel da regra mostra o hint quando emite um
   deles (não-bloqueante — a união do motor deduplica).
7. **Atalho de achievement**: quick-create local (Drawer à direita) com os
   MESMOS campos/validação do form da aba (id/name/description/target,
   critério fixo "por evento", eventType pré-preenchido e travado) — extrair
   o form inteiro do GameAchievements criaria acoplamento entre páginas por
   pouco ganho; anotado como desvio consciente da spec ("o mesmo form").

## Arquivos

| Arquivo | Ação |
|---|---|
| `lib/matchCond.ts` | `condShort` (label sem a chave) |
| `lib/rulesGraph.ts` | reescrita: sources, labels, metric edges, draft ghosts |
| `components/flow/nodes.tsx` | `SourceNode`, `RuleDraftNode`, `EventTypeDraftNode` |
| `components/flow/flow.css` | fontes, draft em edição, edge labels |
| `pages/GameEventRulesCanvas.tsx` | reescrita: draft in-canvas, busca, painéis, quick-create |

## Riscos / atenção

- Callbacks em node.data re-renderizam os nós a cada mudança do draft — ok
  (grafo pequeno); `nodeTypes` continua module-level.
- AutoFit a cada linha adicionada pode "pular" — aceito (fitKey inclui draft).
- Esc cancela via listener global só enquanto o draft está ativo.

# Plan 055 — canvas focado por regra + lista separada

## Visão geral das mudanças

| Arquivo | Mudança |
|---|---|
| `src/lib/rulesGraph.ts` | Nova função pura `siblingRuleIds` (regras vizinhas via source/eventType compartilhado). `pathHighlight` **não muda** — já serve pro filtro. |
| `src/pages/GameEventRulesCanvas.tsx` | Novas props `focusRuleId`/`onFocusRule`; filtro de nodes/edges baseado no `focusRuleId` (quando `draft === null`); toggle "regras relacionadas" (estado local); estado vazio. |
| `src/pages/GameEventRules.tsx` | Remove o `SegmentedControl` canvas/tabela; layout vira lista + canvas lado a lado (sempre); estado `focusRuleId` sobe pra cá; filtro de texto na lista; clique na linha seleciona. |

Nenhuma mudança em `src/lib/api.ts` nem em qualquer client HTTP — confirmado na spec que o CRUD já é granular por id.

## `rulesGraph.ts` — `siblingRuleIds` (função nova)

Mesmo estilo/vizinhança de `pathHighlight` (linha 357), logo abaixo dela:

```ts
// siblingRuleIds devolve os ids de nós `rule`/`ruleDraft` que têm uma edge
// direta pra qualquer node em focusNodeIds (tipicamente o resultado de
// pathHighlight de uma regra) — usado pelo toggle "regras relacionadas"
// (spec 055): acha quem mais escuta a mesma source/eventType da regra
// focada, sem expandir recursivamente (1 salto só, por design).
export function siblingRuleIds(edges: Edge[], focusNodeIds: Set<string>): Set<string> {
  const siblings = new Set<string>();
  for (const e of edges) {
    if (focusNodeIds.has(e.source) && (e.target.startsWith("r:"))) siblings.add(e.target);
    if (focusNodeIds.has(e.target) && (e.source.startsWith("r:"))) siblings.add(e.source);
  }
  return siblings;
}
```

Uso no canvas: `siblingRuleIds(graph.edges, focusPath.nodeIds)` devolve ids tipo `r:xxx`; pra cada um, `pathHighlight(graph.edges, siblingId)` de novo e unir tudo (nodeIds/edgeIds) no conjunto visível. Não precisa de função extra — o caller (`GameEventRulesCanvas`) faz a união num loop simples.

## `GameEventRulesCanvas.tsx`

### Props novas (linha ~101-114, `GameEventRulesCanvasProps`)

```ts
focusRuleId: string | null;
onFocusRule: (ruleId: string | null) => void;
```

### Estado novo

```ts
const [showRelated, setShowRelated] = useState(false); // toggle "regras relacionadas", default off (spec item 4)
```

### Filtro — dentro do `useMemo` principal (hoje linhas 315-416)

Depois de `graph = buildRulesGraph(...)` (linha ~204-211) e antes da lógica de `path`/`search` existente:

```ts
const focusPath = useMemo(() => {
  if (!focusRuleId || draft) return null; // draft ativo desliga o filtro (spec item 7 — não mexe no fluxo de criação/edição)
  const base = pathHighlight(graph.edges, `r:${focusRuleId}`);
  if (!showRelated) return base;
  const visible = new Set(base.nodeIds);
  const visibleEdges = new Set(base.edgeIds);
  for (const siblingId of siblingRuleIds(graph.edges, base.nodeIds)) {
    const p = pathHighlight(graph.edges, siblingId);
    p.nodeIds.forEach((id) => visible.add(id));
    p.edgeIds.forEach((id) => visibleEdges.add(id));
  }
  return { nodeIds: visible, edgeIds: visibleEdges };
}, [graph, focusRuleId, showRelated, draft]);
```

`path` (o highlight por clique, linha 317 hoje) continua existindo **só quando `focusPath` já filtrou o grafo** — ou seja, clicar num node dentro do subgrafo focado ainda abre o `NodeDetails` (overlay) normalmente, mas não desfaz o filtro (`selectedId` interno já é usado só pro overlay + `path` de destaque *dentro* do subgrafo, nunca pra decidir visibilidade — essa responsabilidade passa a ser 100% do `focusPath`).

### Aplicar o filtro na montagem final de `nodes`/`edges` (linhas 349-413)

- `nodes`: `graph.nodes.filter((n) => !focusPath || focusPath.nodeIds.has(n.id) || (n.type === "sectionLabel" && groupHasVisibleSource(n, focusPath)))`. `groupHasVisibleSource` é um helper pequeno inline: um `sectionLabel` só aparece se ao menos um `source` do mesmo grupo estiver em `focusPath.nodeIds` (evita cabeçalho de seção órfão).
- `edges`: `graph.edges.filter((e) => !focusPath || focusPath.edgeIds.has(e.id))`.
- Quando `focusRuleId === null && !draft`: `focusPath` é `null` mas a spec pede canvas **vazio**, não grafo cheio — então quando `!focusRuleId && !draft`, `nodes`/`edges` finais são `[]` diretamente (não passa pelo filtro de `focusPath`, é um caso à parte, checado antes).
- Quando `draft !== null` (criação ou edição): filtro desligado, comportamento **idêntico ao de hoje** (grafo completo, sem esconder nada) — zero mudança de código nesse ramo.

### Handler de clique (`onNodeClick`, hoje linhas 425-449)

Sem mudança na lógica existente de `draft` (linhas 432-448). Só adiciona, no ramo sem draft (linha ~427-430), depois do `setSelectedId(id)` de sempre:

```ts
const node = graph.nodes.find((n) => n.id === id);
if (node?.type === "rule" || node?.type === "ruleDraft") {
  const rawId = id.slice(2); // "r:xxx" -> "xxx" (mesmo padrão de ruleId/srcId/typeId em rulesGraph.ts)
  onFocusRule(rawId);
}
```

Chamada só no evento de clique (nunca num `useEffect` que observe `focusRuleId`) — evita qualquer ciclo de re-render entre prop e state interno.

### Sincronizar `selectedId` quando `focusRuleId` muda por fora (seleção na lista)

```ts
useEffect(() => {
  setSelectedId(focusRuleId ? `r:${focusRuleId}` : null);
}, [focusRuleId]);
```

### Toggle "regras relacionadas" e estado vazio (JSX, perto da toolbar existente linhas ~500-519)

- `<Switch label="Regras relacionadas" checked={showRelated} onChange={...} disabled={!focusRuleId}/>` ao lado do campo de busca existente.
- Campo de busca (linha ~512-519): sem mudança de comportamento em si — como os `nodes`/`edges` que ele filtra/destaca já vêm reduzidos pelo `focusPath`, a busca automaticamente passa a operar só dentro do subgrafo focado (spec item 6) **sem precisar de código novo** — é consequência direta do filtro já ter cortado a lista de nodes antes da busca rodar.
- Estado vazio: quando `nodes.length === 0` (nem draft nem focus), renderizar no lugar do `<FlowCanvas>` um `<Center h={400}><Text c="dimmed">Selecione uma regra na lista para visualizar</Text></Center>`.

## `GameEventRules.tsx`

### Remove

- `SegmentedControl` (linhas 162-169) e o estado `view` (linha 75) — não existe mais "modo tabela" isolado.
- O bloco condicional `view === "table"` inteiro (linhas 208-278, a `<Table>`) **não é removido**, é **movido** pro novo componente de lista (ver abaixo) — reaproveita o JSX quase 1:1, só ganha `onClick`/estado de seleção.

### Adiciona

```ts
const [focusRuleId, setFocusRuleId] = useState<string | null>(null);
const [listFilter, setListFilter] = useState("");
```

- `filteredRules = visibleRules.filter(matchesListFilter(listFilter))` — `matchesListFilter` é um helper simples (case-insensitive, testa `archetype`, chaves/valores de `match`, `eventTypes.join()`), local a este arquivo (não precisa ir pro `rulesGraph.ts`, que é só sobre o grafo).
- Layout novo (substitui o `Stack` de view única):

```tsx
<Group align="flex-start" gap="md" wrap="nowrap">
  <Box w={380} style={{ flexShrink: 0 }}>
    {/* header com TextInput de filtro + botão "Nova regra" (Modal, inalterado) */}
    {/* <Table> reaproveitada, Table.Tr ganha onClick={() => setFocusRuleId(r.id)} e
        bg destacado quando r.id === focusRuleId */}
  </Box>
  <Box style={{ flex: 1, minWidth: 0 }}>
    <GameEventRulesCanvas
      gameId={gameId}
      rules={visibleRules}
      catalog={catalog ?? []}
      archetypeOptions={archetypeOptions}
      modes={modes ?? []}
      reload={reloadAll}
      focusRuleId={focusRuleId}
      onFocusRule={setFocusRuleId}
    />
  </Box>
</Group>
```

- `rules`/`catalog`/`modes`/`reloadAll` continuam fluindo exatamente como hoje — só os 2 props novos (`focusRuleId`/`onFocusRule`) se somam.
- O Modal de criação (linhas 281-335) e o `openCreate`/`form`/`matchPairs` **não mudam** — ficam disponíveis a partir do header da lista, como hoje ficavam disponíveis só na `view === "table"`.

## Testes / verificação

Sem suíte de testes automatizados no admin-ui (`vitest`/`jest` não configurados neste serviço — verificado). Verificação via:
1. `tsc --noEmit` + `eslint` limpos.
2. Smoke manual com Playwright (`playwright-core`, mesmo padrão da spec 054): abrir o jogo "Achievements Test" (66 achievements/regras), confirmar canvas vazio no load, selecionar uma regra na lista → canvas mostra só o subgrafo, editar match/eventTypes salva normalmente, toggle "relacionadas" traz vizinhas, deselecionar volta ao vazio, criar regra nova (ambos os fluxos) continua funcionando sem regressão.
3. `npm run build` verde (garante que nada quebrou o bundle).

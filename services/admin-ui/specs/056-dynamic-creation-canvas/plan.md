# Plan 056 — criação dinâmica + foco na edição

Único arquivo tocado: `src/pages/GameEventRulesCanvas.tsx`. Nada em `rulesGraph.ts` (reaproveita `repackVisible`/`pathHighlight` como estão) nem em `GameEventRules.tsx`.

## 1. Foco também na edição — `focusPath` passa a considerar `draft.editingRuleID`

Hoje: `if (!focusRuleId || draft) return null;` — qualquer draft (criar OU editar) desliga o filtro.

Novo:

```ts
// effectiveFocusId: durante edição, foca na regra sendo editada (pode ser uma
// "vizinha" trazida por showRelated, não a focusRuleId original — spec 056
// item 6). Durante criação (editingRuleID null) ou sem draft, cai no
// focusRuleId normal.
const effectiveFocusId = draft ? draft.editingRuleID : focusRuleId;

const focusPath = useMemo(() => {
  if (!effectiveFocusId) return null; // criação (editingRuleID null) ou nada focado
  const base = pathHighlight(graph.edges, `r:${effectiveFocusId}`);
  if (!showRelated) return base;
  ... (resto igual, sem mudança)
}, [graph, effectiveFocusId, showRelated]);
```

`draft` sai das deps (não é mais usado direto dentro do corpo — `effectiveFocusId` já absorve essa dependência).

## 2. Filtro de `visibleNodes`/`visibleEdges` — três ramos em vez de dois

Estrutura hoje (`!draft` → foco normal ou vazio; `draft` → grafo inteiro). Novo:

```ts
let visibleNodes = graph.nodes;
let visibleEdges = graph.edges;

if (draft && !draft.editingRuleID) {
  // Criação pura (spec 056): só a regra em rascunho + o(s) ghost(s) de tipo +
  // as sources cujas chaves já foram digitadas/validadas numa linha.
  const referencedKeys = new Set(
    draft.rows.map((r) => r.key.trim()).filter(Boolean),
  );
  visibleNodes = graph.nodes.filter((n) => {
    if (n.id === DRAFT_RULE_ID || n.id.startsWith("t:__new__:")) return true; // sempre visíveis
    if (n.type === "source") {
      const d = n.data as unknown as SourceNodeData;
      return referencedKeys.has(d.sourceKey);
    }
    return false; // event types existentes, achievements, sectionLabel: escondidos
  });
  visibleEdges = graph.edges.filter(
    (e) => visibleNodes.some((n) => n.id === e.source) && visibleNodes.some((n) => n.id === e.target),
  );
} else if (!draft) {
  if (!focusPath) {
    visibleNodes = [];
    visibleEdges = [];
  } else {
    const filtered = graph.nodes.filter((n) => { /* igual à spec 055, usa focusPath */ ... });
    visibleNodes = repackVisible(filtered);
    visibleEdges = graph.edges.filter((e) => focusPath.edgeIds.has(e.id));
  }
} else {
  // draft && draft.editingRuleID (edição): mesmo filtro por foco da spec 055,
  // agora dirigido por effectiveFocusId (= draft.editingRuleID) em vez de
  // focusRuleId puro — reaproveita o MESMO bloco do ramo !draft acima.
  if (focusPath) {
    const filtered = graph.nodes.filter((n) => { /* igual */ ... });
    visibleNodes = repackVisible(filtered);
    visibleEdges = graph.edges.filter((e) => focusPath.edgeIds.has(e.id));
  }
  // sem focusPath (não deveria acontecer em edição — effectiveFocusId sempre
  // setado quando editingRuleID existe) — fallback seguro: grafo inteiro.
}
```

Pra evitar duplicar o bloco de filtro por `focusPath` (usado tanto no "foco normal" quanto na "edição"), extrair um helper local dentro do `useMemo`:

```ts
const applyFocusFilter = (path: NonNullable<typeof focusPath>) => {
  const filtered = graph.nodes.filter((n) => {
    if (path.nodeIds.has(n.id)) return true;
    if (n.type === "sectionLabel" && n.id.startsWith("lbl:")) {
      const kind = n.id.slice(4);
      return graph.nodes.some(
        (s) => s.type === "source" && (s.data as unknown as SourceNodeData).kind === kind && path.nodeIds.has(s.id),
      );
    }
    return false;
  });
  return { nodes: repackVisible(filtered), edges: graph.edges.filter((e) => path.edgeIds.has(e.id)) };
};

if (draft && !draft.editingRuleID) {
  // criação pura — bloco novo do item acima
} else if (focusPath) {
  const r = applyFocusFilter(focusPath);
  visibleNodes = r.nodes;
  visibleEdges = r.edges;
} else if (!draft) {
  visibleNodes = [];
  visibleEdges = [];
}
// draft && draft.editingRuleID && !focusPath: não deveria ocorrer; cai no
// default (graph.nodes/graph.edges completos) como fallback seguro.
```

Repack (`repackVisible`) **não** entra no ramo de criação pura — as sources que vão aparecendo já nascem na posição calculada por `buildRulesGraph` para a coluna inteira; como são poucas (tipicamente 1-4), repacotar traria pouco ganho e a ordem relativa dos poucos itens visíveis já fica razoável sem isso. Se na prática ainda parecer espaçado, é candidato a um follow-up igual ao da spec 055.

## 3. Import de `DRAFT_RULE_ID`

Já exportado por `rulesGraph.ts` (`export const DRAFT_RULE_ID = "r:__draft__"`) mas não importado em `GameEventRulesCanvas.tsx` hoje (o arquivo usa a string literal implicitamente via `formNodeID` do build). Adicionar ao import existente de `rulesGraph`.

## 4. `fitKey` do FlowCanvas

Já reage a `focusRuleId`/`showRelated`/`draft` (spec 055). Adicionar `draft?.rows.length` (ou um hash simples das chaves referenciadas) pra also re-fitar quando uma source nova aparece durante a criação — senão o zoom fica preso no enquadramento de quando só tinha a caixinha de regra.

## Testes / verificação

Mesma abordagem da spec 055: sem suíte automatizada no admin-ui. `tsc --noEmit` + `npm run build`, e smoke manual via Playwright no jogo "Achievements Test":
1. Nova regra → canvas com só 2 caixinhas.
2. Digitar uma chave válida (ex. `event`) → node da source aparece.
3. Editar uma regra existente → canvas mostra só o subgrafo dela (contagem de nós reduzida vs. antes do fix).
4. Editar uma vizinha (via "regras relacionadas") → foco segue a vizinha.

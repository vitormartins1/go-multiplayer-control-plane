# Plan 057 — prévia de regras/fonte no catálogo de achievements

## Componente novo: `RuleFocusMiniCanvas` (local a `GameAchievements.tsx`)

```tsx
// NODE_TYPES local, só os 5 tipos "estáveis" (sem draft/ghost — a prévia
// nunca mostra rascunho).
const MINI_NODE_TYPES: NodeTypes = {
  sectionLabel: SectionLabelNode,
  source: SourceNode,
  rule: RuleNode,
  eventType: EventTypeNode,
  achievement: AchievementNode,
};

function RuleFocusMiniCanvas({
  rules,
  catalog,
  focusNodeId,
}: {
  rules: EventRule[];
  catalog: CatalogAchievement[];
  focusNodeId: string; // `t:${eventType}` ou `a:${achievementId}`
}) {
  const graph = useMemo(() => buildRulesGraph(rules, catalog), [rules, catalog]);
  const { nodes, edges } = useMemo(() => {
    const path = pathHighlight(graph.edges, focusNodeId);
    const filtered = graph.nodes.filter((n) => path.nodeIds.has(n.id));
    const repacked = repackVisible(filtered).map((n) => ({ ...n, selectable: false, draggable: false }));
    return { nodes: repacked, edges: graph.edges.filter((e) => path.edgeIds.has(e.id)) };
  }, [graph, focusNodeId]);

  return (
    <FlowCanvas
      nodes={nodes}
      edges={edges}
      nodeTypes={MINI_NODE_TYPES}
      onNodeClick={() => {}}
      height={280}
      fitKey={focusNodeId}
    />
  );
}
```

Notas:
- `buildRulesGraph(rules, catalog)` recebe `rules` **sem filtrar por arquétipo** (diferente do canvas principal) — o achievement não sabe o arquétipo, e o event type pode em tese vir de qualquer regra que o emita. Sem `stats`/`draft` no `opts` — a prévia não precisa de drift detection nem de rascunho (fora de escopo, spec item 7).
- `selectable: false, draggable: false` forçado em todos os nodes — sem interação nenhuma, mesmo que `buildRulesGraph` marque como selecionável por padrão.
- Sem `overlay` no `FlowCanvas` — não há `selectedId`/clique que abra painel de detalhes.
- Achievement por evento: `focusNodeId = typeId(a.eventType)` (`t:${eventType}`).
- Achievement por métrica: `focusNodeId = achId(a.id)` (`a:${id}`) — `pathHighlight` a partir do achievement anda só pra trás (fonte do stat), forward não tem pra onde ir (achievement é folha).

## `GameAchievements.tsx`

- Novo state: `const [expandedCriteriaId, setExpandedCriteriaId] = useState<string | null>(null);` (mesmo padrão de `expandedId` em `GameSessions.tsx`).
- Novo prop no componente: `onNavigateToRule?: (ruleId: string, opts: { edit: boolean }) => void`.
- A `<Badge>`/`<Group>` da coluna Critério (linhas ~495-520, onde hoje renderiza `statKey`/`eventType` + `DriftBadge`) ganha `onClick={() => setExpandedCriteriaId((cur) => (cur === a.id ? null : a.id))}` e `style={{ cursor: "pointer" }}`.
- Depois da `<Table.Tr>` de cada achievement, uma segunda `<Table.Tr>` condicional (só renderiza a `<Collapse>` por dentro, sempre presente no DOM — mesmo padrão de `GameSessions.tsx`) com `colSpan` cobrindo todas as colunas:

```tsx
<Table.Tr>
  <Table.Td colSpan={7} p={0} style={{ border: expandedCriteriaId === a.id ? undefined : "none" }}>
    <Collapse in={expandedCriteriaId === a.id}>
      <Box p="sm" bg="dark.8">
        {expandedCriteriaId === a.id && (
          <RuleFocusMiniCanvas
            rules={a.statKey ? [] : relevantEventRules}
            catalog={catalog ?? []}
            focusNodeId={a.statKey ? achId(a.id) : typeId(a.eventType)}
          />
        )}
        {!a.statKey && (
          <Group justify="flex-end" mt="xs">
            <Button size="xs" variant="light" onClick={() => navigateToRule(a)}>
              {targetRuleFor(a)?.scope === "game" ? "Editar regra" : "Ver regra"}
            </Button>
          </Group>
        )}
      </Box>
    </Collapse>
  </Table.Td>
</Table.Tr>
```

- Helper `targetRuleFor(a: CatalogAchievement): EventRule | undefined` — `relevantEventRules.filter(r => r.eventTypes.includes(a.eventType))`, prioriza `scope === "game"` (primeiro encontrado), senão primeiro `global` (spec item 6, sem desambiguação futura).
- `navigateToRule(a)`: chama `onNavigateToRule?.(targetRuleFor(a)!.id, { edit: targetRuleFor(a)!.scope === "game" })`.
- Renderizar `RuleFocusMiniCanvas` só quando `expandedCriteriaId === a.id` (não pré-montar as N linhas — `buildRulesGraph` rodando pra cada achievement da lista seria caro à toa; só monta quando expandido, mesmo espírito do `keepMounted={false}` das abas).

## `GameDetail.tsx`

- `Tabs` vira controlado: `const [activeTab, setActiveTab] = useState<string | null>("achievements"); <Tabs value={activeTab} onChange={setActiveTab} keepMounted={false}>`.
- Novo state: `const [pendingRuleFocus, setPendingRuleFocus] = useState<{ ruleId: string; edit: boolean } | null>(null);`
- `<GameAchievements gameId={gameId} onNavigateToRule={(ruleId, opts) => { setPendingRuleFocus({ ruleId, ...opts }); setActiveTab("event-rules"); }} />`
- `<GameEventRules gameId={gameId} pendingRuleFocus={pendingRuleFocus} onConsumePendingRuleFocus={() => setPendingRuleFocus(null)} />`

## `GameEventRules.tsx`

- Novos props: `pendingRuleFocus?: { ruleId: string; edit: boolean } | null`, `onConsumePendingRuleFocus?: () => void`.
- `useEffect(() => { if (!pendingRuleFocus || !rules) return; setFocusRuleId(pendingRuleFocus.ruleId); onConsumePendingRuleFocus?.(); }, [pendingRuleFocus, rules])` — só dispara quando os dados já carregaram (senão focaria um id que ainda não existe na lista).
- Passa `autoEditRuleId={pendingRuleFocus?.edit ? pendingRuleFocus.ruleId : null}` pro `GameEventRulesCanvas` (like ainda não foi consumido nesse render — como o consumo (`onConsumePendingRuleFocus`) só limpa o state do PAI depois, `pendingRuleFocus` continua não-nulo durante o render em que `GameEventRulesCanvas` precisa ler `edit`; a task abaixo detalha como evitar re-disparo).

## `GameEventRulesCanvas.tsx`

- Novo prop: `autoEditRuleId?: string | null`.
- `useEffect(() => { if (!autoEditRuleId) return; const rule = rules.find(r => r.id === autoEditRuleId && r.scope === "game"); if (rule) startEdit(rule); }, [autoEditRuleId, rules])` — como o pai (`GameEventRules`) limpa `pendingRuleFocus` (e портanto `autoEditRuleId` vira `null` no próximo render) logo depois de setar `focusRuleId`, esse efeito só dispara uma vez por navegação (o valor não-nulo dura só 1-2 renders).

## Testes / verificação

Sem suíte automatizada (confirmado nas specs anteriores). `tsc --noEmit` + `npm run build`, smoke manual via Playwright no jogo "Achievements Test":
1. Expandir um achievement por evento → prévia mostra a regra + facts + achievement(s) do mesmo tipo.
2. Expandir um achievement por métrica → prévia só com a fonte do stat + o achievement, sem botão.
3. "Editar regra" (regra do jogo) → aba muda, regra focada, form de edição já aberto.
4. "Ver regra" (regra global) → aba muda, regra focada, sem form de edição.
5. Nenhum node clicável/arrastável na prévia (confirmar via tentativa de clique não abrir overlay).

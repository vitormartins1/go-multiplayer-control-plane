# Plan 058 — canvas ocupando a aba Event Rules (chrome flutuante)

## Ideia estrutural

Hoje `GameEventRules.tsx` faz DUAS coisas: carrega dado (`useAsync` × 3) E desenha layout (texto, `Group` lista+canvas, modal de delete, `toggleSuppress`). `GameEventRulesCanvas.tsx` já tem `deleting`/`toggleSuppress`/`confirmDelete` PRÓPRIOS (usados pelo `NodeDetails`) — duplicados dos que hoje vivem em `GameEventRules.tsx` pra alimentar a lista.

Este plan divide a responsabilidade de novo: `GameEventRules.tsx` vira só um shell de dado (fetch + sync de `pendingRuleFocus`, sem nenhum layout próprio); TODO o chrome (lista, toolbar, texto explicativo) migra pra dentro de `GameEventRulesCanvas.tsx`, como um painel único flutuante (`dock`) sobre o canvas — reaproveitando os `deleting`/`toggleSuppress` que já existem lá (elimina a duplicação, não cria uma nova).

## `components/flow/FlowCanvas.tsx`

Novo slot de painel, análogo ao `overlay` (que fica `top-right`, usado por `GameEventRulesCanvas`/`GamePipeline` pro detalhe do node — não muda):

```tsx
export function FlowCanvas({
  // ...props existentes
  dock, // NOVO
}: {
  // ...
  // dock flutua no canto oposto ao overlay (top-left) — painel de
  // navegação/controles (spec 058), mesmo mecanismo do overlay.
  dock?: ReactNode;
}) {
  return (
    <div className="flow-wrap" ...>
      <ReactFlow ...>
        <Background .../>
        {fitKey !== undefined && <AutoFit fitKey={fitKey} />}
        {dock && <Panel position="top-left">{dock}</Panel>}
        {overlay && <Panel position="top-right">{overlay}</Panel>}
        <Controls .../>
        <MiniMap .../>
      </ReactFlow>
    </div>
  );
}
```

## `lib/eventRules.ts`

Move `matchesListFilter` de dentro de `GameEventRules.tsx` (função local) pra cá, exportada — é usado pelo `dock` dentro do canvas agora, não mais pela página:

```ts
export function matchesListFilter(term: string) {
  const t = term.trim().toLowerCase();
  return (r: EventRule) => {
    if (!t) return true;
    if (r.archetype.toLowerCase().includes(t)) return true;
    if (r.eventTypes.some((et) => et.toLowerCase().includes(t))) return true;
    return Object.entries(r.match).some(
      ([k, v]) => k.toLowerCase().includes(t) || String(v).toLowerCase().includes(t),
    );
  };
}
```

## `components/flow/nodes.tsx` — `RuleDraftData`/`RuleDraftNode`

Seletor de arquétipo (spec item 6) migra pro form de criação in-canvas — só aparece quando o jogo tem mais de um arquétipo E é criação (edição nunca muda arquétipo, spec 016):

```ts
export type RuleDraftData = {
  mode: "create" | "edit";
  rows: MatchRow[];
  keySuggestions: string[];
  valueSuggestions: (row: MatchRow) => string[];
  error?: string | null;
  saving?: boolean;
  onRow: (i: number, patch: Partial<MatchRow>) => void;
  onRemoveRow: (i: number) => void;
  onAddRow: () => void;
  onSave: () => void;
  onCancel: () => void;
  // archetypePicker (spec 058): só presente em criação (mode "create") num
  // jogo com archetypeOptions.length > 1 — undefined = não renderiza nada
  // (jogo de 1 arquétipo só, ou edição, onde o arquétipo é imutável).
  archetypePicker?: {
    value: string;
    options: Array<{ value: string; label: string }>;
    onChange: (v: string) => void;
  };
};
```

Em `RuleDraftNode`, logo no topo do `flow-card__body` (antes das linhas de match), só quando `!editing && d.archetypePicker`:

```tsx
{!editing && d.archetypePicker && (
  <Select
    size="xs"
    classNames={FLOW_INPUT}
    data={d.archetypePicker.options}
    value={d.archetypePicker.value}
    onChange={(v) => v && d.archetypePicker!.onChange(v)}
    allowDeselect={false}
    comboboxProps={{ withinPortal: true }}
  />
)}
```

## `GameEventRulesCanvas.tsx`

### Remover

- `<Select>` de arquétipo na toolbar (linhas 678-692 do arquivo atual) — sai de vez.
- O antigo comentário/lógica que só trocava `archetype` via clique no Select (o `onChange` inteiro do Select some junto).

### Novo estado

```ts
const [dockCollapsed, setDockCollapsed] = useState(false);
const [listFilter, setListFilter] = useState("");
// draftArchetype: só relevante durante CRIAÇÃO num jogo com >1 arquétipo —
// default no primeiro; sincroniza com `arch` quando o usuário troca no
// picker do form (spec item 6).
const [draftArchetype, setDraftArchetype] = useState<MultiplayerProfile["archetype"]>(archetypeOptions[0]);
```

`arch` continua exatamente como hoje (derivado da regra focada) — só que agora TAMBÉM é a base do "Nova regra" quando o jogo tem 1 arquétipo só; com mais de um, `draftArchetype` manda (inicializado a partir de `arch`/`archetypeOptions[0]` no momento de abrir o form).

### "Nova regra"

```ts
onClick={() => {
  setSelectedId(null);
  setDraftArchetype(arch); // parte do arquétipo atualmente ativo (senão o 1º)
  setDraft({ editingRuleID: null, rows: [EMPTY_ROW], linked: [], news: newGhost(), error: null, saving: false });
}}
```

`saveDraft`'s branch de criação:
```ts
await api.createEventRule(gameId, {
  archetype: archetypeOptions.length > 1 ? draftArchetype : arch,
  match,
  eventTypes,
});
```

`draftData` (dentro do `useMemo` de nodes/edges) ganha `archetypePicker` só quando `!draft.editingRuleID && archetypeOptions.length > 1`:
```ts
archetypePicker: !draft.editingRuleID && archetypeOptions.length > 1
  ? {
      value: draftArchetype,
      options: archetypeOptions.map((a) => ({ value: a, label: ARCHETYPE_META[a].label })),
      onChange: (v) => setDraftArchetype(v as MultiplayerProfile["archetype"]),
    }
  : undefined,
```

### `EventRulesDock` (novo componente local — lista + toolbar + info, tudo num painel só)

```tsx
function EventRulesDock({
  rules, // visibleRules, já filtradas por arquétipos do jogo (feito no pai)
  focusRuleId,
  onFocusRule,
  listFilter,
  onListFilter,
  collapsed,
  onToggleCollapsed,
  draft,
  onNewRule,
  relatedKinds,
  onRelatedKinds,
  statIgnoreValue,
  onStatIgnoreValue,
  search,
  onSearch,
  onDelete,
  onToggleSuppress,
}: { /* ... */ }) {
  const filtered = rules.filter(matchesListFilter(listFilter));
  return (
    <Paper
      withBorder shadow="md" w={320}
      style={{ background: "rgba(18, 24, 33, 0.95)", maxHeight: "calc(100vh - 190px)", display: "flex", flexDirection: "column" }}
    >
      <Group justify="space-between" wrap="nowrap" p="xs" pb={collapsed ? "xs" : 4}>
        <Group gap={6} wrap="nowrap">
          <Text size="xs" fw={700}>Event Rules</Text>
          <Badge size="xs" variant="light" color="gray">{rules.length}</Badge>
          <Popover width={300} withArrow position="bottom-start">
            <Popover.Target>
              <ActionIcon size="xs" variant="subtle" color="gray" aria-label="Como funciona">
                <IconInfoCircle size={13} />
              </ActionIcon>
            </Popover.Target>
            <Popover.Dropdown>
              <Text size="xs">
                Cada regra casa um subconjunto exato de "facts" do evento recebido (ex.: role/event/outcome
                no host_world_dropin; placement/tier no battle_royale; outcome nos demais) e soma os event
                types listados ao progresso das conquistas cadastradas na aba Achievements. Regras{" "}
                <b>globais</b> valem pra todo jogo com o arquétipo (semeadas por padrão, não editáveis
                aqui); regras do <b>jogo</b> só valem pra este jogo.
              </Text>
            </Popover.Dropdown>
          </Popover>
        </Group>
        <ActionIcon size="sm" variant="subtle" color="gray" onClick={onToggleCollapsed}>
          {collapsed ? <IconChevronDown size={14} /> : <IconChevronUp size={14} />}
        </ActionIcon>
      </Group>
      <Collapse in={!collapsed} style={{ display: "flex", flexDirection: "column", minHeight: 0 }}>
        <Stack gap={6} px="xs" pb="xs">
          <Group gap={6} wrap="nowrap">
            <Button size="xs" leftSection={<IconPlus size={13} />} disabled={!!draft} onClick={onNewRule}>
              Nova
            </Button>
            <TextInput
              size="xs" placeholder="filtrar…" style={{ flex: 1 }}
              value={listFilter} onChange={(e) => onListFilter(e.currentTarget.value)}
              rightSection={listFilter ? <CloseButton size="xs" onClick={() => onListFilter("")} /> : undefined}
            />
          </Group>
          <Group gap={4} wrap="nowrap">
            <Text size="xs" c="dimmed">Relacionadas:</Text>
            <Chip.Group multiple value={[...relatedKinds]} onChange={(v) => onRelatedKinds(new Set(v as SourceKind[]))}>
              <Group gap={4} wrap="nowrap">
                <Chip size="xs" value="structural" disabled={!focusRuleId}>Facts</Chip>
                <Chip size="xs" value="mode" disabled={!focusRuleId}>Modo</Chip>
                <Chip size="xs" value="stat" disabled={!focusRuleId}>Stats</Chip>
              </Group>
            </Chip.Group>
          </Group>
          {relatedKinds.has("stat") && (
            <Switch size="xs" label="qualquer limiar" checked={statIgnoreValue} onChange={(e) => onStatIgnoreValue(e.currentTarget.checked)} />
          )}
          <TextInput
            size="xs" placeholder="buscar no subgrafo…" leftSection={<IconSearch size={12} />}
            value={search} onChange={(e) => onSearch(e.currentTarget.value)}
            rightSection={search ? <CloseButton size="xs" onClick={() => onSearch("")} /> : undefined}
          />
        </Stack>
        <ScrollArea style={{ flex: 1, minHeight: 0 }}>
          <Table striped highlightOnHover fz="xs">
            <Table.Tbody>
              {filtered.map((r) => (
                <Table.Tr
                  key={r.id}
                  onClick={() => onFocusRule(focusRuleId === r.id ? null : r.id)}
                  style={{ cursor: "pointer", opacity: r.suppressed ? 0.5 : undefined, background: r.id === focusRuleId ? "var(--mantine-color-blue-light)" : undefined }}
                >
                  <Table.Td>
                    <Badge size="xs" variant="outline" color={r.scope === "global" ? "gray" : "teal"}>
                      {r.scope === "global" ? "global" : "jogo"}
                    </Badge>
                  </Table.Td>
                  <Table.Td>
                    <Group gap={3} wrap="wrap">
                      {r.eventTypes.map((et) => (
                        <Badge key={et} size="xs" variant="light" color={ARCHETYPE_META[r.archetype].color}>{et}</Badge>
                      ))}
                    </Group>
                  </Table.Td>
                  <Table.Td onClick={(e) => e.stopPropagation()} w={28}>
                    {r.scope === "game" ? (
                      <ActionIcon size="xs" variant="subtle" color="red" onClick={() => onDelete(r)} aria-label="Remover"><IconTrash size={13} /></ActionIcon>
                    ) : (
                      <ActionIcon size="xs" variant="subtle" color={r.suppressed ? "teal" : "orange"} onClick={() => onToggleSuppress(r)} aria-label={r.suppressed ? "Restaurar" : "Suprimir"}>
                        {r.suppressed ? <IconRotate2 size={13} /> : <IconBan size={13} />}
                      </ActionIcon>
                    )}
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </ScrollArea>
      </Collapse>
    </Paper>
  );
}
```

Notas:
- Sem coluna "Escopo"/arquétipo em texto (spec item 8) — a cor da badge de event type já carrega essa informação (`ARCHETYPE_META[r.archetype].color`).
- `Collapse` recolhido: só a barra de cabeçalho fica (nome + contagem + info + chevron) — mesma UX do painel de detalhe do node.
- `ScrollArea` interno da lista, não da página — resolve o item 1 (sem scroll de PÁGINA) sem esconder regras.

### `FlowCanvas` no return principal

```tsx
<FlowCanvas
  nodes={nodes}
  edges={edges}
  nodeTypes={NODE_TYPES}
  onNodeClick={onNodeClick}
  onPaneClick={() => setSelectedId(null)}
  fitKey={...} // igual hoje
  height="calc(100vh - <offset calibrado, ver Testes>)"
  dock={
    <EventRulesDock
      rules={rules}
      focusRuleId={focusRuleId}
      onFocusRule={onFocusRule}
      listFilter={listFilter}
      onListFilter={setListFilter}
      collapsed={dockCollapsed}
      onToggleCollapsed={() => setDockCollapsed((c) => !c)}
      draft={draft}
      onNewRule={() => { setSelectedId(null); setDraftArchetype(arch); setDraft({ ... }); }}
      relatedKinds={relatedKinds}
      onRelatedKinds={setRelatedKinds}
      statIgnoreValue={statIgnoreValue}
      onStatIgnoreValue={setStatIgnoreValue}
      search={search}
      onSearch={setSearch}
      onDelete={(r) => setDeleting(r)}
      onToggleSuppress={toggleSuppress}
    />
  }
  overlay={/* NodeDetails, inalterado */}
/>
```

- `{!draft && !focusRuleId ? <Text .../> : <FlowCanvas .../>}` (o placeholder "selecione uma regra") deixa de existir como ramo separado — o canvas SEMPRE renderiza agora (a lista/dock precisa estar visível mesmo sem regra focada, pra dar o primeiro clique); o placeholder vira o conteúdo do canvas VAZIO (React Flow com 0 nodes já mostra um fundo vazio — adicionar um `Text` centralizado via `overlay`-like técnica só se ficar estranho vazio, calibrar visualmente).

## `GameEventRules.tsx`

Vira só shell de dado — sem `Group`/`Box`, sem texto, sem modal de delete próprio (o do canvas serve pros dois usos agora), sem `toggleSuppress`/`confirmDelete` (removidos, duplicavam o que já existe no canvas):

```tsx
export function GameEventRules({ gameId, pendingRuleFocus, onConsumePendingRuleFocus }: { ... }) {
  const { data: rules, loading, error, reload } = useAsync(...);
  const { data: modes } = useAsync(...);
  const { data: catalog, reload: reloadCatalog } = useAsync(...);
  function reloadAll() { reload(); reloadCatalog(); }
  const archetypes = gameArchetypes(modes ?? []);
  const archetypeOptions = archetypes.size > 0 ? [...archetypes] : ALL_ARCHETYPES;
  const visibleRules = filterRulesForGame(rules ?? [], archetypes);
  const [focusRuleId, setFocusRuleId] = useState<string | null>(null);

  useEffect(() => {
    if (!pendingRuleFocus || !rules) return;
    setFocusRuleId(pendingRuleFocus.ruleId);
    onConsumePendingRuleFocus?.();
  }, [pendingRuleFocus, rules]);

  if (loading && !rules) return <Loader />;
  if (error) return <Alert color="red">achievements indisponível.</Alert>;

  return (
    <GameEventRulesCanvas
      gameId={gameId}
      rules={visibleRules}
      catalog={catalog ?? []}
      archetypeOptions={archetypeOptions}
      modes={modes ?? []}
      reload={reloadAll}
      focusRuleId={focusRuleId}
      onFocusRule={setFocusRuleId}
      autoEditRuleId={pendingRuleFocus?.edit ? pendingRuleFocus.ruleId : null}
    />
  );
}
```

- `visibleRules.length === 0` deixa de ser um ramo especial na PÁGINA — vira, no máximo, a lista vazia dentro do dock (a mensagem "nenhuma regra" já é óbvia com 0 linhas na tabela do dock); sem necessidade de um estado dedicado.

## Testes / verificação

Sem suíte automatizada (padrão do projeto). `tsc --noEmit` + `npm run build`, smoke manual via Playwright no jogo "Achievements Test" (2 arquétipos: host_world_dropin, queue_arena):
1. Sem scroll de página em 1700×1000 — `document.documentElement.scrollHeight` ≈ altura da viewport (ou `<= `, nunca maior).
2. Dock: recolher/expandir preserva a regra focada; lista tem scroll PRÓPRIO quando excede a altura do dock.
3. Info popover mostra o texto explicativo.
4. Selecionar uma regra na lista do dock foca o canvas nela (paridade com o comportamento atual da coluna fixa).
5. "Nova regra" nesse jogo (2 arquétipos) mostra o Select de arquétipo dentro do form in-canvas; escolher um e salvar cria a regra no arquétipo certo (conferir via GET da API).
6. Jogo com 1 arquétipo só (buscar outro jogo de teste, ou criar um): "Nova regra" NÃO mostra o Select — cria direto no único arquétipo.
7. Badges de event type na lista usam a cor do arquétipo (grape pra host_world_dropin, blue pra queue_arena — bater com `ARCHETYPE_META`); texto do arquétipo não aparece mais na coluna Escopo.
8. Editar/deletar/suprimir uma regra pela lista do dock funciona igual ao fluxo anterior (reaproveita `deleting`/`toggleSuppress` do canvas).
9. Regras relacionadas, busca no subgrafo, drift, quick-create de achievement, navegação vinda da aba Achievements ("Editar regra"/"Ver regra") — regressão, tudo igual ao comportamento pré-058.
10. Calibrar o valor exato de `height="calc(100vh - Npx)"` via screenshot (igual à calibração de `COL_X`/alturas em `rulesGraph.ts` — medir, não adivinhar).

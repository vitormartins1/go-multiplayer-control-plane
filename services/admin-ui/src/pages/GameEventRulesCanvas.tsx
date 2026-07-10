import {
  Badge,
  Button,
  CloseButton,
  Drawer,
  Group,
  Modal,
  NumberInput,
  Paper,
  Select,
  Stack,
  Switch,
  Text,
  Textarea,
  TextInput,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { IconBan, IconPlus, IconRotate2, IconSearch, IconTrash, IconTrophy } from "@tabler/icons-react";
import type { Edge, Node, NodeTypes } from "@xyflow/react";
import { useEffect, useMemo, useRef, useState } from "react";
import { FlowCanvas } from "../components/flow/FlowCanvas";
import {
  AchievementNode,
  EventTypeDraftNode,
  EventTypeNode,
  RuleDraftNode,
  RuleNode,
  SectionLabelNode,
  SourceNode,
  type RuleDraftData,
  type TypeDraftData,
} from "../components/flow/nodes";
import {
  api,
  HttpError,
  type CatalogAchievement,
  type EventRule,
  type MultiplayerMode,
  type MultiplayerProfile,
} from "../lib/api";
import { ARCHETYPE_META } from "../lib/archetypeMeta";
import {
  buildMatchValue,
  condLabel,
  condOp,
  condRaw,
  condShort,
  EMPTY_ROW,
  type MatchRow,
  type MatchValue,
} from "../lib/matchCond";
import {
  buildRulesGraph,
  pathHighlight,
  repackVisible,
  siblingRuleIds,
  STAT_PREFIX,
  type AchievementNodeData,
  type DraftGraphInput,
  type EventTypeNodeData,
  type RuleNodeData,
  type SourceNodeData,
  type StatInfo,
} from "../lib/rulesGraph";

// Referência estável (module-level) — nodeTypes recriado por render faria o
// React Flow re-montar todos os nós.
const NODE_TYPES: NodeTypes = {
  sectionLabel: SectionLabelNode,
  source: SourceNode,
  rule: RuleNode,
  ruleDraft: RuleDraftNode,
  eventType: EventTypeNode,
  eventTypeDraft: EventTypeDraftNode,
  achievement: AchievementNode,
};

// scoringStats mora em params — os 4 arquétipos declaram (host_world_dropin
// fechou o gap na spec game-registry/scoringStats-hwd); helper genérico só
// pra manter o narrowing de tipo do union discriminado.
function modeStats(m: MultiplayerMode): string[] {
  return "scoringStats" in m.params ? m.params.scoringStats : [];
}

// Estado do form in-canvas (spec 048; edição na 050). editingRuleID null =
// criação (nó rascunho novo); setado = o nó da própria regra vira o form —
// um único estado garante criação e edição mutuamente exclusivas.
type DraftState = {
  editingRuleID: string | null;
  rows: MatchRow[];
  linked: string[]; // event types existentes ligados
  news: Array<{ id: string; value: string }>; // types novos, editáveis inline
  error: string | null;
  saving: boolean;
};

const ID_RE = /^[a-z][a-z0-9_]*$/;

// GameEventRulesCanvas (specs 043–048 / ADR-0028): grafo de fontes unificadas
// (condições nas edges) sobre o MESMO CRUD da tabela. Criação in-canvas: o nó
// rascunho é editado DENTRO do canvas (inputs nodrag), event types novos
// nascem pelo "+" do rascunho, e nada persiste antes do Salvar.
export function GameEventRulesCanvas({
  gameId,
  rules,
  catalog,
  archetypeOptions,
  modes,
  reload,
  focusRuleId,
  onFocusRule,
}: {
  gameId: string;
  rules: EventRule[];
  catalog: CatalogAchievement[];
  archetypeOptions: MultiplayerProfile["archetype"][];
  modes: MultiplayerMode[];
  reload: () => void;
  // focusRuleId (spec 055): regra selecionada na lista externa — canvas some
  // vazio até uma regra ser focada, e mostra só o subgrafo dela.
  focusRuleId: string | null;
  onFocusRule: (ruleId: string | null) => void;
}) {
  const [archetype, setArchetype] = useState<MultiplayerProfile["archetype"]>(archetypeOptions[0]);
  const arch = archetypeOptions.includes(archetype) ? archetype : archetypeOptions[0];

  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [showStats, setShowStats] = useState(true);
  const [showRelated, setShowRelated] = useState(false);
  const [draft, setDraft] = useState<DraftState | null>(null);
  const [deleting, setDeleting] = useState<EventRule | null>(null);
  // event type órfão a partir do qual criar um achievement (quick-create).
  const [quickAchType, setQuickAchType] = useState<string | null>(null);
  const ghostSeq = useRef(0);

  // sincroniza o overlay de detalhes com a regra focada externamente (lista);
  // nunca o contrário — onFocusRule só é chamado a partir do clique real (mais
  // abaixo), pra não criar ciclo entre prop e state interno. Também troca a
  // aba de arquétipo do canvas se a regra focada for de outro arquétipo — a
  // lista mostra todos juntos, mas o grafo só constrói o arquétipo ativo
  // (senão a regra focada nem existiria no `graph` e o filtro esvaziaria tudo).
  useEffect(() => {
    setSelectedId(focusRuleId ? `r:${focusRuleId}` : null);
    if (focusRuleId) {
      const rule = rules.find((r) => r.id === focusRuleId);
      if (rule && rule.archetype !== archetype) setArchetype(rule.archetype);
    }
  }, [focusRuleId, rules, archetype]);

  const archRules = useMemo(() => rules.filter((r) => r.archetype === arch), [rules, arch]);

  // Stats do arquétipo (spec 045): declaradas + drift (usada por achievement
  // sem declaração em NENHUM modo).
  const statsForArch = useMemo<StatInfo[]>(() => {
    const declaredHere = new Set(modes.filter((m) => m.archetype === arch).flatMap(modeStats));
    const declaredAnywhere = new Set(modes.flatMap(modeStats));
    const out: StatInfo[] = [...declaredHere].sort().map((key) => ({ key, declared: true }));
    const drift = new Set(
      catalog.map((a) => a.statKey).filter((k): k is string => !!k && !declaredAnywhere.has(k)),
    );
    for (const key of [...drift].sort()) out.push({ key, declared: false });
    return out;
  }, [modes, arch, catalog]);

  // Vocabulário derivado do dado real (nunca hardcoded, ADR-0027).
  const keySuggestions = useMemo(
    () => [
      ...new Set([
        ...archRules.flatMap((r) => Object.keys(r.match)),
        "mode",
        ...statsForArch.map((s) => `${STAT_PREFIX}${s.key}`),
      ]),
    ],
    [archRules, statsForArch],
  );
  const modeIdSuggestions = useMemo(
    () => modes.filter((m) => m.archetype === arch).map((m) => m.modeId),
    [modes, arch],
  );
  // valores já usados por chave (só igualdades — sugestão de valor).
  const knownValues = useMemo(() => {
    const m = new Map<string, Set<string>>();
    for (const r of archRules) {
      for (const [k, v] of Object.entries(r.match)) {
        if (typeof v === "string") m.set(k, new Set([...(m.get(k) ?? []), v]));
      }
    }
    return m;
  }, [archRules]);
  const typeSuggestions = useMemo(
    () => [...new Set(archRules.flatMap((r) => r.eventTypes))],
    [archRules],
  );
  // event types emitidos por regras INCONDICIONAIS (match vazio) — o hint de
  // redundância do painel (o motor deduplica; emissão repetida é inócua).
  // Regra suprimida (spec 017/051) não emite nada — não pode tornar outra
  // regra "redundante".
  const unconditionalTypes = useMemo(
    () =>
      new Set(
        archRules
          .filter((r) => !r.suppressed && Object.keys(r.match).length === 0)
          .flatMap((r) => r.eventTypes),
      ),
    [archRules],
  );

  // Rascunho → entrada do build (só condições VÁLIDAS geram edges ao vivo).
  const draftInput = useMemo<DraftGraphInput | undefined>(() => {
    if (!draft) return undefined;
    const conds: DraftGraphInput["conds"] = [];
    for (const row of draft.rows) {
      if (!row.key.trim()) continue;
      const v = buildMatchValue(row.op, row.value);
      if (v !== null) conds.push({ key: row.key.trim(), label: condShort(v) });
    }
    return {
      conds,
      linkedTypes: draft.linked,
      newTypes: draft.news,
      rows: draft.rows.length,
      editingRuleID: draft.editingRuleID ?? undefined,
    };
  }, [draft]);

  const graph = useMemo(
    () =>
      buildRulesGraph(archRules, catalog, {
        stats: showStats ? statsForArch : undefined,
        draft: draftInput,
      }),
    [archRules, catalog, showStats, statsForArch, draftInput],
  );

  // focusPath (spec 055): subgrafo da regra focada. Um draft ativo (criar ou
  // editar) desliga o filtro — o fluxo de criação/edição in-canvas (specs
  // 050/052) continua vendo o grafo cheio, sem mudança nenhuma nesse ramo.
  const focusPath = useMemo(() => {
    if (!focusRuleId || draft) return null;
    const base = pathHighlight(graph.edges, `r:${focusRuleId}`);
    if (!showRelated) return base;
    const nodeIds = new Set(base.nodeIds);
    const edgeIds = new Set(base.edgeIds);
    for (const siblingId of siblingRuleIds(graph.edges, base.nodeIds)) {
      const p = pathHighlight(graph.edges, siblingId);
      p.nodeIds.forEach((id) => nodeIds.add(id));
      p.edgeIds.forEach((id) => edgeIds.add(id));
    }
    return { nodeIds, edgeIds };
  }, [graph, focusRuleId, showRelated, draft]);

  // --- callbacks do rascunho (injetados no node.data) ---
  function patchDraft(patch: Partial<DraftState>) {
    setDraft((d) => (d ? { ...d, ...patch, error: patch.error !== undefined ? patch.error : null } : d));
  }
  function onRow(i: number, patch: Partial<MatchRow>) {
    setDraft((d) =>
      d ? { ...d, error: null, rows: d.rows.map((r, idx) => (idx === i ? { ...r, ...patch } : r)) } : d,
    );
  }
  function onRemoveRow(i: number) {
    setDraft((d) => (d ? { ...d, rows: d.rows.filter((_, idx) => idx !== i) } : d));
  }
  function onAddRow(prefill?: MatchRow) {
    setDraft((d) => {
      if (!d) return d;
      // clique numa fonte: substitui a linha vazia pendente pela pré-preenchida;
      // "+" manual: só acrescenta uma linha vazia.
      const rows = prefill ? [...d.rows.filter((r) => r.key.trim()), prefill] : [...d.rows, EMPTY_ROW];
      return { ...d, error: null, rows };
    });
  }
  // newGhost gera a ÚNICA caixinha de event type novo do form (spec 052) —
  // nasce junto com o rascunho/edição, sem "+" e sem botão de fechar.
  function newGhost(): Array<{ id: string; value: string }> {
    ghostSeq.current += 1;
    return [{ id: String(ghostSeq.current), value: "" }];
  }
  function cancelDraft() {
    setDraft(null);
  }

  // startEdit transforma o nó da regra no form (spec 050) — mesmo estado da
  // criação, pré-carregado do match/types persistidos. Abrir edição descarta
  // rascunho aberto (e vice-versa): um form por vez.
  function startEdit(rule: EventRule) {
    setSelectedId(null);
    const rows: MatchRow[] = Object.entries(rule.match).map(([key, v]) => ({
      key,
      op: condOp(v),
      value: condRaw(v),
    }));
    setDraft({
      editingRuleID: rule.id,
      rows: rows.length > 0 ? rows : [EMPTY_ROW],
      linked: [...rule.eventTypes],
      news: newGhost(),
      error: null,
      saving: false,
    });
  }

  async function saveDraft() {
    if (!draft) return;
    const invalid = draft.rows.some((r) => r.key.trim() && buildMatchValue(r.op, r.value) === null);
    if (invalid) {
      patchDraft({ error: "Valor inválido pro operador (numérico exige número; `in` exige lista)." });
      return;
    }
    const match: Record<string, MatchValue> = {};
    for (const row of draft.rows) {
      if (!row.key.trim()) continue;
      match[row.key.trim()] = buildMatchValue(row.op, row.value)!;
    }
    const eventTypes = [...draft.linked, ...draft.news.map((n) => n.value.trim()).filter(Boolean)];
    if (Object.keys(match).length === 0 || eventTypes.length === 0) {
      patchDraft({ error: "Preencha ao menos uma condição e um event type." });
      return;
    }
    patchDraft({ saving: true });
    try {
      if (draft.editingRuleID) {
        // archetype não é editável (spec 016) — o PUT leva só o conteúdo.
        await api.updateEventRule(gameId, draft.editingRuleID, { match, eventTypes });
        notifications.show({ color: "green", message: "Regra atualizada." });
      } else {
        await api.createEventRule(gameId, { archetype: arch, match, eventTypes });
        notifications.show({ color: "green", message: "Regra adicionada." });
      }
      setDraft(null);
      reload();
    } catch (e) {
      patchDraft({
        saving: false,
        error:
          e instanceof HttpError && e.status === 400
            ? "Dados inválidos (event types minúsculos ex.: raid_win; confira as condições)."
            : "Falha ao salvar (serviço indisponível?).",
      });
    }
  }

  // Esc cancela o rascunho (spec 048).
  useEffect(() => {
    if (!draft) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setDraft(null);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [draft]);

  // --- decoração: injeta data do rascunho e aplica busca/seleção ---
  const { nodes, edges } = useMemo(() => {
    const term = search.trim().toLowerCase();

    // spec 055: sem regra focada (e fora de um draft), canvas vazio — o
    // grafo completo só aparece dentro do fluxo de criação/edição (draft),
    // que fica intocado. Com foco, corta pro subgrafo (+ vizinhas, se
    // showRelated) — sectionLabel só sobrevive se alguma source do grupo
    // dela estiver visível (evita cabeçalho de seção órfão).
    let visibleNodes = graph.nodes;
    let visibleEdges = graph.edges;
    if (!draft) {
      if (!focusPath) {
        visibleNodes = [];
        visibleEdges = [];
      } else {
        const filtered = graph.nodes.filter((n) => {
          if (focusPath.nodeIds.has(n.id)) return true;
          if (n.type === "sectionLabel" && n.id.startsWith("lbl:")) {
            const kind = n.id.slice(4);
            return graph.nodes.some(
              (s) =>
                s.type === "source" &&
                (s.data as unknown as SourceNodeData).kind === kind &&
                focusPath.nodeIds.has(s.id),
            );
          }
          return false;
        });
        // repackVisible: sem isso, os nodes filtrados mantêm as coordenadas Y
        // calculadas pro arquétipo inteiro — vãos enormes onde ficavam os
        // nodes escondidos (achado do Vitor testando o foco na prática).
        visibleNodes = repackVisible(filtered);
        visibleEdges = graph.edges.filter((e) => focusPath.edgeIds.has(e.id));
      }
    }

    const path = !term && selectedId && !draft ? pathHighlight(visibleEdges, selectedId) : null;

    const draftData: RuleDraftData | null = draft
      ? {
          mode: draft.editingRuleID ? "edit" : "create",
          rows: draft.rows,
          keySuggestions,
          valueSuggestions: (row) =>
            row.op === "eq" || row.op === "ne"
              ? [
                  ...new Set([
                    ...(knownValues.get(row.key) ?? []),
                    ...(row.key === "mode" ? modeIdSuggestions : []),
                  ]),
                ]
              : [],
          error: draft.error,
          saving: draft.saving,
          onRow,
          onRemoveRow,
          onAddRow: () => onAddRow(),
          onSave: saveDraft,
          onCancel: cancelDraft,
        }
      : null;

    const matches = (n: Node): boolean => {
      if (!term) return true;
      return nodeText(n).includes(term);
    };
    const cls = (...parts: Array<string | undefined | false>) => parts.filter(Boolean).join(" ") || undefined;

    const nodes: Node[] = visibleNodes.map((n) => {
      let node = n;
      if (draftData && n.type === "ruleDraft") {
        // cobre o rascunho de criação (r:__draft__) E a regra em edição
        // (nó dela vira ruleDraft no build) — só existe um form por vez.
        node = { ...n, data: draftData as unknown as Record<string, unknown> };
      } else if (!draft && n.type === "rule") {
        // fora de um form ativo, regras do JOGO ganham a ação de editar
        // (hover no header, spec 050).
        const { rule } = n.data as unknown as RuleNodeData;
        if (rule.scope === "game") {
          node = {
            ...n,
            data: { ...n.data, onEdit: () => startEdit(rule) } as unknown as Record<string, unknown>,
          };
        }
      } else if (draft && n.id.startsWith("t:__new__:")) {
        const gid = n.id.slice("t:__new__:".length);
        const entry = draft.news.find((g) => g.id === gid);
        const td: TypeDraftData = {
          value: entry?.value ?? "",
          suggestions: typeSuggestions,
          onChange: (v) =>
            setDraft((d) =>
              d ? { ...d, error: null, news: d.news.map((g) => (g.id === gid ? { ...g, value: v } : g)) } : d,
            ),
        };
        node = { ...n, data: td as unknown as Record<string, unknown> };
      }
      const extra = term
        ? matches(n)
          ? "is-active"
          : "is-faded"
        : path
          ? path.nodeIds.has(n.id)
            ? "is-active"
            : "is-faded"
          : undefined;
      return { ...node, className: cls(n.className, extra) };
    });

    const matchedNodeIds = term ? new Set(visibleNodes.filter(matches).map((n) => n.id)) : null;
    const edges: Edge[] = visibleEdges.map((e) => {
      if (e.className === "is-draft-edge") {
        return { ...e, animated: true, style: { stroke: "#fab005", strokeWidth: 1.5 } };
      }
      // Edge de métrica (spec 049): discreta e SEM label enquanto inativa (o
      // critério mora no nó do achievement); acesa e com label quando entra
      // no caminho destacado ou casa a busca.
      const metric = e.className === "is-metric-edge";
      if (matchedNodeIds) {
        const on = matchedNodeIds.has(e.source) && matchedNodeIds.has(e.target);
        return on
          ? { ...e, className: metric ? undefined : e.className, style: { stroke: "#748ffc", strokeWidth: 1.5 } }
          : { ...e, className: cls(e.className, "is-faded"), style: { stroke: "#3a4657" }, label: metric ? undefined : e.label };
      }
      return path && path.edgeIds.has(e.id)
        ? { ...e, className: metric ? undefined : e.className, animated: true, style: { stroke: "#748ffc", strokeWidth: 2 } }
        : {
            ...e,
            className: cls(e.className, path ? "is-faded" : undefined),
            style: { stroke: "#3a4657" },
            label: metric ? undefined : e.label,
          };
    });
    return { nodes, edges };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [graph, selectedId, draft, search, keySuggestions, knownValues, modeIdSuggestions, typeSuggestions, focusPath]);

  const selectedNode = useMemo(
    () => graph.nodes.find((n) => n.id === selectedId) ?? null,
    [graph, selectedId],
  );

  // Com o rascunho ativo, o canvas é superfície de montagem: fonte adiciona
  // condição pré-preenchida (stat entra com ≥); type existente liga/desliga.
  function onNodeClick(id: string) {
    const node = graph.nodes.find((n) => n.id === id);
    if (node?.type === "sectionLabel") return; // rótulo de seção não é selecionável
    if (!draft) {
      setSelectedId(id);
      // spec 055: clicar numa regra (a focada, ou uma vizinha trazida pelo
      // toggle "relacionadas") refoca o canvas nela e sincroniza a lista —
      // só regra dispara isso; clicar em source/eventType/achievement só
      // abre o overlay de detalhes, sem mudar o que está visível.
      if (node?.type === "rule" || node?.type === "ruleDraft") {
        onFocusRule(id.startsWith("r:") ? id.slice(2) : id);
      }
      return;
    }
    if (node?.type === "source") {
      const d = node.data as unknown as SourceNodeData;
      onAddRow({ key: d.sourceKey, op: d.kind === "stat" ? "gte" : "eq", value: "" });
    } else if (node?.type === "eventType") {
      const d = node.data as unknown as EventTypeNodeData;
      setDraft((dr) =>
        dr
          ? {
              ...dr,
              error: null,
              linked: dr.linked.includes(d.eventType)
                ? dr.linked.filter((t) => t !== d.eventType)
                : [...dr.linked, d.eventType],
            }
          : dr,
      );
    }
  }

  async function confirmDelete() {
    if (!deleting) return;
    try {
      await api.deleteEventRule(gameId, deleting.id);
      notifications.show({ color: "green", message: "Regra removida." });
      setDeleting(null);
      setSelectedId(null);
      reload();
    } catch {
      notifications.show({ color: "red", message: "Falha ao remover (serviço indisponível?)." });
      setDeleting(null);
    }
  }

  // toggleSuppress liga/desliga a supressão de uma regra GLOBAL pra este jogo
  // (spec achievements/017, V2b) — reversível com um clique, sem modal (ao
  // contrário do delete de override, que é destrutivo).
  async function toggleSuppress(rule: EventRule) {
    try {
      if (rule.suppressed) {
        await api.unsuppressEventRule(gameId, rule.id);
        notifications.show({ color: "green", message: "Regra restaurada." });
      } else {
        await api.suppressEventRule(gameId, rule.id);
        notifications.show({ color: "orange", message: "Regra suprimida para este jogo." });
      }
      reload();
    } catch {
      notifications.show({ color: "red", message: "Falha ao atualizar a supressão (serviço indisponível?)." });
    }
  }

  return (
    <Stack gap="xs">
      <Group gap="xs">
        <Select
          size="xs"
          w={230}
          data={archetypeOptions.map((a) => ({ value: a, label: ARCHETYPE_META[a].label }))}
          value={arch}
          onChange={(v) => {
            if (v) {
              setArchetype(v as MultiplayerProfile["archetype"]);
              setSelectedId(null);
              setDraft(null);
              onFocusRule(null); // regra focada pode não pertencer ao arquétipo novo
            }
          }}
          allowDeselect={false}
        />
        <Button
          size="xs"
          leftSection={<IconPlus size={14} />}
          disabled={!!draft}
          onClick={() => {
            setSelectedId(null);
            setDraft({ editingRuleID: null, rows: [EMPTY_ROW], linked: [], news: newGhost(), error: null, saving: false });
          }}
        >
          Nova regra
        </Button>
        <Switch size="xs" label="Stats" checked={showStats} onChange={(e) => setShowStats(e.currentTarget.checked)} />
        <Switch
          size="xs"
          label="Regras relacionadas"
          checked={showRelated}
          disabled={!focusRuleId}
          onChange={(e) => setShowRelated(e.currentTarget.checked)}
        />
        <TextInput
          size="xs"
          w={200}
          placeholder="buscar no subgrafo…"
          leftSection={<IconSearch size={13} />}
          value={search}
          onChange={(e) => setSearch(e.currentTarget.value)}
          rightSection={search ? <CloseButton size="xs" onClick={() => setSearch("")} /> : undefined}
        />
        {draft && (
          <Text size="xs" c="dimmed">
            Montando rascunho — clique numa fonte pra adicionar condição; num event type pra ligar.
          </Text>
        )}
      </Group>

      {!draft && !focusRuleId ? (
        <Text c="dimmed" ta="center" py={80}>
          Selecione uma regra na lista para visualizar.
        </Text>
      ) : (
        <FlowCanvas
          nodes={nodes}
          edges={edges}
          nodeTypes={NODE_TYPES}
          onNodeClick={onNodeClick}
          onPaneClick={() => setSelectedId(null)}
          fitKey={`${arch}|${focusRuleId ?? "none"}|${showRelated}|${!!draft}`}
          height="calc(100vh - 345px)"
          overlay={
            selectedNode && !draft ? (
              <Paper withBorder p="sm" w={290} shadow="md" style={{ background: "rgba(18, 24, 33, 0.95)" }}>
                <CloseButton
                  size="sm"
                  onClick={() => setSelectedId(null)}
                  style={{ position: "absolute", top: 8, right: 8 }}
                  aria-label="Fechar detalhe"
                />
                <NodeDetails
                  node={selectedNode}
                  unconditionalTypes={unconditionalTypes}
                  onDelete={(r) => setDeleting(r)}
                  onToggleSuppress={toggleSuppress}
                  onQuickAch={(et) => setQuickAchType(et)}
                />
              </Paper>
            ) : undefined
          }
        />
      )}

      {/* Quick-create de achievement a partir de um event type órfão (spec
          048) — mesmos campos/validação do form da aba Achievements, critério
          fixo "por evento" com o type pré-preenchido. */}
      <QuickAchievementDrawer
        gameId={gameId}
        eventType={quickAchType}
        onClose={() => setQuickAchType(null)}
        onCreated={() => {
          setQuickAchType(null);
          reload();
        }}
      />

      {/* Confirmar remoção */}
      <Modal opened={!!deleting} onClose={() => setDeleting(null)} title="Remover regra">
        <Stack>
          <Text size="sm">
            Remover a regra <code>{deleting?.archetype}</code>? Esta ação não pode ser desfeita.
          </Text>
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setDeleting(null)}>
              Cancelar
            </Button>
            <Button color="red" onClick={confirmDelete}>
              Remover
            </Button>
          </Group>
        </Stack>
      </Modal>
    </Stack>
  );
}

// nodeText: texto pesquisável por tipo de nó (busca da toolbar).
function nodeText(n: Node): string {
  switch (n.type) {
    case "source": {
      const d = n.data as unknown as SourceNodeData;
      return `${d.sourceKey} ${d.conds.join(" ")}`.toLowerCase();
    }
    case "rule": {
      const { rule } = n.data as unknown as RuleNodeData;
      return `${rule.scope} ${Object.entries(rule.match)
        .map(([k, v]) => condLabel(k, v))
        .join(" ")} ${rule.eventTypes.join(" ")}`.toLowerCase();
    }
    case "eventType": {
      const d = n.data as unknown as EventTypeNodeData;
      return d.eventType.toLowerCase();
    }
    case "achievement": {
      const d = n.data as unknown as AchievementNodeData;
      return `${d.name} ${d.eventType ?? ""} ${d.statKey ?? ""}`.toLowerCase();
    }
    default:
      return "";
  }
}

// NodeDetails traduz o nó selecionado pro painel flutuante.
function NodeDetails({
  node,
  unconditionalTypes,
  onDelete,
  onToggleSuppress,
  onQuickAch,
}: {
  node: Node;
  unconditionalTypes: Set<string>;
  onDelete: (r: EventRule) => void;
  onToggleSuppress: (r: EventRule) => void;
  onQuickAch: (eventType: string) => void;
}) {
  if (node.type === "rule") {
    const { rule } = node.data as unknown as RuleNodeData;
    const global = rule.scope === "global";
    const suppressed = global && rule.suppressed;
    const redundant =
      Object.keys(rule.match).length > 0
        ? rule.eventTypes.filter((et) => unconditionalTypes.has(et))
        : [];
    return (
      <Stack gap="xs">
        <Group gap="xs">
          <Text fw={600} size="sm">
            Regra
          </Text>
          <Badge size="xs" variant="outline" color={global ? "gray" : "teal"}>
            {global ? "global" : "jogo"}
          </Badge>
        </Group>
        <Text size="xs" c="dimmed">
          Match (todas as condições precisam bater nos facts do evento):
        </Text>
        <Group gap={4}>
          {Object.entries(rule.match).length === 0 ? (
            <Text size="xs" c="dimmed">
              (sempre bate)
            </Text>
          ) : (
            Object.entries(rule.match).map(([k, v]) => (
              <Badge key={k} size="xs" variant="light" color="gray" tt="none">
                {condLabel(k, v)}
              </Badge>
            ))
          )}
        </Group>
        <Text size="xs" c="dimmed">
          Emite:
        </Text>
        <Group gap={4}>
          {rule.eventTypes.map((et) => (
            <Badge key={et} size="xs" variant="light" color="violet" tt="none">
              {et}
            </Badge>
          ))}
        </Group>
        {redundant.length > 0 && (
          <Text size="xs" c="yellow.5">
            ⚠ {redundant.join(", ")} já é emitido incondicionalmente por outra regra — esta emissão
            não adiciona nada (o motor deduplica). Inócuo, mas provavelmente não é o que você quis.
          </Text>
        )}
        {global ? (
          <>
            <Text size="xs" c="dimmed">
              {suppressed
                ? "Suprimida — não contribui para as avaliações deste jogo."
                : "Regra global do arquétipo (semeada por migration) — somente leitura, mas pode ser suprimida para este jogo."}
            </Text>
            <Button
              variant="light"
              color={suppressed ? "teal" : "orange"}
              size="xs"
              leftSection={suppressed ? <IconRotate2 size={14} /> : <IconBan size={14} />}
              onClick={() => onToggleSuppress(rule)}
            >
              {suppressed ? "Restaurar" : "Suprimir para este jogo"}
            </Button>
          </>
        ) : (
          <Button variant="light" color="red" size="xs" leftSection={<IconTrash size={14} />} onClick={() => onDelete(rule)}>
            Remover regra
          </Button>
        )}
      </Stack>
    );
  }
  if (node.type === "source") {
    const d = node.data as unknown as SourceNodeData;
    const isStat = d.kind === "stat";
    return (
      <Stack gap="xs">
        <Text fw={600} size="sm">
          {isStat ? "Stat (métrica)" : d.kind === "mode" ? "Modo de jogo" : "Fact"}
        </Text>
        <Badge variant="light" color={isStat ? "orange" : "blue"} tt="none" style={{ alignSelf: "flex-start" }}>
          {d.sourceKey}
        </Badge>
        {d.drift && (
          <Text size="xs" c="red">
            Nenhum modo deste jogo declara esta stat — achievements por ela podem nunca progredir
            (drift entre catálogo e modos).
          </Text>
        )}
        <Text size="xs" c="dimmed">
          {isStat
            ? "Métrica por partida do jogador. Em regras vira condição numérica (stat.<chave> ≥ N); em achievements por métrica alimenta o progresso DIRETO, sem regra (edge com o critério)."
            : d.kind === "mode"
              ? "Modo do jogo (modeId do envelope) — condição `mode = <id>` restringe a regra a um modo."
              : "Fact estrutural do evento — as condições em que aparece estão nas edges."}
        </Text>
        {d.conds.length > 0 && (
          <>
            <Text size="xs" c="dimmed">
              Condições em uso:
            </Text>
            <Group gap={4}>
              {d.conds.map((c) => (
                <Badge key={c} size="xs" variant="light" color="gray" tt="none">
                  {c}
                </Badge>
              ))}
            </Group>
          </>
        )}
      </Stack>
    );
  }
  if (node.type === "eventType") {
    const d = node.data as unknown as EventTypeNodeData;
    return (
      <Stack gap="xs">
        <Text fw={600} size="sm">
          Event type
        </Text>
        <Badge variant="light" color="violet" tt="none" style={{ alignSelf: "flex-start" }}>
          {d.eventType}
        </Badge>
        {d.listeners === 0 ? (
          <>
            <Text size="xs" c="dimmed">
              Nenhum achievement deste jogo escuta este event type ainda — ele é emitido mas não
              alimenta nada (órfão).
            </Text>
            <Button
              variant="light"
              size="xs"
              leftSection={<IconTrophy size={14} />}
              onClick={() => onQuickAch(d.eventType)}
            >
              Criar achievement que escuta este type
            </Button>
          </>
        ) : (
          <Text size="xs" c="dimmed">
            {d.listeners} achievement{d.listeners === 1 ? "" : "s"} deste jogo escuta
            {d.listeners === 1 ? "" : "m"} este event type.
          </Text>
        )}
      </Stack>
    );
  }
  const d = node.data as unknown as AchievementNodeData;
  return (
    <Stack gap="xs">
      <Text fw={600} size="sm">
        Achievement
      </Text>
      <Text size="sm">{d.name}</Text>
      <Text size="xs" c="dimmed">
        {d.statKey ? (
          <>
            Progride pela stat <code>{d.statKey}</code> (
            {d.statMode === "per_match" ? "por partida" : "acumulada"}) — direto, sem regra
          </>
        ) : (
          <>
            Escuta <code>{d.eventType}</code>
          </>
        )}{" "}
        · target {d.target}. Gerenciado na aba Achievements.
      </Text>
    </Stack>
  );
}

// QuickAchievementDrawer: criação rápida de achievement POR EVENTO a partir
// de um type órfão do canvas — mesmos campos/validação do form da aba
// Achievements (id/name/description/target), eventType travado.
function QuickAchievementDrawer({
  gameId,
  eventType,
  onClose,
  onCreated,
}: {
  gameId: string;
  eventType: string | null;
  onClose: () => void;
  onCreated: () => void;
}) {
  const [id, setId] = useState("");
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [target, setTarget] = useState<number | string>(1);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (eventType) {
      setId("");
      setName("");
      setDescription("");
      setTarget(1);
      setError(null);
    }
  }, [eventType]);

  async function submit() {
    if (!eventType) return;
    if (!ID_RE.test(id.trim())) {
      setError("id minúsculo (ex.: first_win)");
      return;
    }
    if (!name.trim()) {
      setError("nome obrigatório");
      return;
    }
    setSaving(true);
    try {
      await api.createAchievement(gameId, {
        id: id.trim(),
        name: name.trim(),
        description: description.trim(),
        eventType,
        statKey: "",
        statMode: undefined,
        target: Number(target) || 1,
      });
      notifications.show({ color: "green", message: "Conquista adicionada." });
      onCreated();
    } catch (e) {
      setError(
        e instanceof HttpError && e.status === 409
          ? "Já existe uma conquista com esse id neste jogo."
          : e instanceof HttpError && e.status === 400
            ? "Dados inválidos."
            : "Falha ao adicionar (serviço indisponível?).",
      );
    } finally {
      setSaving(false);
    }
  }

  return (
    <Drawer
      opened={!!eventType}
      onClose={onClose}
      position="right"
      size={420}
      title={
        <>
          Novo achievement — <code>{eventType}</code>
        </>
      }
    >
      <Stack gap="sm">
        <Text size="xs" c="dimmed">
          Critério <b>por evento</b>: progride a cada ocorrência de <code>{eventType}</code>.
          Detalhes e edição ficam na aba Achievements.
        </Text>
        <TextInput label="ID" placeholder="raid_master" size="xs" value={id} onChange={(e) => setId(e.currentTarget.value)} />
        <TextInput label="Nome" placeholder="Mestre da Raid" size="xs" value={name} onChange={(e) => setName(e.currentTarget.value)} />
        <Textarea
          label="Descrição"
          size="xs"
          autosize
          minRows={2}
          value={description}
          onChange={(e) => setDescription(e.currentTarget.value)}
        />
        <NumberInput label="Target (ocorrências)" size="xs" min={1} value={target} onChange={setTarget} />
        {error && (
          <Text size="xs" c="red">
            {error}
          </Text>
        )}
        <Group justify="flex-end" gap="xs">
          <Button variant="default" size="xs" onClick={onClose}>
            Cancelar
          </Button>
          <Button size="xs" loading={saving} onClick={submit}>
            Criar
          </Button>
        </Group>
      </Stack>
    </Drawer>
  );
}

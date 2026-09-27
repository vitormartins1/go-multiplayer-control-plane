import {
  ActionIcon,
  Badge,
  Box,
  Button,
  Chip,
  Collapse,
  CloseButton,
  Drawer,
  Group,
  Modal,
  NumberInput,
  Paper,
  Popover,
  ScrollArea,
  Stack,
  Switch,
  Table,
  Text,
  Textarea,
  TextInput,
  Tooltip,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import {
  IconAdjustments,
  IconBan,
  IconChevronDown,
  IconChevronUp,
  IconHandClick,
  IconInfoCircle,
  IconPlus,
  IconRefresh,
  IconRotate2,
  IconSearch,
  IconTrash,
  IconTrophy,
} from "@tabler/icons-react";
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
import { matchesListFilter } from "../lib/eventRules";
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
  DRAFT_RULE_ID,
  draftHeight,
  pathHighlight,
  repackVisible,
  siblingRuleIds,
  STAT_PREFIX,
  type AchievementNodeData,
  type DraftGraphInput,
  type EventTypeNodeData,
  type RuleNodeData,
  type SourceKind,
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
  autoEditRuleId,
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
  // autoEditRuleId (spec 057): navegação vinda de Achievements pedindo edição
  // direta — só abre o form se a regra for do jogo (global é sempre read-only).
  autoEditRuleId?: string | null;
}) {
  const [archetype, setArchetype] = useState<MultiplayerProfile["archetype"]>(archetypeOptions[0]);
  const arch = archetypeOptions.includes(archetype) ? archetype : archetypeOptions[0];

  // listPanelWidth (achado do Vitor: o fit reservava a largura do dock
  // INTEIRO — lista + rail de ações — mas o rail nunca fica atrás de nenhum
  // node, só a lista tapa área de verdade). Medida só do painel da lista
  // (ver EventRulesDock), repassada pro FlowCanvas via dockReservedWidth.
  const [listPanelWidth, setListPanelWidth] = useState(0);
  const [listFilter, setListFilter] = useState("");
  // draftArchetype (spec 058): o seletor de arquétipo saiu da toolbar fixa —
  // só importa no momento de CRIAR uma regra num jogo com mais de um
  // arquétipo (edição nunca muda arquétipo, spec 016); inicializado a partir
  // do arquétipo ativo (arch) quando o form de criação abre.
  const [draftArchetype, setDraftArchetype] = useState<MultiplayerProfile["archetype"]>(archetypeOptions[0]);

  const [selectedId, setSelectedId] = useState<string | null>(null);
  // detailsCollapsed (painel de detalhe do node selecionado): recolhe pra só
  // o cabeçalho, sem perder a seleção — o conteúdo continua atualizando por
  // baixo (Collapse não desmonta). Clicar em outro node sempre reexpande
  // (achado do Vitor: recolher é só um "deixa enxuto por agora", não um modo
  // permanente).
  const [detailsCollapsed, setDetailsCollapsed] = useState(false);
  const [search, setSearch] = useState("");
  // relatedKinds (2026-07-10): "regras relacionadas" por kind de source
  // (facts/modo/stats) em vez de um switch único — vazio = desligado (achado
  // do Vitor: facts genéricos tipo outcome=SUCCESS sozinhos já geram muito
  // fan-out; agora dá pra restringir a só stats, por exemplo).
  const [relatedKinds, setRelatedKinds] = useState<Set<SourceKind>>(new Set());
  // statIgnoreValue (2026-07-10): stats são numéricos/em tiers — duas regras
  // no mesmo stat com limiares diferentes costumam ser relacionadas mesmo
  // sem o valor bater exato (diferente de facts/modo, categóricos). Controle
  // do usuário, só relevante com o chip "Stats" ligado.
  const [statIgnoreValue, setStatIgnoreValue] = useState(false);
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
    // autoEditRuleId (spec 057) entra na MESMA sincronização: no mount vindo
    // de Achievements, autoEditRuleId já chega preenchido (deriva direto da
    // prop pendingRuleFocus) enquanto focusRuleId ainda é null (é estado
    // próprio do GameEventRules, que só atualiza num efeito assíncrono
    // depois) — sem isso, a troca de arquétipo não disparava a tempo do
    // startEdit, e archRules ficava filtrado pro arquétipo errado.
    const targetRuleId = focusRuleId ?? autoEditRuleId ?? null;
    if (targetRuleId) {
      const rule = rules.find((r) => r.id === targetRuleId);
      if (rule && rule.archetype !== archetype) setArchetype(rule.archetype);
    }
  }, [focusRuleId, autoEditRuleId, rules, archetype]);

  // effectiveArch (achado do Vitor testando o picker de arquétipo do form
  // de criação): durante CRIAÇÃO num jogo com mais de um arquétipo, o
  // vocabulário de sugestões (key/value/mode/event type) tem que refletir o
  // arquétipo ESCOLHIDO no form (draftArchetype) — não o arquétipo ativo do
  // canvas (arch). São coisas diferentes: `arch` é "o que o canvas está
  // mostrando agora"; `draftArchetype` é "em que arquétipo a regra nova vai
  // nascer". Sem isso, o autocomplete (e até os nós de fonte clicáveis pra
  // preencher condição) continuavam do arquétipo antigo mesmo depois de
  // trocar no picker. Fora de criação (ou jogo de 1 arquétipo só), os dois
  // coincidem — edição nunca muda arquétipo (spec 016), então nesse caso
  // `arch` já está correto.
  const effectiveArch =
    draft && !draft.editingRuleID && archetypeOptions.length > 1 ? draftArchetype : arch;

  // archRules inclui a regra em edição mesmo se `arch` ainda não convergiu
  // pro arquétipo dela (defensivo contra qualquer race remanescente entre o
  // efeito de sincronização acima e o startEdit disparado por autoEditRuleId
  // — sem isso, buildRulesGraph recebia draft.editingRuleID apontando pra
  // uma regra ausente do conjunto filtrado e quebrava, achado testando a
  // navegação "Editar regra" vinda de Achievements).
  const archRules = useMemo(
    () => rules.filter((r) => r.archetype === effectiveArch || r.id === draft?.editingRuleID),
    [rules, effectiveArch, draft],
  );

  // Stats do arquétipo (spec 045): declaradas + drift (usada por achievement
  // sem declaração em NENHUM modo).
  const statsForArch = useMemo<StatInfo[]>(() => {
    const declaredHere = new Set(modes.filter((m) => m.archetype === effectiveArch).flatMap(modeStats));
    const declaredAnywhere = new Set(modes.flatMap(modeStats));
    const out: StatInfo[] = [...declaredHere].sort().map((key) => ({ key, declared: true }));
    const drift = new Set(
      catalog.map((a) => a.statKey).filter((k): k is string => !!k && !declaredAnywhere.has(k)),
    );
    for (const key of [...drift].sort()) out.push({ key, declared: false });
    return out;
  }, [modes, effectiveArch, catalog]);

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
    () => modes.filter((m) => m.archetype === effectiveArch).map((m) => m.modeId),
    [modes, effectiveArch],
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
    () => buildRulesGraph(archRules, catalog, { stats: statsForArch, draft: draftInput }),
    [archRules, catalog, statsForArch, draftInput],
  );

  // effectiveFocusId (spec 056): durante edição, foca na regra sendo editada
  // (pode ser uma "vizinha" trazida por showRelated, não a focusRuleId
  // original) — durante criação pura (editingRuleID null) não há foco por
  // regra (o canvas usa o ramo dinâmico de sources, mais abaixo).
  const effectiveFocusId = draft ? draft.editingRuleID : focusRuleId;

  // focusPath (spec 055, estendido pela 056): subgrafo da regra focada —
  // agora também ativo durante edição (effectiveFocusId), só fica null
  // mesmo durante criação pura.
  const focusPath = useMemo(() => {
    if (!effectiveFocusId) return null;
    const ruleNodeId = `r:${effectiveFocusId}`;
    const base = pathHighlight(graph.edges, ruleNodeId);
    if (relatedKinds.size === 0) return base;
    const nodeIds = new Set(base.nodeIds);
    const edgeIds = new Set(base.edgeIds);
    for (const siblingId of siblingRuleIds(graph.nodes, graph.edges, ruleNodeId, relatedKinds, statIgnoreValue)) {
      const p = pathHighlight(graph.edges, siblingId);
      p.nodeIds.forEach((id) => nodeIds.add(id));
      p.edgeIds.forEach((id) => edgeIds.add(id));
    }
    return { nodeIds, edgeIds };
  }, [graph, effectiveFocusId, relatedKinds, statIgnoreValue]);

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

  // startCreate abre o form de criação in-canvas (spec 048/058) — parte do
  // arquétipo atualmente ativo (arch); se o jogo tiver mais de um, o
  // draftArchetype fica editável no próprio form (archetypePicker).
  function startCreate() {
    setSelectedId(null);
    setDraftArchetype(arch);
    setDraft({ editingRuleID: null, rows: [EMPTY_ROW], linked: [], news: newGhost(), error: null, saving: false });
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
      // sem ghost de tipo novo ao editar (achado do Vitor testando: abria uma
      // caixinha "event type novo" vazia sem o usuário ter pedido) — editar
      // mexe só na regra por enquanto; linkar/desvincular tipos existentes
      // continua via clique no node (event types visíveis no foco da regra).
      news: [],
      error: null,
      saving: false,
    });
  }

  // autoEditRuleId (spec 057): navegação vinda de Achievements — dispara o
  // form de edição sozinho quando a regra pedida é do jogo (chega já com
  // focusRuleId setado igual, então o subgrafo focado já está correto).
  useEffect(() => {
    if (!autoEditRuleId) return;
    const rule = rules.find((r) => r.id === autoEditRuleId && r.scope === "game");
    if (rule) startEdit(rule);
  }, [autoEditRuleId, rules]);

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
        notifications.show({ color: "nominal", message: "Regra atualizada." });
      } else {
        // Sem seletor manual na toolbar (spec 058): jogo de 1 arquétipo só
        // usa `arch` direto; com mais de um, draftArchetype (editável no
        // próprio form, archetypePicker) manda.
        const archetype = archetypeOptions.length > 1 ? draftArchetype : arch;
        await api.createEventRule(gameId, { archetype, match, eventTypes });
        notifications.show({ color: "nominal", message: "Regra adicionada." });
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
    // grafo completo só sobra como fallback (não deveria ocorrer na prática —
    // ver comentário do último ramo). Foco (normal ou edição) corta pro
    // subgrafo via applyFocusFilter; criação pura tem seu próprio filtro
    // dinâmico (spec 056); sem foco e sem draft, vazio.
    let visibleNodes = graph.nodes;
    let visibleEdges = graph.edges;

    // draftHeightOverrides (spec 056 follow-up): a altura real do node
    // ruleDraft (draftHeight(rows), a mesma fórmula do build) — repackVisible
    // não tem como calculá-la sozinho porque, neste ponto, o node ainda tem
    // `data: {}` (o draftData só é injetado mais abaixo). Sem isso, a coluna
    // vizinha (event type novo, sources) não acompanha o card crescendo ou
    // encolhendo conforme o usuário adiciona/remove condições de match — o
    // objetivo é ela ficar sempre alinhada ao meio da caixinha da regra.
    const draftHeightOverrides = draft
      ? new Map([[draft.editingRuleID ? `r:${draft.editingRuleID}` : DRAFT_RULE_ID, draftHeight(draft.rows.length)]])
      : undefined;

    // applyFocusFilter: nodes/edges alcançáveis por path (spec 055), com
    // repack — sectionLabel só sobrevive se alguma source do grupo dela
    // estiver visível (evita cabeçalho de seção órfão). Reaproveitado tanto
    // pelo foco "normal" (lista) quanto pela edição (spec 056).
    const applyFocusFilter = (path: { nodeIds: Set<string>; edgeIds: Set<string> }) => {
      const filtered = graph.nodes.filter((n) => {
        if (path.nodeIds.has(n.id)) return true;
        if (n.type === "sectionLabel" && n.id.startsWith("lbl:")) {
          const kind = n.id.slice(4);
          return graph.nodes.some(
            (s) =>
              s.type === "source" &&
              (s.data as unknown as SourceNodeData).kind === kind &&
              path.nodeIds.has(s.id),
          );
        }
        return false;
      });
      // repackVisible: sem isso, os nodes filtrados mantêm as coordenadas Y
      // calculadas pro arquétipo inteiro — vãos enormes onde ficavam os nodes
      // escondidos (achado do Vitor testando o foco na prática).
      return {
        nodes: repackVisible(filtered, draftHeightOverrides),
        edges: graph.edges.filter((e) => path.edgeIds.has(e.id)),
      };
    };

    if (draft && !draft.editingRuleID) {
      // Criação pura (spec 056): só a caixinha da regra + o(s) ghost(s) de
      // tipo + as sources cujas chaves já foram digitadas/validadas numa
      // linha de match — linkar um tipo EXISTENTE continua funcionando via
      // autocomplete no próprio ghost (typeSuggestions), sem precisar do nó
      // dele aparecer (decisão do Vitor).
      const referencedKeys = new Set(draft.rows.map((r) => r.key.trim()).filter(Boolean));
      const filtered = graph.nodes.filter((n) => {
        if (n.id === DRAFT_RULE_ID || n.id.startsWith("t:__new__:")) return true;
        if (n.type === "source") return referencedKeys.has((n.data as unknown as SourceNodeData).sourceKey);
        return false;
      });
      // repackVisible (achado do Vitor testando na prática): sem isso as
      // sources que vão aparecendo mantêm a posição Y calculada pra coluna
      // inteira, em vez de se organizarem só com base no que está na tela —
      // mesmo raciocínio do follow-up da spec 055.
      visibleNodes = repackVisible(filtered, draftHeightOverrides);
      const visibleIds = new Set(filtered.map((n) => n.id));
      visibleEdges = graph.edges.filter((e) => visibleIds.has(e.source) && visibleIds.has(e.target));
    } else if (focusPath) {
      const r = applyFocusFilter(focusPath);
      visibleNodes = r.nodes;
      visibleEdges = r.edges;
    } else if (!draft) {
      visibleNodes = [];
      visibleEdges = [];
    }
    // draft && draft.editingRuleID && !focusPath: não deveria ocorrer
    // (effectiveFocusId sempre setado quando editingRuleID existe) — cai no
    // fallback seguro (grafo inteiro) declarado acima.

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
          archetypePicker:
            !draft.editingRuleID && archetypeOptions.length > 1
              ? {
                  value: draftArchetype,
                  options: archetypeOptions.map((a) => ({ value: a, label: ARCHETYPE_META[a].label })),
                  // trocar de arquétipo limpa o form pro estado inicial
                  // (achado do Vitor: o que já estava preenchido — chave,
                  // valor, event types ligados — provavelmente se referia
                  // ao arquétipo anterior; vocabulário muda junto, então o
                  // conteúdo antigo deixaria de fazer sentido). Obriga a
                  // pessoa a recadastrar do zero no arquétipo novo, em vez
                  // de manter condição/type potencialmente inválido.
                  onChange: (v) => {
                    setDraftArchetype(v as MultiplayerProfile["archetype"]);
                    setDraft((d) =>
                      d ? { ...d, rows: [EMPTY_ROW], linked: [], news: newGhost(), error: null } : d,
                    );
                  },
                }
              : undefined,
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
  }, [
    graph,
    selectedId,
    draft,
    search,
    keySuggestions,
    knownValues,
    modeIdSuggestions,
    typeSuggestions,
    focusPath,
    draftArchetype,
    archetypeOptions,
  ]);

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
      setDetailsCollapsed(false);
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
      notifications.show({ color: "nominal", message: "Regra removida." });
      setDeleting(null);
      setSelectedId(null);
      reload();
    } catch {
      notifications.show({ color: "critical", message: "Falha ao remover (serviço indisponível?)." });
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
        notifications.show({ color: "nominal", message: "Regra restaurada." });
      } else {
        await api.suppressEventRule(gameId, rule.id);
        notifications.show({ color: "orange", message: "Regra suprimida para este jogo." });
      }
      reload();
    } catch {
      notifications.show({ color: "critical", message: "Falha ao atualizar a supressão (serviço indisponível?)." });
    }
  }

  return (
    <Stack gap="xs">
      <FlowCanvas
        nodes={nodes}
        edges={edges}
        nodeTypes={NODE_TYPES}
        onNodeClick={onNodeClick}
        onPaneClick={() => setSelectedId(null)}
        fitKey={`${arch}|${focusRuleId ?? "none"}|${[...relatedKinds].sort().join(",")}|${statIgnoreValue}|${!!draft}|${nodes.length}`}
        height="calc(100vh - 224px)"
        dockReservedWidth={listPanelWidth}
        // emptyState (achado, revisão Event Rules): antes o canvas ficava
        // em branco (spec 055 é deliberada — sem regra focada, sem grafo),
        // mas sem nenhuma pista pro usuário. Só aparece quando nodes.length
        // é 0 de verdade (FlowCanvas já filtra isso). visibleFrom="sm": em
        // mobile o dock (lista de regras) já ocupa a largura toda — este
        // canvas é desktop-first por natureza (grafo com pan/zoom/drag),
        // sem isso a dica ficava por cima da lista.
        emptyState={
          <Box visibleFrom="sm">
            <Stack align="center" gap={6}>
              <IconHandClick size={28} color="var(--mantine-color-dimmed)" />
              <Text size="sm" c="dimmed">
                Selecione uma regra na lista à esquerda pra ver o grafo
              </Text>
            </Stack>
          </Box>
        }
        dock={
          <EventRulesDock
            rules={rules}
            focusRuleId={focusRuleId}
            onFocusRule={onFocusRule}
            listFilter={listFilter}
            onListFilter={setListFilter}
            draft={draft}
            onNewRule={startCreate}
            relatedKinds={relatedKinds}
            onRelatedKinds={setRelatedKinds}
            statIgnoreValue={statIgnoreValue}
            onStatIgnoreValue={setStatIgnoreValue}
            search={search}
            onSearch={setSearch}
            onDelete={(r) => setDeleting(r)}
            onToggleSuppress={toggleSuppress}
            onReload={reload}
            onListPanelWidthChange={setListPanelWidth}
          />
        }
        overlay={
          selectedNode && !draft ? (
            <Paper withBorder p="sm" w={290} shadow="md" style={{ background: "rgba(18, 24, 33, 0.95)" }}>
              <Group justify="space-between" wrap="nowrap" align="flex-start" gap={4} mb={detailsCollapsed ? 0 : "xs"}>
                <Group gap={6} wrap="nowrap" align="baseline" style={{ minWidth: 0 }}>
                  <Text size="xs" fw={700} c="dimmed" tt="uppercase" style={{ flexShrink: 0 }}>
                    {detailsHeaderSummary(selectedNode).kind}
                  </Text>
                  {/* sem truncate (spec 058, achado do Vitor): a identidade completa
                      só aparece aqui — o corpo não repete mais o tipo/identidade, então
                      cortar o nome aqui perderia informação sem ela sobrar em outro lugar. */}
                  <Text size="xs" c="dimmed" style={{ minWidth: 0, wordBreak: "break-word" }}>
                    {detailsHeaderSummary(selectedNode).label}
                  </Text>
                </Group>
                <Group gap={2} wrap="nowrap" style={{ flexShrink: 0 }}>
                  <ActionIcon
                    size="sm"
                    variant="subtle"
                    color="gray"
                    onClick={() => setDetailsCollapsed((c) => !c)}
                    aria-label={detailsCollapsed ? "Expandir detalhe" : "Recolher detalhe"}
                  >
                    {detailsCollapsed ? <IconChevronDown size={14} /> : <IconChevronUp size={14} />}
                  </ActionIcon>
                  <CloseButton size="sm" onClick={() => setSelectedId(null)} aria-label="Fechar detalhe" />
                </Group>
              </Group>
              <Collapse in={!detailsCollapsed}>
                <NodeDetails
                  node={selectedNode}
                  unconditionalTypes={unconditionalTypes}
                  onDelete={(r) => setDeleting(r)}
                  onToggleSuppress={toggleSuppress}
                  onQuickAch={(et) => setQuickAchType(et)}
                />
              </Collapse>
            </Paper>
          ) : undefined
        }
      />

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
            <Button color="critical" onClick={confirmDelete}>
              Remover
            </Button>
          </Group>
        </Stack>
      </Modal>
    </Stack>
  );
}

// EventRulesToolbarRail (spec 058, achado do Vitor: queria os controles ao
// lado DIREITO da lista, em fileira horizontal — não mais empilhados à
// esquerda dela): rail horizontal compacto de ícones. "Nova regra" e
// "filtrar/buscar" abrem um popover (pra baixo, já que os ícones estão lado
// a lado — abrir pra direita sobreporia o próximo ícone da fileira);
// "regras relacionadas" é um TOGGLE que expande INLINE pra direita (achado
// do Vitor: queria os chips/switch de relacionadas direto na fileira, não
// num balãozinho flutuante) — mesmos dados/callbacks de antes, só
// reembalados pro contexto flutuante (o Vitor liberou usar layout de
// botão/input diferente aqui).
function EventRulesToolbarRail({
  focusRuleId,
  draft,
  onNewRule,
  relatedKinds,
  onRelatedKinds,
  statIgnoreValue,
  onStatIgnoreValue,
  search,
  onSearch,
}: {
  focusRuleId: string | null;
  draft: DraftState | null;
  onNewRule: () => void;
  relatedKinds: Set<SourceKind>;
  onRelatedKinds: (kinds: Set<SourceKind>) => void;
  statIgnoreValue: boolean;
  onStatIgnoreValue: (v: boolean) => void;
  search: string;
  onSearch: (v: string) => void;
}) {
  const [relatedOpen, setRelatedOpen] = useState(false);
  const [searchOpen, setSearchOpen] = useState(false);
  return (
    <Paper withBorder shadow="md" p={4} style={{ background: "rgba(18, 24, 33, 0.95)", pointerEvents: "auto" }}>
      <Group gap={4} wrap="nowrap">
        <Tooltip label={draft ? "Salve ou cancele o rascunho atual" : "Nova regra"} position="bottom" withinPortal>
          <ActionIcon size="md" variant="filled" disabled={!!draft} onClick={onNewRule} aria-label="Nova regra">
            <IconPlus size={16} />
          </ActionIcon>
        </Tooltip>
        <Tooltip label="Buscar no subgrafo" position="bottom" withinPortal>
          <ActionIcon
            size="md"
            variant={searchOpen || search ? "light" : "subtle"}
            color={searchOpen || search ? "blue" : "gray"}
            onClick={() => setSearchOpen((o) => !o)}
            aria-label="Buscar no subgrafo"
            aria-expanded={searchOpen}
          >
            <IconSearch size={16} />
          </ActionIcon>
        </Tooltip>
        {searchOpen && (
          <TextInput
            size="xs"
            placeholder="destaca no canvas…"
            value={search}
            onChange={(e) => onSearch(e.currentTarget.value)}
            rightSection={search ? <CloseButton size="xs" onClick={() => onSearch("")} /> : undefined}
            style={{ width: 200 }}
          />
        )}
        <Tooltip label="Regras relacionadas" position="bottom" withinPortal>
          <ActionIcon
            size="md"
            variant={relatedOpen || relatedKinds.size > 0 ? "light" : "subtle"}
            color={relatedOpen || relatedKinds.size > 0 ? "blue" : "gray"}
            onClick={() => setRelatedOpen((o) => !o)}
            aria-label="Regras relacionadas"
            aria-expanded={relatedOpen}
          >
            <IconAdjustments size={16} />
          </ActionIcon>
        </Tooltip>
        {relatedOpen && (
          <Group gap={6} wrap="nowrap" pl={6} style={{ borderLeft: "1px solid #2a3340" }}>
            <Chip.Group multiple value={[...relatedKinds]} onChange={(v) => onRelatedKinds(new Set(v as SourceKind[]))}>
              <Group gap={4} wrap="nowrap">
                <Chip size="xs" value="structural" disabled={!focusRuleId}>
                  Facts
                </Chip>
                <Chip size="xs" value="mode" disabled={!focusRuleId}>
                  Modo
                </Chip>
                <Chip size="xs" value="stat" disabled={!focusRuleId}>
                  Stats
                </Chip>
              </Group>
            </Chip.Group>
            {relatedKinds.has("stat") && (
              <Switch
                size="xs"
                label="qualquer limiar"
                checked={statIgnoreValue}
                onChange={(e) => onStatIgnoreValue(e.currentTarget.checked)}
              />
            )}
            {!focusRuleId && (
              <Text size="xs" c="dimmed" style={{ whiteSpace: "nowrap" }}>
                selecione uma regra
              </Text>
            )}
          </Group>
        )}
      </Group>
    </Paper>
  );
}

// EventRulesDock (spec 058): painel flutuando sobre o canvas — rail de
// controles (acima) + lista de regras, lado a lado, ambos flutuando juntos
// no canto do canvas. Antes disso, lista/toolbar/texto eram chrome FIXO em
// GameEventRules.tsx, disputando espaço permanente com o canvas — a mudança
// migrou tudo isso pra dentro do canvas, deixando a aba sem scroll de
// página (o scroll que sobra é só o interno da lista, dentro do painel).
function EventRulesDock({
  rules,
  focusRuleId,
  onFocusRule,
  listFilter,
  onListFilter,
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
  onReload,
  onListPanelWidthChange,
}: {
  rules: EventRule[];
  focusRuleId: string | null;
  onFocusRule: (ruleId: string | null) => void;
  listFilter: string;
  onListFilter: (v: string) => void;
  draft: DraftState | null;
  onNewRule: () => void;
  relatedKinds: Set<SourceKind>;
  onRelatedKinds: (kinds: Set<SourceKind>) => void;
  statIgnoreValue: boolean;
  onStatIgnoreValue: (v: boolean) => void;
  search: string;
  onSearch: (v: string) => void;
  onDelete: (r: EventRule) => void;
  onToggleSuppress: (r: EventRule) => void;
  onReload: () => void;
  // onListPanelWidthChange (achado do Vitor): só a largura da LISTA conta
  // pra área que o fitView reserva — o rail de ações (ao lado) nunca fica
  // atrás de node nenhum, reservar a largura dele também só empurrava o
  // grafo mais longe do que precisava.
  onListPanelWidthChange: (w: number) => void;
}) {
  const filtered = rules.filter(matchesListFilter(listFilter));
  const listRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = listRef.current;
    if (!el) return;
    const ro = new ResizeObserver((entries) => onListPanelWidthChange(entries[0].contentRect.width));
    ro.observe(el);
    return () => ro.disconnect();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  return (
    // pointerEvents "none" no container + "auto" nos dois cards (spec 058,
    // achado do Vitor: às vezes não dava pra clicar em alguns facts/regras)
    // — o Panel do React Flow não põe pointer-events:none por padrão, e o
    // Group (flex row) tem um retângulo delimitador tão alto quanto o maior
    // filho (a lista); a faixa "vazia" ao lado do rail curto, que parece
    // canvas livre, na verdade fazia parte desse retângulo e bloqueava
    // clique nos nodes por baixo. Só os cards de verdade (lista/rail)
    // voltam a aceitar clique.
    <Group align="flex-start" gap={8} wrap="nowrap" style={{ pointerEvents: "none" }}>
      <Paper
        ref={listRef}
        withBorder
        shadow="md"
        w={320}
        style={{
          background: "rgba(18, 24, 33, 0.95)",
          maxHeight: "calc(100vh - 190px)",
          display: "flex",
          flexDirection: "column",
          pointerEvents: "auto",
        }}
      >
        <Group justify="space-between" wrap="nowrap" align="center" p="xs">
          <Group gap={6} wrap="nowrap" align="center" style={{ flex: 1, minWidth: 0 }}>
            <TextInput
              size="xs"
              placeholder="filtrar…"
              value={listFilter}
              onChange={(e) => onListFilter(e.currentTarget.value)}
              rightSection={listFilter ? <CloseButton size="xs" onClick={() => onListFilter("")} /> : undefined}
              style={{ flex: 1, minWidth: 0 }}
            />
            <Badge size="xs" variant="light" color="gray" style={{ flexShrink: 0 }}>
              {filtered.length}
            </Badge>
            <Popover width={300} withArrow position="bottom-start" withinPortal>
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
                  aqui); regras do <b>jogo</b> só valem pra este jogo. Selecione uma regra pra ver o subgrafo
                  dela no canvas.
                </Text>
              </Popover.Dropdown>
            </Popover>
          </Group>
          <ActionIcon size="xs" variant="subtle" color="gray" onClick={onReload} aria-label="Atualizar" style={{ flexShrink: 0 }}>
            <IconRefresh size={14} />
          </ActionIcon>
        </Group>
        <ScrollArea style={{ flex: 1, minHeight: 0 }}>
          <Table highlightOnHover fz="xs">
            <Table.Tbody>
              {filtered.map((r, i) => {
                const ArchIcon = ARCHETYPE_META[r.archetype].icon;
                return (
                <Table.Tr
                  key={r.id}
                  onClick={() => onFocusRule(focusRuleId === r.id ? null : r.id)}
                  style={{
                    cursor: "pointer",
                    opacity: r.suppressed ? 0.5 : undefined,
                    background:
                      r.id === focusRuleId
                        ? "var(--mantine-color-blue-light)"
                        : i % 2 === 1
                          ? "rgba(255, 255, 255, 0.045)"
                          : undefined,
                  }}
                >
                  <Table.Td style={{ whiteSpace: "nowrap" }}>
                    <Group gap={4} wrap="nowrap">
                      <Tooltip label={ARCHETYPE_META[r.archetype].label} withArrow position="top-start" openDelay={300}>
                        <ArchIcon size={13} />
                      </Tooltip>
                      <Badge size="xs" variant="outline" color={r.scope === "global" ? "gray" : "teal"}>
                        {r.scope === "global" ? "global" : "jogo"}
                      </Badge>
                    </Group>
                  </Table.Td>
                  <Table.Td>
                    <Group gap={3} wrap="wrap">
                      {r.eventTypes.map((et) => (
                        <Badge key={et} size="xs" variant="light" color="gray">
                          {et}
                        </Badge>
                      ))}
                    </Group>
                  </Table.Td>
                  <Table.Td onClick={(e) => e.stopPropagation()} w={28}>
                    {r.scope === "game" ? (
                      <ActionIcon size="xs" variant="subtle" color="critical" onClick={() => onDelete(r)} aria-label="Remover">
                        <IconTrash size={13} />
                      </ActionIcon>
                    ) : (
                      <ActionIcon
                        size="xs"
                        variant="subtle"
                        color={r.suppressed ? "teal" : "orange"}
                        onClick={() => onToggleSuppress(r)}
                        aria-label={r.suppressed ? "Restaurar" : "Suprimir"}
                      >
                        {r.suppressed ? <IconRotate2 size={13} /> : <IconBan size={13} />}
                      </ActionIcon>
                    )}
                  </Table.Td>
                </Table.Tr>
                );
              })}
            </Table.Tbody>
          </Table>
        </ScrollArea>
      </Paper>
      <EventRulesToolbarRail
        focusRuleId={focusRuleId}
        draft={draft}
        onNewRule={onNewRule}
        relatedKinds={relatedKinds}
        onRelatedKinds={onRelatedKinds}
        statIgnoreValue={statIgnoreValue}
        onStatIgnoreValue={onStatIgnoreValue}
        search={search}
        onSearch={onSearch}
      />
    </Group>
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

// detailsHeaderSummary: rótulo curto do node selecionado, mostrado no
// cabeçalho do painel de detalhe mesmo quando ele está recolhido (spec do
// Vitor: o painel some quase inteiro, mas o cabeçalho continua dizendo o
// quê está selecionado).
function detailsHeaderSummary(n: Node): { kind: string; label: string } {
  switch (n.type) {
    case "rule": {
      const { rule } = n.data as unknown as RuleNodeData;
      return { kind: "Regra", label: rule.scope === "global" ? "global" : "do jogo" };
    }
    case "source": {
      const d = n.data as unknown as SourceNodeData;
      return { kind: d.kind === "stat" ? "Stat" : d.kind === "mode" ? "Modo" : "Fact", label: d.sourceKey };
    }
    case "eventType": {
      const d = n.data as unknown as EventTypeNodeData;
      return { kind: "Event type", label: d.eventType };
    }
    case "achievement": {
      const d = n.data as unknown as AchievementNodeData;
      return { kind: "Achievement", label: d.name };
    }
    default:
      return { kind: "Detalhe", label: "" };
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
        {/* sem repetir "Regra" + escopo aqui — já está no cabeçalho sempre
            visível do painel (spec 058, achado do Vitor: corpo repetia o
            que o cabeçalho já mostrava, pra todos os tipos de node). */}
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
          <Button variant="light" color="critical" size="xs" leftSection={<IconTrash size={14} />} onClick={() => onDelete(rule)}>
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
        {d.drift && (
          <Text size="xs" c="critical">
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
      <Text size="xs" c="dimmed">
        {d.statKey ? (
          <>
            Progride pela stat <code>{d.statKey}</code> (
            {d.statMode === "per_match" ? "por partida" : "acumulada"}) — direto, sem regra.
          </>
        ) : (
          <>
            Escuta <code>{d.eventType}</code>.
          </>
        )}
      </Text>
      <Badge size="xs" variant="light" color="yellow" tt="none" style={{ alignSelf: "flex-start" }}>
        meta: {d.target}
      </Badge>
      <Text size="xs" c="dimmed" fs="italic">
        Gerenciado na aba Achievements.
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
      notifications.show({ color: "nominal", message: "Conquista adicionada." });
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
          <Text size="xs" c="critical">
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

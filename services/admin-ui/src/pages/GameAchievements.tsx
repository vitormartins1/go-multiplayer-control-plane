import {
  ActionIcon,
  Alert,
  Autocomplete,
  Badge,
  Box,
  Button,
  Collapse,
  Drawer,
  Group,
  List,
  Loader,
  Modal,
  NumberInput,
  SegmentedControl,
  Select,
  Stack,
  Table,
  Text,
  Textarea,
  TextInput,
  Tooltip,
} from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import { useForm, type UseFormReturnType } from "@mantine/form";
import { notifications } from "@mantine/notifications";
import {
  IconAlertTriangle,
  IconChartBar,
  IconDownload,
  IconPencil,
  IconPlus,
  IconRefresh,
  IconTrash,
  IconUpload,
  IconX,
} from "@tabler/icons-react";
import type { NodeTypes } from "@xyflow/react";
import { Fragment, useMemo, useRef, useState, type ChangeEvent, type ReactNode } from "react";
import {
  api,
  HttpError,
  type AchievementProgressEntry,
  type CatalogAchievement,
  type EventRule,
  type ImportResponse,
  type MultiplayerMode,
  type NewAchievement,
  type StatMode,
} from "../lib/api";
import { CSV_TEMPLATE, parseCSV, type ParseError } from "../lib/csv";
import { gameArchetypes, filterRulesForGame } from "../lib/eventRules";
import { useAsync } from "../lib/useAsync";
import { AchievementIcon } from "../components/AchievementIcon";
import { PlayerAvatar } from "../components/PlayerAvatar";
import { GuestBadge } from "../components/GuestBadge";
import { FlowCanvas } from "../components/flow/FlowCanvas";
import { AchievementNode, EventTypeNode, RuleNode, SectionLabelNode, SourceNode } from "../components/flow/nodes";
import {
  buildRulesGraph,
  pathHighlight,
  repackVisible,
  srcId,
  STAT_PREFIX,
  type SourceNodeData,
  type StatInfo,
  typeId,
} from "../lib/rulesGraph";

const ID_RE = /^[a-z0-9]([a-z0-9_-]*[a-z0-9])?$/;
const STAT_RE = /^[a-z][a-z0-9_]*$/;

// Sugestões de um campo: os valores (data do Autocomplete) + a origem de cada
// um, mostrada esmaecida na opção (ex.: kills → "modo tdm").
type Suggestions = { values: string[]; origin: Map<string, string> };

const NO_SUGGESTIONS: Suggestions = { values: [], origin: new Map() };

// statSuggestions = união dos scoringStats dos modos do jogo + o intrínseco
// score. Origem = os modos que declaram o stat.
function statSuggestions(modes: MultiplayerMode[]): Suggestions {
  const declaredBy = new Map<string, string[]>();
  for (const m of modes) {
    // defensivo: respostas antigas do registry omitiam o array vazio (omitempty)
    for (const s of m.params.scoringStats ?? []) {
      declaredBy.set(s, [...(declaredBy.get(s) ?? []), m.modeId]);
    }
  }
  const origin = new Map<string, string>([["score", "intrínseco (todo resultado)"]]);
  for (const [s, ids] of declaredBy) {
    origin.set(s, `modo${ids.length > 1 ? "s" : ""} ${ids.join(", ")}`);
  }
  return { values: [...origin.keys()], origin };
}

// eventSuggestions = união dos eventTypes das regras EFETIVAS do jogo
// (migration 008: globais do arquétipo + overrides do jogo) — substitui o
// antigo dicionário hardcoded EVENT_VOCABULARY, que precisava ser atualizado
// manualmente toda vez que um sink Go ganhava um event type novo.
function eventSuggestions(rules: EventRule[]): Suggestions {
  const origin = new Map<string, string>();
  for (const r of rules) {
    for (const ev of r.eventTypes) if (!origin.has(ev)) origin.set(ev, r.archetype);
  }
  return { values: [...origin.keys()], origin };
}

// DriftBadge é o aviso de consistência (029): critério apontando para algo que
// o cadastro de modos não conhece. Detecta drift, não impede — read-only,
// provavelmente typo/rename, mas pode ser intencional (envelope aberto).
// Só o ícone cru (sem Badge em volta — ficava "triângulo dentro de bolinha",
// visualmente estranho) pra não competir por espaço horizontal na coluna de
// critério — o label vira aria-label; o detalhe some no tooltip, como já era.
function DriftBadge({ label, tooltip }: { label: string; tooltip: string }) {
  return (
    <Tooltip label={tooltip} multiline w={280} withArrow>
      <IconAlertTriangle
        size={14}
        color="var(--mantine-color-yellow-5)"
        aria-label={label}
        style={{ cursor: "default", flexShrink: 0 }}
      />
    </Tooltip>
  );
}

// MINI_NODE_TYPES (spec 057): só os 5 tipos "estáveis" — a prévia nunca
// mostra rascunho/ghost (não edita nada ali).
const MINI_NODE_TYPES: NodeTypes = {
  sectionLabel: SectionLabelNode,
  source: SourceNode,
  rule: RuleNode,
  eventType: EventTypeNode,
  achievement: AchievementNode,
};

// RuleFocusMiniCanvas (spec 057): prévia read-only do subgrafo de um event
// type ou achievement — mesmo pathHighlight/repackVisible do canvas de Event
// Rules (055/056), sem toolbar e sem nenhuma interação (nodes forçados
// selectable/draggable false). `rules` chega SEM filtro de arquétipo — o
// achievement não sabe o arquétipo, e quem emite o event type é quem importa.
function RuleFocusMiniCanvas({
  rules,
  catalog,
  focusNodeId,
  overlay,
}: {
  rules: EventRule[];
  catalog: CatalogAchievement[];
  focusNodeId: string;
  // overlay flutua no canto do mini-canvas (Panel top-right do FlowCanvas) —
  // mesmo padrão do canvas de Event Rules, em vez de um Group abaixo do
  // canvas ocupando espaço vertical à toa.
  overlay?: ReactNode;
}) {
  // buildRulesGraph só cria nós de achievement por MÉTRICA (metricAchs) quando
  // recebe `opts.stats` — sem isso, um achievement por stat nem aparece no
  // grafo (achado testando: prévia vinha totalmente vazia, nem o próprio
  // achievement). Não precisamos da distinção declarado/drift aqui (isso já
  // aparece na própria linha da tabela) — `declared: true` sempre, só pra
  // garantir que a source e a edge existam.
  const statsInput = useMemo<StatInfo[]>(
    () =>
      [...new Set(catalog.map((a) => a.statKey).filter((k): k is string => !!k))].map((key) => ({
        key,
        declared: true,
      })),
    [catalog],
  );
  const graph = useMemo(() => buildRulesGraph(rules, catalog, { stats: statsInput }), [rules, catalog, statsInput]);
  const { nodes, edges } = useMemo(() => {
    // Foco numa fonte de STAT: restringe o caminho às edges de MÉTRICA
    // (fonte → achievement direto) — uma regra que usa o mesmo stat como
    // condição de match não tem relação com ESTE achievement específico
    // (achado do Vitor: a prévia de "deaths" também trazia a regra
    // flawless_invasion, que só coincide na chave, sem alimentar o
    // achievement). Sem essa restrição, pathHighlight segue TODAS as edges
    // que saem da fonte, inclusive as de condição de regra.
    const focusNode = graph.nodes.find((n) => n.id === focusNodeId);
    const isStatSource = focusNode?.type === "source" && (focusNode.data as SourceNodeData).kind === "stat";
    const edgesForPath = isStatSource
      ? graph.edges.filter((e) => !(e.source === focusNodeId && e.className !== "is-metric-edge"))
      : graph.edges;
    const path = pathHighlight(edgesForPath, focusNodeId);
    const filtered = graph.nodes.filter((n) => path.nodeIds.has(n.id));
    const repacked = repackVisible(filtered).map((n) => ({ ...n, selectable: false, draggable: false }));
    return { nodes: repacked, edges: graph.edges.filter((e) => path.edgeIds.has(e.id)) };
  }, [graph, focusNodeId]);

  return (
    <FlowCanvas nodes={nodes} edges={edges} nodeTypes={MINI_NODE_TYPES} height={280} fitKey={focusNodeId} overlay={overlay} />
  );
}

// SuggestInput é o Autocomplete com origem esmaecida por opção — texto fora da
// lista continua aceito (creatable por natureza do Autocomplete).
function SuggestInput({
  label,
  placeholder,
  suggestions,
  inputProps,
}: {
  label: string;
  placeholder: string;
  suggestions: Suggestions;
  inputProps: object; // GetInputPropsReturnType do @mantine/form (spread direto)
}) {
  return (
    <Autocomplete
      label={label}
      placeholder={placeholder}
      data={suggestions.values}
      renderOption={({ option }) => (
        <Group gap="xs" justify="space-between" style={{ flex: 1 }} wrap="nowrap">
          <Text size="sm">{option.value}</Text>
          <Text size="xs" c="dimmed">
            {suggestions.origin.get(option.value)}
          </Text>
        </Group>
      )}
      {...inputProps}
    />
  );
}

// Campos comuns do form de conquista: o critério é por evento OU por métrica.
type CriterionValues = {
  criterion: "event" | "stat";
  eventType: string;
  statKey: string;
  statMode: StatMode;
  target: number | string;
};

// validators/onSubmit compartilhados entre criar e editar (o critério depende do
// toggle; o servidor é a verdade da validação).
const criterionValidators = {
  eventType: (v: string, values: CriterionValues) =>
    values.criterion === "event" && !v.trim() ? "obrigatório" : null,
  statKey: (v: string, values: CriterionValues) =>
    values.criterion === "stat" && !STAT_RE.test(v.trim()) ? "minúsculo, começa com letra (ex.: kills)" : null,
};

// buildCriterion monta os campos de critério do payload conforme o toggle,
// zerando o lado que não se aplica (o backend rejeita evento+stat juntos).
function buildCriterion(v: CriterionValues): Pick<NewAchievement, "eventType" | "statKey" | "statMode" | "target"> {
  const target = Number(v.target) || 1;
  return v.criterion === "stat"
    ? { eventType: "", statKey: v.statKey.trim(), statMode: v.statMode, target }
    : { eventType: v.eventType.trim(), statKey: "", statMode: undefined, target };
}

// CriterionFields renderiza o seletor de critério + os campos condicionais.
// Reutilizado nos forms de criar e editar (ambos incluem CriterionValues).
// stats/events são sugestões derivadas dos modos do jogo (027) — opcionais:
// sem elas o campo degrada para texto livre puro.
function CriterionFields<T extends CriterionValues>({
  form,
  stats = NO_SUGGESTIONS,
  events = NO_SUGGESTIONS,
}: {
  form: UseFormReturnType<T>;
  stats?: Suggestions;
  events?: Suggestions;
}) {
  const isStat = form.values.criterion === "stat";
  return (
    <>
      <SegmentedControl
        fullWidth
        data={[
          { value: "event", label: "Por evento" },
          { value: "stat", label: "Por métrica" },
        ]}
        {...form.getInputProps("criterion")}
      />
      {isStat ? (
        <>
          <SuggestInput
            label="Stat"
            placeholder="kills"
            suggestions={stats}
            inputProps={form.getInputProps("statKey")}
          />
          <Select
            label="Agregação"
            allowDeselect={false}
            data={[
              { value: "cumulative", label: "Acumulado (soma entre partidas)" },
              { value: "per_match", label: "Por partida (melhor de uma partida)" },
            ]}
            {...form.getInputProps("statMode")}
          />
        </>
      ) : (
        <SuggestInput
          label="Event type"
          placeholder="match_won"
          suggestions={events}
          inputProps={form.getInputProps("eventType")}
        />
      )}
      <NumberInput label={isStat ? "Meta (valor-alvo)" : "Meta (ocorrências)"} min={1} {...form.getInputProps("target")} />
    </>
  );
}

export function GameAchievements({
  gameId,
  onNavigateToRule,
}: {
  gameId: string;
  // onNavigateToRule (spec 057): pedido de navegação pra aba Event Rules com
  // a regra já focada — edit=true só faz sentido pra regra do jogo (a global
  // é sempre read-only lá).
  onNavigateToRule?: (ruleId: string, opts: { edit: boolean }) => void;
}) {
  const { data: catalog, loading, error, reload } = useAsync((s) => api.gameCatalog(gameId, s), [gameId]);
  // modos do jogo p/ sugestões de stat (027); falha → sem sugestões.
  const { data: modes } = useAsync((s) => api.listMultiplayerModes(gameId, s), [gameId]);
  // regras efetivas (migration 008) p/ sugestões/validação de evento — globais
  // do arquétipo + overrides do jogo, substitui o antigo vocabulário hardcoded.
  const { data: eventRules } = useAsync((s) => api.gameEventRules(gameId, s), [gameId]);
  const stats = statSuggestions(modes ?? []);
  // Regras globais só entram na sugestão/validação se o arquétipo delas está
  // cadastrado no jogo (senão um jogo só host_world_dropin sugeriria/aceitaria
  // como "conhecido" vocabulário de queue_arena/battle_royale/coop_mission).
  const archetypes = gameArchetypes(modes ?? []);
  const relevantEventRules = filterRulesForGame(eventRules ?? [], archetypes);
  const events = eventSuggestions(relevantEventRules);
  // Check de drift (029) só com dados carregados E existentes — sem falso alarme.
  const canCheckStats = (modes?.length ?? 0) > 0;
  const canCheckEvents = relevantEventRules.length > 0;
  const statDeclared = (key: string) => !canCheckStats || stats.origin.has(key);
  const eventKnown = (type: string) => !canCheckEvents || events.origin.has(type);
  const [opened, { open, close }] = useDisclosure(false);
  const [editing, setEditing] = useState<CatalogAchievement | null>(null);
  const [deleting, setDeleting] = useState<CatalogAchievement | null>(null);
  // expandedCriteriaId (spec 057): prévia em canvas da tag de critério — mesmo
  // padrão de expander de GameSessions.tsx (uma linha por vez).
  const [expandedCriteriaId, setExpandedCriteriaId] = useState<string | null>(null);

  // targetRuleFor: a regra que emite o event type do achievement — prioriza
  // scope=game (primeiro encontrado), senão a primeira global. Sem
  // desambiguação de múltiplas regras (fora de escopo, spec 057 item 6).
  function targetRuleFor(a: CatalogAchievement): EventRule | undefined {
    const candidates = relevantEventRules.filter((r) => r.eventTypes.includes(a.eventType));
    return candidates.find((r) => r.scope === "game") ?? candidates[0];
  }

  // Importação CSV
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [importing, setImporting] = useState(false);
  const [importResult, setImportResult] = useState<ImportResponse | null>(null);
  const [importErrors, setImportErrors] = useState<ParseError[]>([]);

  // Progresso da conquista
  const [progressAchievement, setProgressAchievement] = useState<CatalogAchievement | null>(null);
  const [progressEntries, setProgressEntries] = useState<AchievementProgressEntry[]>([]);
  const [progressLoading, setProgressLoading] = useState(false);

  async function openProgress(a: CatalogAchievement) {
    setProgressAchievement(a);
    setProgressEntries([]);
    setProgressLoading(true);
    try {
      const { progress } = await api.achievementProgress(gameId, a.id);
      setProgressEntries(progress);
    } catch {
      notifications.show({ color: "red", message: "Falha ao carregar o progresso." });
    } finally {
      setProgressLoading(false);
    }
  }

  const form = useForm({
    initialValues: {
      id: "",
      name: "",
      description: "",
      criterion: "event" as "event" | "stat",
      eventType: "",
      statKey: "",
      statMode: "cumulative" as StatMode,
      target: 1 as number | string,
    },
    validate: {
      id: (v) => (ID_RE.test(v.trim()) ? null : "id minúsculo (ex.: first_win)"),
      name: (v) => (v.trim() ? null : "obrigatório"),
      ...criterionValidators,
    },
  });

  // form de edição: sem id (a chave não muda).
  const editForm = useForm({
    initialValues: {
      name: "",
      description: "",
      criterion: "event" as "event" | "stat",
      eventType: "",
      statKey: "",
      statMode: "cumulative" as StatMode,
      target: 1 as number | string,
    },
    validate: {
      name: (v) => (v.trim() ? null : "obrigatório"),
      ...criterionValidators,
    },
  });
  // editSnapshot guarda os valores no momento em que a edição abriu — é pra
  // onde o botão Resetar volta (diferente de editForm.reset(), que voltaria
  // pros initialValues genéricos do form, não pro que estava salvo desta
  // conquista específica).
  const [editSnapshot, setEditSnapshot] = useState<typeof editForm.values | null>(null);

  // focusRow troca a linha expandida — sai do modo edição sempre que o alvo
  // muda (evita `editing` apontar pra uma conquista que não é mais a expandida).
  function focusRow(id: string | null) {
    setExpandedCriteriaId(id);
    setEditing((prev) => (prev && prev.id !== id ? null : prev));
  }

  async function submit(values: typeof form.values) {
    try {
      await api.createAchievement(gameId, {
        id: values.id,
        name: values.name,
        description: values.description,
        ...buildCriterion(values),
      });
      notifications.show({ color: "green", message: "Conquista adicionada." });
      close();
      form.reset();
      reload();
    } catch (e) {
      const msg =
        e instanceof HttpError && e.status === 409
          ? "Já existe uma conquista com esse id neste jogo."
          : e instanceof HttpError && e.status === 400
            ? "Dados inválidos."
            : "Falha ao adicionar (serviço indisponível?).";
      form.setErrors({ id: msg });
    }
  }

  // openEdit abre a edição INLINE no expander (não é mais modal): expande a
  // linha e já entra em modo edição — clicar na tag do critério continua
  // abrindo só a prévia read-only, sem passar por aqui.
  function openEdit(a: CatalogAchievement) {
    const values = {
      name: a.name,
      description: a.description,
      criterion: (a.statKey ? "stat" : "event") as "event" | "stat",
      eventType: a.eventType ?? "",
      statKey: a.statKey ?? "",
      statMode: a.statMode ?? "cumulative",
      target: a.target,
    };
    editForm.setValues(values);
    setEditSnapshot(values);
    setEditing(a);
    setExpandedCriteriaId(a.id);
  }

  async function submitEdit(values: typeof editForm.values) {
    if (!editing) return;
    try {
      await api.updateAchievement(gameId, editing.id, {
        name: values.name,
        description: values.description,
        ...buildCriterion(values),
      });
      notifications.show({ color: "green", message: "Conquista atualizada." });
      focusRow(null);
      reload();
    } catch (e) {
      const msg =
        e instanceof HttpError && e.status === 404
          ? "Conquista não existe mais."
          : e instanceof HttpError && e.status === 400
            ? "Dados inválidos."
            : "Falha ao salvar (serviço indisponível?).";
      editForm.setErrors({ name: msg });
    }
  }

  // resetEdit volta aos valores do momento em que a edição abriu (não é
  // reload — puramente local, sem chamada ao servidor).
  function resetEdit() {
    if (editSnapshot) editForm.setValues(editSnapshot);
  }

  function handleDownloadTemplate() {
    const blob = new Blob([CSV_TEMPLATE], { type: "text/csv" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "achievements-template.csv";
    a.click();
    URL.revokeObjectURL(url);
  }

  async function handleFileChange(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file) return;

    if (!file.name.toLowerCase().endsWith(".csv")) {
      notifications.show({ color: "red", message: "Selecione um arquivo .csv" });
      return;
    }
    if (file.size > 200_000) {
      notifications.show({ color: "red", message: "Arquivo muito grande (máx. 200 KB)." });
      return;
    }

    const text = await file.text();
    const { rows, errors } = parseCSV(text);

    if (errors.length > 0) {
      setImportErrors(errors);
      return;
    }
    if (rows.length > 500) {
      notifications.show({ color: "red", message: "O arquivo tem mais de 500 conquistas. Divida em arquivos menores." });
      return;
    }

    setImporting(true);
    try {
      const achievements = rows.map(({ id, name, description, eventType, target, statKey, statMode }) => ({
        id, name, description, eventType, target, statKey: statKey ?? "", statMode,
      }));
      const result = await api.importAchievements(gameId, achievements);
      setImportResult(result);
      if (result.created > 0) reload();
    } catch {
      notifications.show({ color: "red", message: "Falha ao importar (serviço indisponível?)." });
    } finally {
      setImporting(false);
    }
  }

  async function confirmDelete() {
    if (!deleting) return;
    try {
      await api.deleteAchievement(gameId, deleting.id);
      notifications.show({ color: "green", message: "Conquista removida." });
      setDeleting(null);
      reload();
    } catch {
      notifications.show({ color: "red", message: "Falha ao remover (serviço indisponível?)." });
      setDeleting(null);
    }
  }

  return (
    <Stack>
      <Group justify="space-between">
        <Text fw={600}>Catálogo de conquistas</Text>
        <Group>
          <Button variant="default" size="xs" leftSection={<IconRefresh size={14} />} onClick={reload}>
            Atualizar
          </Button>
          <Button variant="default" size="xs" leftSection={<IconDownload size={14} />} onClick={handleDownloadTemplate}>
            Baixar modelo
          </Button>
          <Button
            variant="default"
            size="xs"
            leftSection={<IconUpload size={14} />}
            loading={importing}
            onClick={() => fileInputRef.current?.click()}
          >
            Importar CSV
          </Button>
          <input
            ref={fileInputRef}
            type="file"
            accept=".csv"
            style={{ display: "none" }}
            onChange={handleFileChange}
          />
          <Button size="xs" leftSection={<IconPlus size={14} />} onClick={open}>
            Nova conquista
          </Button>
        </Group>
      </Group>

      {loading && !catalog && <Loader />}
      {!!error && <Alert color="red">achievements indisponível.</Alert>}
      {catalog && catalog.length === 0 && <Text c="dimmed">Nenhuma conquista neste jogo ainda.</Text>}
      {catalog && catalog.length > 0 && (
        <Table striped highlightOnHover withTableBorder>
          <Table.Thead>
            <Table.Tr>
              <Table.Th w={40}></Table.Th>
              <Table.Th>Id</Table.Th>
              <Table.Th w={220}>Nome</Table.Th>
              <Table.Th>Descrição</Table.Th>
              <Table.Th>Critério</Table.Th>
              <Table.Th>Meta</Table.Th>
              <Table.Th w={90}></Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {catalog.map((a) => {
              const expanded = expandedCriteriaId === a.id;
              const isEditingThisRow = editing?.id === a.id;
              // previewAchievement/previewCatalog/previewFocusNodeId (edição
              // inline): enquanto editando, o mini-canvas reflete os valores
              // AINDA NÃO salvos do form — troca de critério/meta atualiza o
              // grafo na hora, sem precisar dar Salvar. Fora do modo edição,
              // é o dado real do catálogo (comportamento de sempre).
              const previewAchievement: CatalogAchievement = isEditingThisRow
                ? {
                    ...a,
                    name: editForm.values.name,
                    description: editForm.values.description,
                    ...buildCriterion(editForm.values),
                  }
                : a;
              const previewCatalog = isEditingThisRow
                ? (catalog ?? []).map((item) => (item.id === a.id ? previewAchievement : item))
                : (catalog ?? []);
              // foca na FONTE do stat (não no achievement individual) — mesmo
              // padrão do lado de evento (foca no event type, não no achievement):
              // pathHighlight faz dois caminhos unidirecionais a partir do foco, então
              // só alcança os "irmãos" quando o foco é o nó de ramificação (source/type),
              // nunca a folha (achievement) — achado do Vitor testando com um stat com
              // 5 achievements ligados, só o próprio aparecia na prévia.
              const previewFocusNodeId = previewAchievement.statKey
                ? srcId(STAT_PREFIX + previewAchievement.statKey)
                : typeId(previewAchievement.eventType);
              const rule = previewAchievement.statKey ? undefined : targetRuleFor(previewAchievement);
              return (
                <Fragment key={a.id}>
                  <Table.Tr>
                    <Table.Td>
                      <AchievementIcon seed={a.id} />
                    </Table.Td>
                    <Table.Td c="dimmed">{a.id}</Table.Td>
                    <Table.Td>
                      <Text truncate="end" maw={200} title={a.name}>
                        {a.name}
                      </Text>
                    </Table.Td>
                    <Table.Td c="dimmed">{a.description}</Table.Td>
                    <Table.Td>
                      {a.statKey ? (
                        <Group
                          gap={6}
                          wrap="nowrap"
                          style={{ cursor: "pointer" }}
                          onClick={() => (expanded ? focusRow(null) : focusRow(a.id))}
                        >
                          <Badge variant="light" color="grape" style={{ cursor: "pointer" }}>
                            {a.statKey} · {a.statMode === "per_match" ? "por partida" : "acumulado"}
                          </Badge>
                          {!statDeclared(a.statKey) && (
                            <DriftBadge
                              label="não declarado"
                              tooltip={`O stat "${a.statKey}" não consta dos scoringStats de nenhum modo do jogo. O data plane ainda pode reportá-lo, mas confira se não é typo ou resto de um rename.`}
                            />
                          )}
                        </Group>
                      ) : (
                        <Group
                          gap={6}
                          wrap="nowrap"
                          style={{ cursor: "pointer" }}
                          onClick={() => (expanded ? focusRow(null) : focusRow(a.id))}
                        >
                          <Badge variant="light" color="violet" style={{ cursor: "pointer" }}>
                            {a.eventType}
                          </Badge>
                          {!!a.eventType && !eventKnown(a.eventType) && (
                            <DriftBadge
                              label="fora do vocabulário"
                              tooltip={`O evento "${a.eventType}" não faz parte do vocabulário dos arquétipos cadastrados neste jogo — a conquista pode nunca disparar. Confira se não é typo.`}
                            />
                          )}
                        </Group>
                      )}
                    </Table.Td>
                    <Table.Td>{a.target}</Table.Td>
                    <Table.Td>
                      <Group gap={4} wrap="nowrap">
                        <ActionIcon variant="subtle" color="blue" onClick={() => openProgress(a)} aria-label="Ver progresso">
                          <IconChartBar size={16} />
                        </ActionIcon>
                        <ActionIcon variant="subtle" color="gray" onClick={() => openEdit(a)} aria-label="Editar">
                          <IconPencil size={16} />
                        </ActionIcon>
                        <ActionIcon variant="subtle" color="red" onClick={() => setDeleting(a)} aria-label="Remover">
                          <IconTrash size={16} />
                        </ActionIcon>
                      </Group>
                    </Table.Td>
                  </Table.Tr>
                  {expanded && (
                    <Table.Tr key={`${a.id}-preview`}>
                      <Table.Td colSpan={7} p={0}>
                        <Collapse in={expanded}>
                          <Box p="sm" bg="var(--mantine-color-dark-8)">
                            <Group wrap="nowrap" align="stretch" gap="md">
                              {/* Editar deixou de ser modal (pedido do Vitor): abre
                                  aqui, ao lado do canvas — 20% pro form, 80% pro
                                  canvas (que ocupava 100% antes de existir edição). */}
                              {isEditingThisRow && (
                                <Box style={{ flex: "0 0 20%", minWidth: 240 }}>
                                  <form onSubmit={editForm.onSubmit(submitEdit)}>
                                    <Stack gap="xs">
                                      <TextInput label="Nome" size="xs" {...editForm.getInputProps("name")} />
                                      <Textarea
                                        label="Descrição"
                                        size="xs"
                                        autosize
                                        minRows={2}
                                        {...editForm.getInputProps("description")}
                                      />
                                      <CriterionFields form={editForm} stats={stats} events={events} />
                                      <Button type="submit" size="xs" fullWidth>
                                        Salvar
                                      </Button>
                                      <Group grow gap="xs">
                                        <Button variant="default" size="xs" onClick={resetEdit}>
                                          Resetar
                                        </Button>
                                        <Button variant="subtle" color="gray" size="xs" onClick={() => focusRow(null)}>
                                          Cancelar
                                        </Button>
                                      </Group>
                                    </Stack>
                                  </form>
                                </Box>
                              )}
                              <Box style={{ flex: "1 1 0%", minWidth: 0 }}>
                                <RuleFocusMiniCanvas
                                  rules={relevantEventRules}
                                  catalog={previewCatalog}
                                  focusNodeId={previewFocusNodeId}
                                  overlay={
                                    <Group gap={6}>
                                      {!previewAchievement.statKey && rule && (
                                        <Button
                                          size="xs"
                                          variant="light"
                                          onClick={() => onNavigateToRule?.(rule.id, { edit: rule.scope === "game" })}
                                        >
                                          {rule.scope === "game" ? "Editar regra" : "Ver regra"}
                                        </Button>
                                      )}
                                      <ActionIcon
                                        variant="subtle"
                                        color="gray"
                                        onClick={() => focusRow(null)}
                                        aria-label="Fechar prévia"
                                      >
                                        <IconX size={16} />
                                      </ActionIcon>
                                    </Group>
                                  }
                                />
                              </Box>
                            </Group>
                          </Box>
                        </Collapse>
                      </Table.Td>
                    </Table.Tr>
                  )}
                </Fragment>
              );
            })}
          </Table.Tbody>
        </Table>
      )}

      {/* Nova conquista */}
      <Modal opened={opened} onClose={close} title="Nova conquista">
        <form onSubmit={form.onSubmit(submit)}>
          <Stack>
            <TextInput label="Id (slug)" placeholder="first_win" {...form.getInputProps("id")} />
            <TextInput label="Nome" placeholder="Primeira Vitória" {...form.getInputProps("name")} />
            <Textarea label="Descrição" autosize minRows={2} {...form.getInputProps("description")} />
            <CriterionFields form={form} stats={stats} events={events} />
            <Group justify="flex-end">
              <Button type="submit">Adicionar</Button>
            </Group>
          </Stack>
        </form>
      </Modal>

      {/* Erros de validação do CSV (antes de enviar) */}
      <Modal
        opened={importErrors.length > 0}
        onClose={() => setImportErrors([])}
        title="Arquivo inválido"
        size="lg"
      >
        <Stack>
          <Text size="sm" c="dimmed">
            Corrija os erros abaixo no arquivo CSV e tente novamente. Nenhuma conquista foi importada.
          </Text>
          <List size="sm" spacing={4}>
            {importErrors.map((e) => (
              <List.Item key={`${e.row}-${e.message}`} c="red">
                {e.message}
              </List.Item>
            ))}
          </List>
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setImportErrors([])}>
              Fechar
            </Button>
          </Group>
        </Stack>
      </Modal>

      {/* Resultado da importação */}
      <Modal
        opened={!!importResult}
        onClose={() => setImportResult(null)}
        title="Importação concluída"
        size="lg"
      >
        {importResult && (
          <Stack>
            <Group>
              <Badge color="green" size="lg">
                {importResult.created} criada{importResult.created !== 1 ? "s" : ""}
              </Badge>
              {importResult.failed > 0 && (
                <Badge color="red" size="lg">
                  {importResult.failed} falha{importResult.failed !== 1 ? "s" : ""}
                </Badge>
              )}
            </Group>
            {importResult.failed > 0 && (
              <>
                <Text size="sm" fw={500}>
                  Linhas com erro:
                </Text>
                <Table striped withTableBorder>
                  <Table.Thead>
                    <Table.Tr>
                      <Table.Th w={60}>Linha</Table.Th>
                      <Table.Th>ID</Table.Th>
                      <Table.Th>Motivo</Table.Th>
                    </Table.Tr>
                  </Table.Thead>
                  <Table.Tbody>
                    {importResult.results
                      .filter((r) => r.status !== "created")
                      .map((r) => (
                        <Table.Tr key={r.row}>
                          <Table.Td c="dimmed">{r.row}</Table.Td>
                          <Table.Td>
                            <code>{r.id}</code>
                          </Table.Td>
                          <Table.Td c="red">
                            {r.status === "duplicate"
                              ? "Já existe uma conquista com este ID neste jogo"
                              : r.status === "invalid"
                                ? "Dados inválidos (verifique os campos)"
                                : "Serviço indisponível — tente novamente"}
                          </Table.Td>
                        </Table.Tr>
                      ))}
                  </Table.Tbody>
                </Table>
              </>
            )}
            <Group justify="flex-end">
              <Button onClick={() => setImportResult(null)}>Fechar</Button>
            </Group>
          </Stack>
        )}
      </Modal>

      {/* Progresso da conquista (todos os jogadores) — Drawer à direita
          (spec 047, mesmo padrão do editor de modos), lista visível ao fundo. */}
      <Drawer
        opened={!!progressAchievement}
        onClose={() => { setProgressAchievement(null); setProgressEntries([]); }}
        position="right"
        size={560}
        title={progressAchievement ? `Progresso — ${progressAchievement.name}` : "Progresso"}
      >
        <Stack>
          {progressLoading && <Loader />}
          {!progressLoading && progressEntries.length === 0 && (
            <Text c="dimmed">Nenhum jogador iniciou esta conquista ainda.</Text>
          )}
          {!progressLoading && progressEntries.length > 0 && (
            <>
              <Group gap="md">
                <Badge color="green">
                  {progressEntries.filter((e) => e.unlocked).length} desbloqueada{progressEntries.filter((e) => e.unlocked).length !== 1 ? "s" : ""}
                </Badge>
                <Badge color="blue">
                  {progressEntries.filter((e) => !e.unlocked).length} em progresso
                </Badge>
              </Group>
              <Table striped withTableBorder>
                <Table.Thead>
                  <Table.Tr>
                    <Table.Th>Jogador</Table.Th>
                    <Table.Th ta="center">Progresso</Table.Th>
                    <Table.Th ta="center">Status</Table.Th>
                    <Table.Th>Desbloqueado em</Table.Th>
                  </Table.Tr>
                </Table.Thead>
                <Table.Tbody>
                  {progressEntries.map((e) => (
                    <Table.Tr key={e.playerId}>
                      <Table.Td>
                        <Group gap={6} wrap="nowrap">
                          <PlayerAvatar seed={e.playerId} size={24} />
                          <Tooltip label={e.playerId}>
                            <Text size="sm" c="dimmed">
                              {e.playerId.slice(0, 8)}…
                            </Text>
                          </Tooltip>
                          <GuestBadge playerId={e.playerId} />
                        </Group>
                      </Table.Td>
                      <Table.Td ta="center">
                        {e.progress}/{progressAchievement?.target ?? "?"}
                      </Table.Td>
                      <Table.Td ta="center">
                        <Badge color={e.unlocked ? "green" : "orange"} variant="light">
                          {e.unlocked ? "desbloqueado" : "em progresso"}
                        </Badge>
                      </Table.Td>
                      <Table.Td c="dimmed">
                        {e.unlockedAt ? new Date(e.unlockedAt).toLocaleString("pt-BR") : "—"}
                      </Table.Td>
                    </Table.Tr>
                  ))}
                </Table.Tbody>
              </Table>
            </>
          )}
          <Group justify="flex-end">
            <Button variant="default" onClick={() => { setProgressAchievement(null); setProgressEntries([]); }}>
              Fechar
            </Button>
          </Group>
        </Stack>
      </Drawer>

      {/* Confirmar remoção */}
      <Modal opened={!!deleting} onClose={() => setDeleting(null)} title="Remover conquista">
        <Stack>
          <Text size="sm">
            Remover <b>{deleting?.name}</b> (<code>{deleting?.id}</code>)? O progresso dos jogadores nesta
            conquista também será apagado. Esta ação não pode ser desfeita.
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

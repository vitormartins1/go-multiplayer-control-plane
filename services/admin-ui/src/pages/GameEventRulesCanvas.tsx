import {
  ActionIcon,
  Autocomplete,
  Badge,
  Button,
  CloseButton,
  Drawer,
  Group,
  Modal,
  Paper,
  Select,
  Stack,
  TagsInput,
  Text,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { IconPlus, IconTrash } from "@tabler/icons-react";
import type { Edge, Node, NodeTypes } from "@xyflow/react";
import { useMemo, useState, type DragEvent } from "react";
import { FlowCanvas } from "../components/flow/FlowCanvas";
import { AchievementNode, EventTypeNode, FactNode, RuleNode } from "../components/flow/nodes";
import { api, HttpError, type CatalogAchievement, type EventRule, type MultiplayerProfile } from "../lib/api";
import { ARCHETYPE_META } from "../lib/archetypeMeta";
import {
  buildRulesGraph,
  pathHighlight,
  DRAFT_RULE_ID,
  type AchievementNodeData,
  type DraftRule,
  type EventTypeNodeData,
  type FactNodeData,
} from "../lib/rulesGraph";

// Referência estável (module-level) — ver FlowCanvas: nodeTypes recriado por
// render faria o React Flow re-montar todos os nós.
const NODE_TYPES: NodeTypes = {
  fact: FactNode,
  rule: RuleNode,
  eventType: EventTypeNode,
  achievement: AchievementNode,
};

const DND_MIME = "application/x-rules-fact";

type MatchPair = { key: string; value: string };

// GameEventRulesCanvas (specs 043+044 / ADR-0028): grafo de 4 camadas sobre o
// MESMO CRUD da tabela. Layout 044: toolbar em cima (arquétipo + nova regra),
// canvas na largura toda, detalhe do nó em overlay flutuante e criação num
// drawer de baixo — enquanto o drawer está aberto, a regra em edição aparece
// como NÓ RASCUNHO ligado ao vivo aos facts/event types (clicar num nó ou
// arrastar um chip do drawer adiciona ao draft).
export function GameEventRulesCanvas({
  gameId,
  rules,
  catalog,
  archetypeOptions,
  reload,
}: {
  gameId: string;
  rules: EventRule[];
  catalog: CatalogAchievement[];
  archetypeOptions: MultiplayerProfile["archetype"][];
  reload: () => void;
}) {
  const [archetype, setArchetype] = useState<MultiplayerProfile["archetype"]>(archetypeOptions[0]);
  // Se o jogo ganhou/perdeu modos e o arquétipo selecionado sumiu das opções,
  // volta pro primeiro válido (sem efeito: derivação no render).
  const arch = archetypeOptions.includes(archetype) ? archetype : archetypeOptions[0];

  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [matchPairs, setMatchPairs] = useState<MatchPair[]>([{ key: "", value: "" }]);
  const [eventTypes, setEventTypes] = useState<string[]>([]);
  const [formError, setFormError] = useState<string | null>(null);
  const [deleting, setDeleting] = useState<EventRule | null>(null);

  const archRules = useMemo(() => rules.filter((r) => r.archetype === arch), [rules, arch]);

  // Rascunho ao vivo (R2 da spec 044): só os pares preenchidos entram no nó.
  const draft: DraftRule | undefined = useMemo(() => {
    if (!drawerOpen) return undefined;
    const match: Record<string, string> = {};
    for (const { key, value } of matchPairs) {
      if (key.trim()) match[key.trim()] = value.trim();
    }
    return { match, eventTypes };
  }, [drawerOpen, matchPairs, eventTypes]);

  const graph = useMemo(() => buildRulesGraph(archRules, catalog, draft), [archRules, catalog, draft]);

  // Vocabulário conhecido do arquétipo (chips do drawer + sugestões) — sempre
  // derivado do dado real, nunca hardcoded (princípio do ADR-0027).
  const knownFacts = useMemo(() => {
    const seen = new Map<string, MatchPair>();
    for (const r of archRules) {
      for (const [key, value] of Object.entries(r.match)) seen.set(`${key}=${value}`, { key, value });
    }
    return [...seen.values()];
  }, [archRules]);
  const keySuggestions = useMemo(() => [...new Set(knownFacts.map((f) => f.key))], [knownFacts]);
  const typeSuggestions = useMemo(
    () => [...new Set(archRules.flatMap((r) => r.eventTypes))],
    [archRules],
  );

  // Decoração: caminho da seleção (ativo/esmaecido) preservando a marcação de
  // rascunho vinda do build; edges do rascunho são sempre proeminentes.
  const { nodes, edges } = useMemo(() => {
    const path = selectedId && !drawerOpen ? pathHighlight(graph.edges, selectedId) : null;
    const cls = (base: string | undefined, extra: string | undefined) =>
      [base, extra].filter(Boolean).join(" ") || undefined;
    const nodes: Node[] = graph.nodes.map((n) => ({
      ...n,
      className: cls(n.className, path ? (path.nodeIds.has(n.id) ? "is-active" : "is-faded") : undefined),
    }));
    const edges: Edge[] = graph.edges.map((e) => {
      if (e.className === "is-draft-edge") {
        return { ...e, animated: true, style: { stroke: "#fab005", strokeWidth: 1.5 } };
      }
      return path && path.edgeIds.has(e.id)
        ? { ...e, animated: true, style: { stroke: "#748ffc", strokeWidth: 2 } }
        : { ...e, className: cls(e.className, path ? "is-faded" : undefined), style: { stroke: "#3a4657" } };
    });
    return { nodes, edges };
  }, [graph, selectedId, drawerOpen]);

  const selectedNode = useMemo(
    () => graph.nodes.find((n) => n.id === selectedId) ?? null,
    [graph, selectedId],
  );

  function openCreate(prefill?: MatchPair) {
    setDrawerOpen(true);
    setSelectedId(null);
    setFormError(null);
    setMatchPairs(prefill ? [prefill, { key: "", value: "" }] : [{ key: "", value: "" }]);
    setEventTypes([]);
  }

  function closeDrawer() {
    // R3: cancelar não deixa resíduo — o draft é derivado do estado do form,
    // então basta fechar (o buildRulesGraph deixa de receber o rascunho).
    setDrawerOpen(false);
    setFormError(null);
  }

  // addFactToDraft soma um par ao draft sem duplicar, mantendo uma linha vazia
  // no fim pro próximo par manual.
  function addFactToDraft(fact: MatchPair) {
    setMatchPairs((p) => {
      const filled = p.filter((r) => r.key.trim());
      if (filled.some((r) => r.key === fact.key && r.value === fact.value)) return p;
      return [...filled, fact, { key: "", value: "" }];
    });
  }

  function onDrop(e: DragEvent) {
    const raw = e.dataTransfer.getData(DND_MIME);
    if (!raw) return;
    e.preventDefault();
    const fact = JSON.parse(raw) as MatchPair;
    if (drawerOpen) addFactToDraft(fact);
    else openCreate(fact);
  }

  // Com o drawer aberto o canvas vira superfície de montagem (R4): clicar num
  // fact/event type adiciona ao rascunho em vez de selecionar/destacar.
  function onNodeClick(id: string) {
    if (!drawerOpen) {
      setSelectedId(id);
      return;
    }
    if (id === DRAFT_RULE_ID) return;
    const node = graph.nodes.find((n) => n.id === id);
    if (node?.type === "fact") {
      const d = node.data as unknown as FactNodeData;
      addFactToDraft({ key: d.factKey, value: d.factValue });
    } else if (node?.type === "eventType") {
      const d = node.data as unknown as EventTypeNodeData;
      setEventTypes((ts) => (ts.includes(d.eventType) ? ts : [...ts, d.eventType]));
    }
  }

  async function submit() {
    if (!draft || Object.keys(draft.match).length === 0 || draft.eventTypes.length === 0) {
      setFormError("Preencha ao menos um par do match e um event type.");
      return;
    }
    try {
      await api.createEventRule(gameId, { archetype: arch, match: draft.match, eventTypes: draft.eventTypes });
      notifications.show({ color: "green", message: "Regra adicionada." });
      closeDrawer();
      reload();
    } catch (e) {
      setFormError(
        e instanceof HttpError && e.status === 400
          ? "Dados inválidos (confira archetype/match/eventTypes)."
          : "Falha ao adicionar (serviço indisponível?).",
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

  return (
    <Stack gap="xs">
      {/* Toolbar (spec 044): contexto em cima, canvas na largura toda. */}
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
              closeDrawer();
            }
          }}
          allowDeselect={false}
        />
        <Button size="xs" leftSection={<IconPlus size={14} />} onClick={() => openCreate()} disabled={drawerOpen}>
          Nova regra
        </Button>
        {drawerOpen && (
          <Text size="xs" c="dimmed">
            Montando rascunho — clique num fact/event type do canvas pra adicioná-lo à regra.
          </Text>
        )}
      </Group>

      <FlowCanvas
        nodes={nodes}
        edges={edges}
        nodeTypes={NODE_TYPES}
        onNodeClick={onNodeClick}
        onPaneClick={() => setSelectedId(null)}
        onDrop={onDrop}
        // re-enquadra quando o grafo muda de forma (arquétipo, nós do
        // rascunho) — senão o ghost nasce fora da vista. O canvas NÃO muda de
        // tamanho com o drawer (feedback 044): o rascunho fica no TOPO da
        // coluna de regras (ver rulesGraph), longe da faixa coberta.
        fitKey={`${arch}|${graph.nodes.length}`}
        overlay={
          selectedNode && !drawerOpen ? (
            <Paper withBorder p="sm" w={280} shadow="md" style={{ background: "rgba(18, 24, 33, 0.95)" }}>
              <CloseButton
                size="sm"
                onClick={() => setSelectedId(null)}
                style={{ position: "absolute", top: 8, right: 8 }}
                aria-label="Fechar detalhe"
              />
              <NodeDetails node={selectedNode} onDelete={(r) => setDeleting(r)} />
            </Paper>
          ) : undefined
        }
      />

      {/* Criação visual (spec 044): drawer de baixo, canvas segue interativo
          (sem overlay/trap de foco) e o rascunho reflete ao vivo. */}
      <Drawer
        opened={drawerOpen}
        onClose={closeDrawer}
        position="bottom"
        size={270}
        padding="sm"
        withOverlay={false}
        trapFocus={false}
        lockScroll={false}
        title={`Nova regra — ${ARCHETYPE_META[arch].label}`}
      >
        <Stack gap="xs">
          {/* Duas colunas fluidas (quebram em janela estreita), ações no
              rodapé — layout anterior de 3 colunas fixas estourava. */}
          <Group align="flex-start" gap="xl" wrap="wrap">
            <Stack gap={6} style={{ flex: "1 1 320px", maxWidth: 480 }}>
              <Text size="xs" c="dimmed">
                Match (todos os pares precisam bater no evento)
              </Text>
              {matchPairs.map((row, i) => (
                <Group key={i} gap={4} wrap="nowrap">
                  <Autocomplete
                    size="xs"
                    placeholder="chave"
                    data={keySuggestions}
                    value={row.key}
                    onChange={(v) => setMatchPairs((p) => p.map((r, idx) => (idx === i ? { ...r, key: v } : r)))}
                    style={{ flex: 1 }}
                  />
                  <Autocomplete
                    size="xs"
                    placeholder="valor"
                    data={[...new Set(knownFacts.filter((f) => f.key === row.key).map((f) => f.value))]}
                    value={row.value}
                    onChange={(v) => setMatchPairs((p) => p.map((r, idx) => (idx === i ? { ...r, value: v } : r)))}
                    style={{ flex: 1 }}
                  />
                  <ActionIcon
                    variant="subtle"
                    color="red"
                    size="sm"
                    disabled={matchPairs.length === 1}
                    onClick={() => setMatchPairs((p) => p.filter((_, idx) => idx !== i))}
                    aria-label="Remover par"
                  >
                    <IconTrash size={14} />
                  </ActionIcon>
                </Group>
              ))}
              <Group gap={4}>
                <Button
                  variant="default"
                  size="xs"
                  onClick={() => setMatchPairs((p) => [...p, { key: "", value: "" }])}
                >
                  + par chave/valor
                </Button>
                {knownFacts.map((f) => (
                  <Badge
                    key={`${f.key}=${f.value}`}
                    className="flow-palette-item"
                    variant="light"
                    color="blue"
                    tt="none"
                    draggable
                    onDragStart={(e) => e.dataTransfer.setData(DND_MIME, JSON.stringify(f))}
                    onClick={() => addFactToDraft(f)}
                    style={{ cursor: "pointer" }}
                  >
                    {f.key}={f.value}
                  </Badge>
                ))}
              </Group>
            </Stack>

            <Stack gap={6} style={{ flex: "1 1 280px", maxWidth: 420 }}>
              <TagsInput
                size="xs"
                label="Event types"
                description="Enter adiciona; clique num event type do canvas também."
                data={typeSuggestions}
                value={eventTypes}
                onChange={setEventTypes}
              />
            </Stack>
          </Group>

          <Group justify="flex-end" gap="xs">
            {formError && (
              <Text size="xs" c="red">
                {formError}
              </Text>
            )}
            <Button variant="default" size="xs" onClick={closeDrawer}>
              Cancelar
            </Button>
            <Button size="xs" onClick={submit}>
              Salvar regra
            </Button>
          </Group>
        </Stack>
      </Drawer>

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

// NodeDetails traduz o nó selecionado pro painel flutuante — cada camada
// mostra o que importa dela; só regra do jogo tem ação (remover).
function NodeDetails({ node, onDelete }: { node: Node; onDelete: (r: EventRule) => void }) {
  if (node.type === "rule") {
    const { rule } = node.data as { rule: EventRule };
    const global = rule.scope === "global";
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
          Match (subconjunto exato dos facts do evento):
        </Text>
        <Group gap={4}>
          {Object.entries(rule.match).length === 0 ? (
            <Text size="xs" c="dimmed">
              (sempre bate)
            </Text>
          ) : (
            Object.entries(rule.match).map(([k, v]) => (
              <Badge key={k} size="xs" variant="light" color="gray" tt="none">
                {k}={v}
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
        {global ? (
          <Text size="xs" c="dimmed">
            Regra global do arquétipo (semeada por migration) — somente leitura.
          </Text>
        ) : (
          <Button
            variant="light"
            color="red"
            size="xs"
            leftSection={<IconTrash size={14} />}
            onClick={() => onDelete(rule)}
          >
            Remover regra
          </Button>
        )}
      </Stack>
    );
  }
  if (node.type === "fact") {
    const d = node.data as unknown as FactNodeData;
    return (
      <Stack gap="xs">
        <Text fw={600} size="sm">
          Fact
        </Text>
        <Badge variant="light" color="blue" tt="none" style={{ alignSelf: "flex-start" }}>
          {d.factKey}={d.factValue}
        </Badge>
        <Text size="xs" c="dimmed">
          Par chave/valor normalizado do evento recebido — o caminho destacado mostra quais regras o
          usam.
        </Text>
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
        <Text size="xs" c="dimmed">
          {d.listeners === 0
            ? "Nenhum achievement deste jogo escuta este event type ainda (cadastre um na aba Achievements)."
            : `${d.listeners} achievement${d.listeners === 1 ? "" : "s"} deste jogo escuta${d.listeners === 1 ? "" : "m"} este event type.`}
        </Text>
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
        Escuta <code>{d.eventType}</code> · target {d.target}. Gerenciado na aba Achievements.
      </Text>
    </Stack>
  );
}

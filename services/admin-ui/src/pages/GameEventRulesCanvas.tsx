import {
  ActionIcon,
  Autocomplete,
  Badge,
  Button,
  Divider,
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
  type AchievementNodeData,
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

// GameEventRulesCanvas é a view em canvas da aba Event Rules (spec 043 /
// ADR-0028): grafo de 4 camadas (facts -> regras -> event types ->
// achievements) sobre o MESMO CRUD da tabela — nenhum endpoint novo. Paleta à
// esquerda (nova regra + facts arrastáveis), painel de propriedades à direita
// (detalhe do nó selecionado ou form de criação). Seleção destaca o caminho
// completo com edges animadas; o resto esmaece.
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
  const [creating, setCreating] = useState(false);
  const [matchPairs, setMatchPairs] = useState<MatchPair[]>([{ key: "", value: "" }]);
  const [eventTypes, setEventTypes] = useState<string[]>([]);
  const [formError, setFormError] = useState<string | null>(null);
  const [deleting, setDeleting] = useState<EventRule | null>(null);

  const archRules = useMemo(() => rules.filter((r) => r.archetype === arch), [rules, arch]);
  const graph = useMemo(() => buildRulesGraph(archRules, catalog), [archRules, catalog]);

  // Vocabulário conhecido do arquétipo (paleta + sugestões do form) — sempre
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

  // Decoração de seleção: caminho ativo (BFS bidirecional) vs esmaecido.
  const { nodes, edges } = useMemo(() => {
    const path = selectedId ? pathHighlight(graph.edges, selectedId) : null;
    const nodes: Node[] = graph.nodes.map((n) => ({
      ...n,
      className: path ? (path.nodeIds.has(n.id) ? "is-active" : "is-faded") : undefined,
    }));
    const edges: Edge[] = graph.edges.map((e) =>
      path && path.edgeIds.has(e.id)
        ? { ...e, animated: true, style: { stroke: "#748ffc", strokeWidth: 2 } }
        : { ...e, className: path ? "is-faded" : undefined, style: { stroke: "#3a4657" } },
    );
    return { nodes, edges };
  }, [graph, selectedId]);

  const selectedNode = useMemo(
    () => graph.nodes.find((n) => n.id === selectedId) ?? null,
    [graph, selectedId],
  );

  function openCreate(prefill?: MatchPair) {
    setCreating(true);
    setSelectedId(null);
    setFormError(null);
    setMatchPairs(prefill ? [prefill, { key: "", value: "" }] : [{ key: "", value: "" }]);
    setEventTypes([]);
  }

  function onDrop(e: DragEvent) {
    const raw = e.dataTransfer.getData(DND_MIME);
    if (!raw) return;
    e.preventDefault();
    const fact = JSON.parse(raw) as MatchPair;
    if (creating) {
      // já criando: soma o fact ao draft (sem duplicar)
      setMatchPairs((p) => {
        const filled = p.filter((r) => r.key.trim());
        if (filled.some((r) => r.key === fact.key && r.value === fact.value)) return p;
        return [...filled, fact, { key: "", value: "" }];
      });
    } else {
      openCreate(fact);
    }
  }

  async function submit() {
    const match: Record<string, string> = {};
    for (const { key, value } of matchPairs) {
      if (key.trim()) match[key.trim()] = value.trim();
    }
    if (Object.keys(match).length === 0 || eventTypes.length === 0) {
      setFormError("Preencha ao menos um par do match e um event type.");
      return;
    }
    try {
      await api.createEventRule(gameId, { archetype: arch, match, eventTypes });
      notifications.show({ color: "green", message: "Regra adicionada." });
      setCreating(false);
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
    <Group align="stretch" gap="sm" wrap="nowrap">
      {/* Paleta */}
      <Paper withBorder p="sm" w={215} style={{ flexShrink: 0 }}>
        <Stack gap="xs">
          <Select
            label="Arquétipo"
            size="xs"
            data={archetypeOptions.map((a) => ({ value: a, label: ARCHETYPE_META[a].label }))}
            value={arch}
            onChange={(v) => {
              if (v) {
                setArchetype(v as MultiplayerProfile["archetype"]);
                setSelectedId(null);
                setCreating(false);
              }
            }}
            allowDeselect={false}
          />
          <Button size="xs" leftSection={<IconPlus size={14} />} onClick={() => openCreate()}>
            Nova regra
          </Button>
          <Divider label="Facts do arquétipo" labelPosition="center" />
          {knownFacts.length === 0 && (
            <Text size="xs" c="dimmed">
              Nenhum fact conhecido ainda.
            </Text>
          )}
          <Group gap={4}>
            {knownFacts.map((f) => (
              <Badge
                key={`${f.key}=${f.value}`}
                className="flow-palette-item"
                variant="light"
                color="blue"
                tt="none"
                draggable
                onDragStart={(e) => e.dataTransfer.setData(DND_MIME, JSON.stringify(f))}
              >
                {f.key}={f.value}
              </Badge>
            ))}
          </Group>
          <Text size="xs" c="dimmed">
            Arraste um fact pro canvas pra começar uma regra com ele.
          </Text>
        </Stack>
      </Paper>

      {/* Canvas */}
      <FlowCanvas
        nodes={nodes}
        edges={edges}
        nodeTypes={NODE_TYPES}
        onNodeClick={(id) => {
          setSelectedId(id);
          setCreating(false);
        }}
        onPaneClick={() => setSelectedId(null)}
        onDrop={onDrop}
      />

      {/* Painel de propriedades */}
      <Paper withBorder p="sm" w={300} style={{ flexShrink: 0 }}>
        {creating ? (
          <Stack gap="xs">
            <Text fw={600} size="sm">
              Nova regra — {ARCHETYPE_META[arch].label}
            </Text>
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
                  data={[
                    ...new Set(knownFacts.filter((f) => f.key === row.key).map((f) => f.value)),
                  ]}
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
            <Button variant="default" size="xs" onClick={() => setMatchPairs((p) => [...p, { key: "", value: "" }])}>
              + par chave/valor
            </Button>
            <TagsInput
              size="xs"
              label="Event types"
              description="Enter adiciona cada um."
              data={typeSuggestions}
              value={eventTypes}
              onChange={setEventTypes}
            />
            {formError && (
              <Text size="xs" c="red">
                {formError}
              </Text>
            )}
            <Group justify="flex-end" gap="xs">
              <Button variant="default" size="xs" onClick={() => setCreating(false)}>
                Cancelar
              </Button>
              <Button size="xs" onClick={submit}>
                Adicionar
              </Button>
            </Group>
          </Stack>
        ) : selectedNode ? (
          <NodeDetails
            node={selectedNode}
            onDelete={(r) => setDeleting(r)}
          />
        ) : (
          <Stack gap="xs">
            <Text fw={600} size="sm">
              Propriedades
            </Text>
            <Text size="xs" c="dimmed">
              Selecione um nó pra ver os detalhes e destacar o caminho evento → regra → event type →
              achievement. Regras globais (cinza) valem pra todo jogo do arquétipo e são somente
              leitura; regras do jogo (verde) podem ser removidas.
            </Text>
          </Stack>
        )}
      </Paper>

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
    </Group>
  );
}

// NodeDetails traduz o nó selecionado pro painel — cada camada mostra o que
// importa dela; só regra do jogo tem ação (remover).
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

import { Alert, CloseButton, Loader, Paper, Stack, Text } from "@mantine/core";
import type { Node, NodeTypes } from "@xyflow/react";
import { useMemo, useState } from "react";
import { FlowCanvas } from "../components/flow/FlowCanvas";
import {
  PipelineConsumerNode,
  PipelineKindNode,
  PipelineModeNode,
} from "../components/flow/pipelineNodes";
import { api } from "../lib/api";
import { buildPipelineGraph, type PipelineConsumerData } from "../lib/pipelineGraph";
import { gameArchetypes, filterRulesForGame } from "../lib/eventRules";
import { useAsync } from "../lib/useAsync";

const NODE_TYPES: NodeTypes = {
  pipelineMode: PipelineModeNode,
  pipelineKind: PipelineKindNode,
  pipelineConsumer: PipelineConsumerNode,
};

// GamePipeline (spec 046): o mapa de dados do jogo — modos -> envelope ->
// consumers, read-only, 3º uso do kit flow. Torna visível a fronteira do
// adendo ADR-0028 (event rules só no ramo do achievements; participation não
// alimenta leaderboard) e tira as scoringStats do esconderijo do form.
export function GamePipeline({ gameId }: { gameId: string }) {
  const { data: modes, loading, error } = useAsync((s) => api.listMultiplayerModes(gameId, s), [gameId]);
  const { data: rules } = useAsync((s) => api.gameEventRules(gameId, s), [gameId]);
  const { data: catalog } = useAsync((s) => api.gameCatalog(gameId, s), [gameId]);

  const [selectedId, setSelectedId] = useState<string | null>(null);

  const graph = useMemo(() => {
    if (!modes) return { nodes: [], edges: [] };
    const visibleRules = filterRulesForGame(rules ?? [], gameArchetypes(modes));
    return buildPipelineGraph(modes, visibleRules, catalog ?? []);
  }, [modes, rules, catalog]);

  const selectedNode = useMemo(
    () => graph.nodes.find((n) => n.id === selectedId) ?? null,
    [graph, selectedId],
  );

  if (loading && !modes) return <Loader />;
  if (error && !modes) return <Alert color="red">game-registry indisponível.</Alert>;
  if (modes && modes.length === 0) {
    return (
      <Text c="dimmed">
        Sem modos multiplayer cadastrados — o pipeline aparece quando o jogo tiver ao menos um modo
        (Configurações → Multiplayer).
      </Text>
    );
  }

  return (
    <Stack gap="xs">
      <Text size="sm" c="dimmed">
        O resultado de cada partida vira um <b>envelope</b> publicado no fan-out (SNS→SQS) e cada
        consumer o traduz pra sua própria língua — as event rules moram só no ramo do achievements.
      </Text>
      <FlowCanvas
        nodes={graph.nodes}
        edges={graph.edges.map((e) => ({ ...e, style: { stroke: "#3a4657" } }))}
        nodeTypes={NODE_TYPES}
        fitKey={graph.nodes.length}
        onNodeClick={setSelectedId}
        onPaneClick={() => setSelectedId(null)}
        overlay={
          selectedNode ? (
            <Paper withBorder p="sm" w={290} shadow="md" style={{ background: "rgba(18, 24, 33, 0.95)" }}>
              <CloseButton
                size="sm"
                onClick={() => setSelectedId(null)}
                style={{ position: "absolute", top: 8, right: 8 }}
                aria-label="Fechar detalhe"
              />
              <PipelineDetails node={selectedNode} />
            </Paper>
          ) : undefined
        }
      />
    </Stack>
  );
}

function PipelineDetails({ node }: { node: Node }) {
  if (node.type === "pipelineConsumer") {
    const d = node.data as unknown as PipelineConsumerData;
    return (
      <Stack gap="xs">
        <Text fw={600} size="sm">
          {d.title}
        </Text>
        <Text size="xs" c="dimmed">
          {d.hint}
        </Text>
      </Stack>
    );
  }
  if (node.type === "pipelineKind") {
    return (
      <Stack gap="xs">
        <Text fw={600} size="sm">
          Envelope
        </Text>
        <Text size="xs" c="dimmed">
          Contrato do fan-out (ADR-0020/0023): a forma do resultado publicada pelo session-service ao
          fim da partida. Carrega também o <code>modeId</code> (ADR-0024) — usado como fact{" "}
          <code>mode</code> nas event rules.
        </Text>
      </Stack>
    );
  }
  return (
    <Stack gap="xs">
      <Text fw={600} size="sm">
        Modo
      </Text>
      <Text size="xs" c="dimmed">
        Perfil de multiplayer nomeado (ADR-0024). As stats declaradas aqui viram boards no
        leaderboard e podem alimentar achievements por métrica — o vocabulário de dados do jogo
        nasce neste cadastro.
      </Text>
    </Stack>
  );
}

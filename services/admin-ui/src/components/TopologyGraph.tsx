import { useMemo } from "react";
import { ReactFlow, Background, Controls, type Node, type Edge as RFEdge } from "@xyflow/react";
import type { Topology } from "../lib/api";
import { statusClass } from "./StatusBadge";

// Posições estáticas por id (8 nós conhecidos) — layout simples e estável.
const POS: Record<string, { x: number; y: number }> = {
  "admin-bff": { x: 40, y: 20 },
  "identity-service": { x: 320, y: 20 },
  "achievements-service": { x: 40, y: 150 },
  "player-data-service": { x: 210, y: 150 },
  "cloud-save-service": { x: 380, y: 150 },
  "leaderboard-service": { x: 550, y: 150 },
  "matchmaking-service": { x: 180, y: 290 },
  "session-service": { x: 420, y: 290 },
};

// TopologyGraph: grafo read-only dos serviços, cor por status; clique no nó
// navega ao domínio. Sem drag/edição.
export function TopologyGraph({
  topo,
  onSelectDomain,
}: {
  topo: Topology;
  onSelectDomain: (id: string) => void;
}) {
  const nodes: Node[] = useMemo(
    () =>
      topo.nodes.map((n) => ({
        id: n.id,
        position: POS[n.id] ?? { x: 0, y: 0 },
        data: { label: n.label },
        className: `topo-node ${statusClass(n.up)}`,
        connectable: false,
        draggable: false,
      })),
    [topo.nodes],
  );

  const edges: RFEdge[] = useMemo(
    () =>
      topo.edges.map((e, i) => ({
        id: `e${i}`,
        source: e.from,
        target: e.to,
        label: e.label,
        labelStyle: { fill: "#9aa7b4", fontSize: 11 },
        style: { stroke: "#2a3340" },
      })),
    [topo.edges],
  );

  return (
    <div className="topology">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodesDraggable={false}
        nodesConnectable={false}
        edgesFocusable={false}
        fitView
        onNodeClick={(_, node) => onSelectDomain(node.id)}
        proOptions={{ hideAttribution: true }}
      >
        <Background color="#2a3340" gap={20} />
        <Controls showInteractive={false} />
      </ReactFlow>
    </div>
  );
}

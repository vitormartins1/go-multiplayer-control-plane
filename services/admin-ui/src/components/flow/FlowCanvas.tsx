import {
  Background,
  Controls,
  MiniMap,
  ReactFlow,
  type Edge,
  type Node,
  type NodeTypes,
} from "@xyflow/react";
import type { DragEvent } from "react";
import "./flow.css";

// Cor de cada tipo de nó no MiniMap (mesma paleta dos headers em flow.css).
const MINIMAP_COLOR: Record<string, string> = {
  fact: "#4dabf7",
  rule: "#20c997",
  eventType: "#b197fc",
  achievement: "#fab005",
};

// FlowCanvas é o wrapper tematizado do React Flow (kit da spec 043 R7):
// Background/Controls/MiniMap dark, fitView, sem edição estrutural (nós não
// conectáveis — a estrutura vem sempre do dado, nunca do desenho). nodeTypes
// chega por prop com referência estável (module-level no chamador), senão o
// React Flow re-monta os nós a cada render.
export function FlowCanvas({
  nodes,
  edges,
  nodeTypes,
  onNodeClick,
  onPaneClick,
  onDrop,
}: {
  nodes: Node[];
  edges: Edge[];
  nodeTypes: NodeTypes;
  onNodeClick?: (nodeId: string) => void;
  onPaneClick?: () => void;
  onDrop?: (e: DragEvent) => void;
}) {
  return (
    <div
      className="flow-wrap"
      style={{ flex: 1, minWidth: 0 }}
      onDragOver={onDrop ? (e) => e.preventDefault() : undefined}
      onDrop={onDrop}
    >
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        colorMode="dark"
        fitView
        fitViewOptions={{ padding: 0.15, maxZoom: 1 }}
        minZoom={0.3}
        nodesConnectable={false}
        edgesFocusable={false}
        deleteKeyCode={null}
        onNodeClick={onNodeClick ? (_, n) => onNodeClick(n.id) : undefined}
        onPaneClick={onPaneClick}
        proOptions={{ hideAttribution: true }}
      >
        <Background color="#232c3a" gap={22} />
        <Controls showInteractive={false} />
        <MiniMap
          pannable
          zoomable
          nodeColor={(n) => MINIMAP_COLOR[n.type ?? ""] ?? "#8b949e"}
          nodeStrokeWidth={2}
          style={{ width: 140, height: 96 }}
        />
      </ReactFlow>
    </div>
  );
}

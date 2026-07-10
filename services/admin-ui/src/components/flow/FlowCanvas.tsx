import {
  Background,
  Controls,
  MiniMap,
  Panel,
  ReactFlow,
  useReactFlow,
  type Edge,
  type Node,
  type NodeTypes,
} from "@xyflow/react";
import { useEffect, type DragEvent, type ReactNode } from "react";
import "./flow.css";

// AutoFit re-enquadra o grafo quando `fitKey` muda (nó rascunho adicionado,
// troca de arquétipo, resize do canvas) — o fitView do ReactFlow só roda no
// mount. Filho de <ReactFlow>, então tem acesso ao store interno.
function AutoFit({ fitKey }: { fitKey: unknown }) {
  const rf = useReactFlow();
  useEffect(() => {
    const t = setTimeout(() => rf.fitView({ padding: 0.15, maxZoom: 1, duration: 250 }), 50);
    return () => clearTimeout(t);
  }, [fitKey, rf]);
  return null;
}

// Cor de cada tipo de nó no MiniMap (mesma paleta dos headers em flow.css).
const MINIMAP_COLOR: Record<string, string> = {
  fact: "#4dabf7",
  rule: "#20c997",
  eventType: "#b197fc",
  stat: "#ffa94d",
  achievement: "#fab005",
  pipelineMode: "#4dabf7",
  pipelineKind: "#b197fc",
  pipelineConsumer: "#51cf66",
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
  dock,
  overlay,
  fitKey,
  height,
}: {
  nodes: Node[];
  edges: Edge[];
  nodeTypes: NodeTypes;
  onNodeClick?: (nodeId: string) => void;
  onPaneClick?: () => void;
  onDrop?: (e: DragEvent) => void;
  // dock flutua no canto oposto ao overlay (Panel top-left) — painel de
  // navegação/controles do canvas (spec 058), mesmo mecanismo do overlay.
  dock?: ReactNode;
  // overlay flutua sobre o canvas (Panel top-right) — detalhe do nó
  // selecionado etc., sem coluna lateral roubando largura (spec 044).
  overlay?: ReactNode;
  // fitKey: qualquer mudança re-enquadra o grafo (ver AutoFit).
  fitKey?: unknown;
  // height sobrescreve os 580px do CSS — número (px) ou expressão CSS
  // (ex.: "calc(100vh - 340px)" pro canvas ocupar a altura da aba, spec 047).
  height?: number | string;
}) {
  return (
    <div
      className="flow-wrap"
      style={height ? { height, minHeight: 460 } : undefined}
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
        {fitKey !== undefined && <AutoFit fitKey={fitKey} />}
        {dock && <Panel position="top-left">{dock}</Panel>}
        {overlay && <Panel position="top-right">{overlay}</Panel>}
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

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

const FIT_MARGIN = 90; // respiro (px) além da área reservada pro dock/overlay.

// AutoFit re-enquadra o grafo quando `fitKey` muda (nó rascunho adicionado,
// troca de arquétipo, resize do canvas) — o fitView do ReactFlow só roda no
// mount. Filho de <ReactFlow>, então tem acesso ao store interno.
// `reservedLeft` (spec 058, achado do Vitor): o fitView padrão considera o
// canvas INTEIRO, sem saber que o dock flutuante cobre uma faixa da
// esquerda — nós acabavam posicionados por baixo da lista. O padding do
// fitView aceita valores ASSIMÉTRICOS por lado (reserva real de espaço, não
// só respiro visual) — reserva a largura MEDIDA do dock (ResizeObserver, não
// um número chutado) só do lado esquerdo.
function AutoFit({ fitKey, reservedLeft }: { fitKey: unknown; reservedLeft: number }) {
  const rf = useReactFlow();
  useEffect(() => {
    const t = setTimeout(
      () =>
        rf.fitView({
          padding: {
            left: `${reservedLeft + FIT_MARGIN}px`,
            right: `${FIT_MARGIN}px`,
            top: `${FIT_MARGIN}px`,
            bottom: `${FIT_MARGIN}px`,
          },
          maxZoom: 1,
          duration: 250,
        }),
      50,
    );
    return () => clearTimeout(t);
  }, [fitKey, reservedLeft, rf]);
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
  dockReservedWidth,
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
  // dockReservedWidth (spec 058, achado do Vitor): largura (px) que o
  // fitView deve reservar à esquerda — NEM sempre é a largura inteira do
  // `dock` (ex.: um rail de ações ao lado da lista não bloqueia nós por
  // baixo dele, só a lista bloqueia; reservar a largura toda empurrava o
  // grafo mais pra direita do que precisava). Quem sabe a estrutura interna
  // do `dock` é o chamador — ele mede só a parte que realmente precisa
  // ficar livre e passa o número aqui. 0/undefined = sem reserva.
  dockReservedWidth?: number;
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
        {fitKey !== undefined && <AutoFit fitKey={fitKey} reservedLeft={dockReservedWidth ?? 0} />}
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

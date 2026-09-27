import { IconArrowsMaximize, IconArrowsMinimize } from "@tabler/icons-react";
import {
  Background,
  ControlButton,
  Controls,
  MiniMap,
  Panel,
  ReactFlow,
  useReactFlow,
  type Edge,
  type Node,
  type NodeTypes,
} from "@xyflow/react";
import type { Padding } from "@xyflow/system";
import { useEffect, useState, type DragEvent, type ReactNode } from "react";
import "./flow.css";

const FIT_MARGIN = 90; // respiro (px) além da área reservada pro dock/overlay.

// reservedFitOptions monta o `fitViewOptions` com padding ASSIMÉTRICO
// reservando `reservedLeft` px do lado esquerdo (área do dock) — usado
// tanto pelo AutoFit (reenquadre automático) quanto pelo botão nativo de
// fit-view do React Flow (Controls), que por padrão IGNORA o fitViewOptions
// passado ao <ReactFlow> raiz — Controls tem o PRÓPRIO prop fitViewOptions,
// separado (achado do Vitor: clicar no botão do canto inferior esquerdo
// enquadrava sem considerar a área reservada pro dock). `margin` é
// parametrizável (default FIT_MARGIN) — o canvas grande de Event Rules usa
// os 90px de sempre, mas um canvas compacto (ex.: a prévia mini de
// achievements) precisa de bem menos respiro, senão os nodes ficam
// pequenos demais pra ler (achado do Vitor, spec 057).
// reservedRight (spec 094): o mesmo para a direita, onde flutua o overlay
// (painel de detalhe); sem isso a última coluna do grafo ficava atrás dele.
// maxZoom (spec 094): o Event Rules deixa o fit ampliar um grafo pequeno
// além de 100%; os outros canvas mantêm o teto de 1.
type FitReserve = { left: number; right: number; margin: number; maxZoom: number };

function reservedFitOptions({ left, right, margin, maxZoom }: FitReserve): { padding: Padding; maxZoom: number } {
  return {
    padding: {
      left: `${left + margin}px`,
      right: `${right + margin}px`,
      top: `${margin}px`,
      bottom: `${margin}px`,
    },
    maxZoom,
  };
}

// AutoFit re-enquadra o grafo quando `fitKey` muda (nó rascunho adicionado,
// troca de arquétipo, resize do canvas) — o fitView do ReactFlow só roda no
// mount. Filho de <ReactFlow>, então tem acesso ao store interno.
// Também re-enquadra quando a área livre muda (spec 094): lista recolhida,
// detalhe expandido, tela cheia.
function AutoFit({ fitKey, reserve, full }: { fitKey: unknown; reserve: FitReserve; full: boolean }) {
  const rf = useReactFlow();
  const { left, right, margin, maxZoom } = reserve;
  useEffect(() => {
    const t = setTimeout(() => rf.fitView({ ...reservedFitOptions({ left, right, margin, maxZoom }), duration: 250 }), 50);
    return () => clearTimeout(t);
  }, [fitKey, left, right, margin, maxZoom, full, rf]);
  return null;
}

// Cor de cada tipo de nó no MiniMap (mesma paleta dos headers em flow.css).
const MINIMAP_COLOR: Record<string, string> = {
  fact: "#4dabf7",
  rule: "#20c997",
  eventType: "#b197fc",
  stat: "#ffa94d",
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
  dock,
  dockReservedWidth,
  overlay,
  overlayReservedWidth,
  fitKey,
  height,
  fitMargin,
  fitMaxZoom = 1,
  fullscreenToggle = false,
  interactive = true,
  minimap = true,
  scrollZoom,
  emptyState,
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
  // overlayReservedWidth (spec 094): largura (px) que o fit reserva à
  // direita para o overlay. Só o chamador sabe se o overlay está expandido
  // (e tapando nós) ou recolhido; 0/undefined = sem reserva.
  overlayReservedWidth?: number;
  // fitKey: qualquer mudança re-enquadra o grafo (ver AutoFit).
  fitKey?: unknown;
  // height sobrescreve os 580px do CSS — número (px) ou expressão CSS
  // (ex.: "calc(100vh - 340px)" pro canvas ocupar a altura da aba, spec 047).
  height?: number | string;
  // fitMargin (achado do Vitor, spec 057): respiro do fitView em px — default
  // FIT_MARGIN (90, calibrado pro canvas grande de Event Rules). Um canvas
  // compacto (ex.: a prévia mini de achievements) passa um valor menor, senão
  // o respiro fixo consome proporcionalmente demais da área e os nodes
  // encolhem pra caber.
  fitMargin?: number;
  // fitMaxZoom (spec 094): teto do zoom do enquadramento automático e do
  // botão de fit. Default 1 (os canvas de sempre); o Event Rules passa 1.5
  // para um grafo pequeno ocupar o espaço em vez de ficar ilhado no meio.
  fitMaxZoom?: number;
  // fullscreenToggle (spec 094): botão "Tela cheia" nos controles; o canvas
  // passa a cobrir a janela inteira, e Esc (ou o botão) volta.
  fullscreenToggle?: boolean;
  // interactive=false (achado do Vitor, pipeline): desliga pan/zoom/arrastar
  // nós e some com os botões de zoom/fit — só clicar/selecionar continua
  // funcionando (onNodeClick/onPaneClick inalterados). Default true preserva
  // o comportamento de sempre nos outros canvas (Event Rules, prévia mini).
  interactive?: boolean;
  // minimap=false (pedido do Vitor, pipeline): tira o minimapa inteiro — não
  // faz sentido num canvas só-leitura sem pan/zoom pra navegar. Default true
  // preserva o comportamento de sempre nos outros canvas.
  minimap?: boolean;
  // scrollZoom=false (pedido do Vitor, prévia de achievements): destrava o
  // scroll do MOUSE sobre o canvas — sem isso, o wheel sobre a área do
  // ReactFlow sempre zoom (zoomOnScroll) e sempre chama preventDefault()
  // (preventScrolling), mesmo com interactive=true, então rolar a página com
  // o mouse em cima do mini-canvas expandido "trava" e zoom ao invés de
  // rolar. Continua sendo possível mudar o zoom pelos botões do Controls e
  // pelo minimapa (esses dependem só de `interactive`, não deste prop).
  // Default = `interactive` (preserva o comportamento de sempre no canvas
  // grande de Event Rules e no Pipeline).
  scrollZoom?: boolean;
  // emptyState (achado, revisão Event Rules): mensagem central, só quando
  // nodes.length === 0 — o Event Rules fica deliberadamente vazio até uma
  // regra ser focada (spec 055), mas isso deixava o canvas em branco sem
  // nenhuma pista pro usuário na primeira visita. Opt-in (só quem passa o
  // prop usa) — o Pipeline (sempre populado) e a prévia mini de achievements
  // (sempre focada) não precisam disso.
  emptyState?: ReactNode;
}) {
  const wheelZooms = scrollZoom ?? interactive;
  const [full, setFull] = useState(false);
  useEffect(() => {
    if (!full) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setFull(false);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [full]);
  const reserve: FitReserve = {
    left: dockReservedWidth ?? 0,
    right: overlayReservedWidth ?? 0,
    margin: fitMargin ?? FIT_MARGIN,
    maxZoom: fitMaxZoom,
  };
  return (
    <div
      className={full ? "flow-wrap flow-wrap--full" : "flow-wrap"}
      style={height && !full ? { height, minHeight: 460 } : undefined}
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
        nodesDraggable={interactive}
        panOnDrag={interactive}
        zoomOnScroll={wheelZooms}
        zoomOnPinch={interactive}
        zoomOnDoubleClick={interactive}
        // preventScrolling (achado do Vitor, pipeline): o ReactFlow chama
        // event.preventDefault() em QUALQUER wheel sobre o canvas por padrão,
        // mesmo com zoomOnScroll desligado — sem isso, um canvas read-only
        // (interactive=false) trava o scroll da página quando o mouse passa
        // por cima dele. Segue `wheelZooms` (não `interactive`): a prévia de
        // achievements quer manter zoom via Controls/minimapa (interactive
        // continua true) mas SEM capturar o wheel — só `scrollZoom=false`
        // desliga isso.
        preventScrolling={wheelZooms}
        edgesFocusable={false}
        deleteKeyCode={null}
        onNodeClick={onNodeClick ? (_, n) => onNodeClick(n.id) : undefined}
        onPaneClick={onPaneClick}
        proOptions={{ hideAttribution: true }}
      >
        <Background color="#232c3a" gap={22} />
        {fitKey !== undefined && <AutoFit fitKey={fitKey} reserve={reserve} full={full} />}
        {dock && <Panel position="top-left">{dock}</Panel>}
        {overlay && <Panel position="top-right">{overlay}</Panel>}
        {emptyState && nodes.length === 0 && (
          <Panel position="top-center" style={{ top: "50%", transform: "translateY(-50%)", pointerEvents: "none" }}>
            {emptyState}
          </Panel>
        )}
        {interactive && (
          <Controls showInteractive={false} fitViewOptions={reservedFitOptions(reserve)}>
            {fullscreenToggle && (
              <ControlButton
                onClick={() => setFull((f) => !f)}
                title={full ? "Sair da tela cheia (Esc)" : "Tela cheia"}
                aria-label={full ? "Sair da tela cheia" : "Tela cheia"}
              >
                {full ? <IconArrowsMinimize size={14} /> : <IconArrowsMaximize size={14} />}
              </ControlButton>
            )}
          </Controls>
        )}
        {minimap && (
          <MiniMap
            pannable={interactive}
            zoomable={interactive}
            nodeColor={(n) => MINIMAP_COLOR[n.type ?? ""] ?? "#8b949e"}
            nodeStrokeWidth={2}
            style={{ width: 140, height: 96 }}
          />
        )}
      </ReactFlow>
    </div>
  );
}

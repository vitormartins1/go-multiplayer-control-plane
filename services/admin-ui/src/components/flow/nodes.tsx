import { Badge } from "@mantine/core";
import { IconBolt, IconFilter, IconGitBranch, IconTrophy, IconWorld } from "@tabler/icons-react";
import { Handle, Position, type NodeProps } from "@xyflow/react";
import type {
  AchievementNodeData,
  EventTypeNodeData,
  FactNodeData,
  RuleNodeData,
} from "../../lib/rulesGraph";

// Nós custom do kit de fluxo (spec 043 R7): componentes React puros — recebem
// tudo via node.data, não conhecem API nem página. O TopologyGraph atual usa
// os nós DEFAULT do React Flow (por isso a cara simples); aqui cada nó é um
// card no padrão do exemplo call-center do Foblex (referência de UX, ADR-0028).

export function FactNode({ data }: NodeProps) {
  const d = data as unknown as FactNodeData;
  return (
    <div className="flow-card flow-card--fact">
      <div className="flow-card__header">
        <IconFilter size={12} /> Fact
      </div>
      <div className="flow-card__body">
        <div className="flow-card__row">
          <span className="flow-card__key">{d.factKey} =</span>
          <span>{d.factValue}</span>
        </div>
      </div>
      <Handle type="source" position={Position.Right} isConnectable={false} />
    </div>
  );
}

export function RuleNode({ data }: NodeProps) {
  const { rule, draft } = data as unknown as RuleNodeData;
  const global = rule.scope === "global";
  const pairs = Object.entries(rule.match);
  return (
    <div className={`flow-card ${global ? "flow-card--rule-global" : "flow-card--rule-game"}`}>
      <div className="flow-card__header">
        {global ? <IconWorld size={12} /> : <IconGitBranch size={12} />}
        {draft ? "Nova regra" : global ? "Regra global" : "Regra do jogo"}
        <Badge size="xs" variant="outline" color={draft ? "yellow" : global ? "gray" : "teal"} ml="auto">
          {draft ? "rascunho" : global ? "read-only" : "editável"}
        </Badge>
      </div>
      <div className="flow-card__body">
        {pairs.length === 0 ? (
          <div className="flow-card__row">
            <span className="flow-card__key">(sempre bate)</span>
          </div>
        ) : (
          pairs.map(([k, v]) => (
            <div key={k} className="flow-card__row">
              <span className="flow-card__key">{k} =</span>
              <span>{v}</span>
            </div>
          ))
        )}
      </div>
      <div className="flow-card__footer">
        emite {rule.eventTypes.length} event type{rule.eventTypes.length === 1 ? "" : "s"}
      </div>
      <Handle type="target" position={Position.Left} isConnectable={false} />
      <Handle type="source" position={Position.Right} isConnectable={false} />
    </div>
  );
}

export function EventTypeNode({ data }: NodeProps) {
  const d = data as unknown as EventTypeNodeData;
  return (
    <div className="flow-card flow-card--type">
      <div className="flow-card__header">
        <IconBolt size={12} /> Event type
      </div>
      <div className="flow-card__body">
        <div className="flow-card__row">
          <span>{d.eventType}</span>
        </div>
      </div>
      <Handle type="target" position={Position.Left} isConnectable={false} />
      {d.listeners > 0 && <Handle type="source" position={Position.Right} isConnectable={false} />}
    </div>
  );
}

export function AchievementNode({ data }: NodeProps) {
  const d = data as unknown as AchievementNodeData;
  return (
    <div className="flow-card flow-card--ach">
      <div className="flow-card__header">
        <IconTrophy size={12} /> Achievement
      </div>
      <div className="flow-card__body">
        <div>{d.name}</div>
        <div className="flow-card__row">
          <span className="flow-card__key">target:</span>
          <span>{d.target}</span>
        </div>
      </div>
      <Handle type="target" position={Position.Left} isConnectable={false} />
    </div>
  );
}

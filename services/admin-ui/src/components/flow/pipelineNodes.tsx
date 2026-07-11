import { Badge } from "@mantine/core";
import { IconChartPie, IconDeviceGamepad2, IconMail, IconTrophy, IconChartBar } from "@tabler/icons-react";
import { Handle, Position, type NodeProps } from "@xyflow/react";
import type {
  PipelineConsumerData,
  PipelineKindData,
  PipelineModeData,
} from "../../lib/pipelineGraph";

// Nós da vista de pipeline (spec 046) — mesmos princípios dos nós de regra:
// componentes puros, dados via node.data, estilo do kit (flow.css).

export function PipelineModeNode({ data }: NodeProps) {
  const d = data as unknown as PipelineModeData;
  const ArchIcon = d.archetypeIcon;
  return (
    <div className="flow-card flow-card--fact">
      <div className="flow-card__header">
        <IconDeviceGamepad2 size={12} /> Modo
        <Badge size="xs" variant="light" color="gray" leftSection={<ArchIcon size={11} />} ml="auto">
          {d.archetypeLabel}
        </Badge>
      </div>
      <div className="flow-card__body">
        <div className="flow-card__row">
          <span>{d.modeId}</span>
        </div>
        <div className="flow-card__row">
          <span className="flow-card__key">
            {d.stats.length > 0 ? `stats: ${d.stats.join(", ")}` : "sem stats declaradas"}
          </span>
        </div>
      </div>
      <Handle type="source" position={Position.Right} isConnectable={false} />
    </div>
  );
}

export function PipelineKindNode({ data }: NodeProps) {
  const d = data as unknown as PipelineKindData;
  return (
    <div className="flow-card flow-card--type">
      <div className="flow-card__header">
        <IconMail size={12} /> Envelope
      </div>
      <div className="flow-card__body">
        <div className="flow-card__row">
          <span>{d.kind}</span>
        </div>
      </div>
      <Handle type="target" position={Position.Left} isConnectable={false} />
      <Handle type="source" position={Position.Right} isConnectable={false} />
    </div>
  );
}

const CONSUMER_ICON = {
  ach: <IconTrophy size={12} />,
  lb: <IconChartBar size={12} />,
  an: <IconChartPie size={12} />,
} as const;

export function PipelineConsumerNode({ data }: NodeProps) {
  const d = data as unknown as PipelineConsumerData;
  return (
    <div className={`flow-card flow-card--consumer-${d.variant}`}>
      <div className="flow-card__header">
        {CONSUMER_ICON[d.variant]} {d.title}
      </div>
      <div className="flow-card__body">
        {d.lines.map((l) => (
          <div key={l} className="flow-card__row">
            <span>{l}</span>
          </div>
        ))}
      </div>
      <Handle type="target" position={Position.Left} isConnectable={false} />
    </div>
  );
}

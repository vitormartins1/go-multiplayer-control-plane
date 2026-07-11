import {
  ActionIcon,
  Autocomplete,
  Badge,
  Button,
  Group,
  Popover,
  Select,
  Text,
  UnstyledButton,
} from "@mantine/core";
import {
  IconAlertTriangle,
  IconBan,
  IconBolt,
  IconChartBar,
  IconCheck,
  IconDeviceGamepad2,
  IconFilter,
  IconGitBranch,
  IconPencil,
  IconPlus,
  IconTrash,
  IconTrophy,
  IconWorld,
  IconX,
} from "@tabler/icons-react";
import { Handle, Position, type NodeProps } from "@xyflow/react";
import { useState } from "react";
import { condLabel, OP_OPTIONS, OP_SYMBOL, type MatchOp, type MatchRow } from "../../lib/matchCond";
import {
  STAT_PREFIX,
  type AchievementNodeData,
  type EventTypeNodeData,
  type RuleNodeData,
  type SectionLabelData,
  type SourceNodeData,
} from "../../lib/rulesGraph";

// Nós do editor de event rules (specs 043–048): componentes React puros, dado
// via node.data. Os nós de RASCUNHO têm formulário DENTRO do nó (spec 048):
// inputs com className="nodrag" (não disparam drag do canvas) e o nó só
// arrasta pelo header (dragHandle) — padrão oficial de custom nodes.

// --- Rótulo de seção da coluna de fontes (spec 049): não interativo. ---

export function SectionLabelNode({ data }: NodeProps) {
  const d = data as unknown as SectionLabelData;
  return <div className="flow-section-label">{d.label}</div>;
}

// --- Fontes (camada 1): um nó por CHAVE; condições ficam nas edges. ---

export function SourceNode({ data }: NodeProps) {
  const d = data as unknown as SourceNodeData;
  const isStat = d.kind === "stat";
  const display = isStat ? d.sourceKey.slice(STAT_PREFIX.length) : d.sourceKey;
  return (
    <div className={`flow-card ${isStat ? "flow-card--stat" : "flow-card--fact"}`}>
      <div className="flow-card__header">
        {isStat ? <IconChartBar size={12} /> : d.kind === "mode" ? <IconDeviceGamepad2 size={12} /> : <IconFilter size={12} />}
        {isStat ? "Stat" : d.kind === "mode" ? "Modo" : "Fact"}
      </div>
      <div className="flow-card__body">
        <div className="flow-card__row">
          <span>{display}</span>
        </div>
        {d.drift && (
          <div className="flow-card__row flow-card__warn">
            <IconAlertTriangle size={11} /> não declarada nos modos
          </div>
        )}
      </div>
      <Handle type="source" position={Position.Right} isConnectable={false} />
    </div>
  );
}

// --- Regras (camada 2). ---

export function RuleNode({ data }: NodeProps) {
  const { rule, onEdit } = data as unknown as RuleNodeData;
  const global = rule.scope === "global";
  const suppressed = global && rule.suppressed;
  const pairs = Object.entries(rule.match);
  return (
    <div
      className={`flow-card ${global ? "flow-card--rule-global" : "flow-card--rule-game"} ${
        suppressed ? "flow-card--suppressed" : ""
      }`}
    >
      <div className="flow-card__header">
        {global ? <IconWorld size={12} /> : <IconGitBranch size={12} />}
        {global ? "Regra global" : "Regra do jogo"}
        {global || !onEdit ? (
          <Badge
            size="xs"
            variant="outline"
            color={suppressed ? "orange" : global ? "gray" : "teal"}
            leftSection={suppressed ? <IconBan size={10} /> : undefined}
            ml="auto"
          >
            {suppressed ? "suprimida" : global ? "read-only" : "editável"}
          </Badge>
        ) : (
          // hover troca o badge pela ação (spec 050) — swap via CSS.
          <span className="flow-editswap nodrag">
            <Badge size="xs" variant="outline" color="teal" className="flow-editswap__badge">
              editável
            </Badge>
            <UnstyledButton
              className="flow-editswap__btn"
              onClick={(e) => {
                e.stopPropagation(); // não seleciona o nó
                onEdit();
              }}
              aria-label="Editar regra"
            >
              <IconPencil size={10} /> editar
            </UnstyledButton>
          </span>
        )}
      </div>
      <div className="flow-card__body">
        {pairs.length === 0 ? (
          <div className="flow-card__row">
            <span className="flow-card__key">(sempre bate)</span>
          </div>
        ) : (
          pairs.map(([k, v]) => (
            <div key={k} className="flow-card__row">
              <span>{condLabel(k, v)}</span>
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

// --- Rascunho da regra (spec 048): edição DENTRO do nó. ---

export type RuleDraftData = {
  mode: "create" | "edit"; // edição reusa o MESMO form (spec 050)
  rows: MatchRow[];
  keySuggestions: string[];
  valueSuggestions: (row: MatchRow) => string[];
  error?: string | null;
  saving?: boolean;
  onRow: (i: number, patch: Partial<MatchRow>) => void;
  onRemoveRow: (i: number) => void;
  onAddRow: () => void;
  onSave: () => void;
  onCancel: () => void;
  // archetypePicker (spec 058): só presente em criação (mode "create") num
  // jogo com mais de um arquétipo — undefined = não renderiza nada (jogo de
  // 1 arquétipo só, ou edição, onde o arquétipo é imutável, spec 016).
  archetypePicker?: {
    value: string;
    options: Array<{ value: string; label: string }>;
    onChange: (v: string) => void;
  };
};

// inputs do form in-canvas com a identidade do card (spec 050) — sem o visual
// "form Mantine" cru que destoava do canvas.
const FLOW_INPUT = { input: "flow-input" };

// OpChip: operador como chip clicável com popover compacto — economiza a
// largura que um Select ocuparia dentro do nó.
function OpChip({ op, onChange }: { op: MatchOp; onChange: (op: MatchOp) => void }) {
  const [opened, setOpened] = useState(false);
  return (
    <Popover opened={opened} onChange={setOpened} position="bottom" withinPortal>
      <Popover.Target>
        <UnstyledButton className="nodrag flow-opchip" onClick={() => setOpened((o) => !o)} aria-label="Operador">
          {OP_SYMBOL[op]}
        </UnstyledButton>
      </Popover.Target>
      <Popover.Dropdown p={4}>
        <Group gap={2}>
          {OP_OPTIONS.map((o) => (
            <Button
              key={o.value}
              size="compact-xs"
              variant={o.value === op ? "filled" : "subtle"}
              onClick={() => {
                onChange(o.value);
                setOpened(false);
              }}
            >
              {o.label}
            </Button>
          ))}
        </Group>
      </Popover.Dropdown>
    </Popover>
  );
}

export function RuleDraftNode({ data }: NodeProps) {
  const d = data as unknown as RuleDraftData;
  if (!d.rows) return null; // data ainda não injetada pela página
  const editing = d.mode === "edit";
  return (
    <div className="flow-card flow-card--rule-game flow-card--editing">
      <div className="flow-card__header" style={{ cursor: "grab" }}>
        {editing ? <IconPencil size={12} /> : <IconGitBranch size={12} />}
        {editing ? "Editar regra" : "Nova regra"}
        <Group gap={4} ml="auto" className="nodrag">
          <Button size="compact-xs" variant="subtle" color="gray" leftSection={<IconX size={12} />} onClick={d.onCancel}>
            Cancelar
          </Button>
          <Button size="compact-xs" variant="light" leftSection={<IconCheck size={12} />} loading={d.saving} onClick={d.onSave}>
            Salvar
          </Button>
        </Group>
      </div>
      <div className="flow-card__body nodrag">
        {!editing && d.archetypePicker && (
          <Select
            size="xs"
            classNames={FLOW_INPUT}
            data={d.archetypePicker.options}
            value={d.archetypePicker.value}
            onChange={(v) => v && d.archetypePicker!.onChange(v)}
            allowDeselect={false}
            comboboxProps={{ withinPortal: true }}
          />
        )}
        {d.rows.map((row, i) => (
          <Group key={i} gap={4} wrap="nowrap">
            <Autocomplete
              size="xs"
              placeholder="chave"
              classNames={FLOW_INPUT}
              data={d.keySuggestions}
              value={row.key}
              onChange={(v) => d.onRow(i, { key: v })}
              style={{ flex: 1, minWidth: 0 }}
              comboboxProps={{ withinPortal: true }}
            />
            <OpChip op={row.op} onChange={(op) => d.onRow(i, { op })} />
            <Autocomplete
              size="xs"
              placeholder={row.op === "in" ? "a, b" : row.op === "eq" || row.op === "ne" ? "valor" : "nº"}
              classNames={FLOW_INPUT}
              data={d.valueSuggestions(row)}
              value={row.value}
              onChange={(v) => d.onRow(i, { value: v })}
              style={{ flex: 1, minWidth: 0 }}
              comboboxProps={{ withinPortal: true }}
            />
            <ActionIcon
              variant="subtle"
              color="critical"
              size="xs"
              disabled={d.rows.length === 1}
              onClick={() => d.onRemoveRow(i)}
              aria-label="Remover condição"
            >
              <IconTrash size={12} />
            </ActionIcon>
          </Group>
        ))}
        <Group gap={4}>
          <ActionIcon variant="subtle" color="gray" size="xs" onClick={d.onAddRow} aria-label="Adicionar condição" title="+ condição">
            <IconPlus size={12} />
          </ActionIcon>
          <Text size="xs" c="dimmed">
            condição · clique numa fonte pra preencher
          </Text>
        </Group>
        {d.error && (
          <Text size="xs" c="critical">
            {d.error}
          </Text>
        )}
      </div>
      <Handle type="target" position={Position.Left} isConnectable={false} />
      <Handle type="source" position={Position.Right} isConnectable={false} />
    </div>
  );
}

// --- Event types (camada 3). ---

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

export type TypeDraftData = {
  value: string;
  suggestions: string[];
  onChange: (v: string) => void;
};

// EventTypeDraftNode nasce sempre junto do form (spec 052) — exatamente um
// por rascunho, do início ao fim; sem botão de fechar (deixar vazio ao
// salvar é válido, o save filtra valores vazios).
export function EventTypeDraftNode({ data }: NodeProps) {
  const d = data as unknown as TypeDraftData;
  if (!d.onChange) return null; // data ainda não injetada
  return (
    <div className="flow-card flow-card--type flow-card--editing-type">
      <div className="flow-card__header">
        <IconBolt size={12} /> Event type novo
      </div>
      <div className="flow-card__body nodrag">
        <Autocomplete
          size="xs"
          placeholder="nome_do_type"
          classNames={FLOW_INPUT}
          data={d.suggestions}
          value={d.value}
          onChange={d.onChange}
          comboboxProps={{ withinPortal: true }}
        />
      </div>
      <Handle type="target" position={Position.Left} isConnectable={false} />
    </div>
  );
}

// --- Achievements (camada 4). ---

export function AchievementNode({ data }: NodeProps) {
  const d = data as unknown as AchievementNodeData;
  return (
    <div className="flow-card flow-card--ach">
      <div className="flow-card__header">
        <IconTrophy size={12} /> Achievement
        {d.statKey && (
          <Badge size="xs" variant="light" color="orange" ml="auto">
            métrica
          </Badge>
        )}
      </div>
      <div className="flow-card__body">
        <div>{d.name}</div>
        {/* Métrica: o critério mora no nó (spec 049) — a edge stat→achievement
            é discreta por padrão e o dado não pode depender dela. */}
        {d.statKey ? (
          <div className="flow-card__row">
            <span>
              {d.statKey} {d.statMode === "per_match" ? "por partida" : "acum."} ≥ {d.target}
            </span>
          </div>
        ) : (
          <div className="flow-card__row">
            <span className="flow-card__key">target:</span>
            <span>{d.target}</span>
          </div>
        )}
      </div>
      <Handle type="target" position={Position.Left} isConnectable={false} />
    </div>
  );
}

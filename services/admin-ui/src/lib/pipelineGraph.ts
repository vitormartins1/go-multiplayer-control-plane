import type { Edge, Node } from "@xyflow/react";
import type { CatalogAchievement, EventRule, MultiplayerMode } from "./api";
import { ARCHETYPE_META } from "./archetypeMeta";

// pipelineGraph monta a vista de pipeline do fan-out (spec 046): modos ->
// envelope (kind) -> consumers, com o que cada consumer produz DESTE jogo.
// Read-only e derivado 100% das APIs — os únicos mapeamentos fixos abaixo
// espelham contratos reais do backend (kind por arquétipo, ADR-0023; dispatch
// por consumer, código Go dos consumers).

export type PipelineModeData = {
  modeId: string;
  archetypeLabel: string;
  archetypeColor: string;
  stats: string[];
};
export type PipelineKindData = { kind: string };
export type PipelineConsumerData = {
  title: string;
  variant: "ach" | "lb" | "an";
  lines: string[];
  hint: string;
};

const KIND_BY_ARCHETYPE: Record<string, string> = {
  queue_arena: "win_loss_draw",
  host_world_dropin: "participation_event",
  battle_royale: "placement",
  coop_mission: "collective",
};

// Espelha o dispatch real dos consumers Go: o leaderboard DESCARTA
// participation_event (kind sem ranking) — a edge ausente é proposital
// (decisão de fronteira, adendo ADR-0028).
const CONSUMERS_BY_KIND: Record<string, Array<"ach" | "lb" | "an">> = {
  win_loss_draw: ["ach", "lb", "an"],
  placement: ["ach", "lb", "an"],
  collective: ["ach", "lb", "an"],
  participation_event: ["ach", "an"],
};

const COL_X = [0, 340, 640];
const GAP_Y = 18;

const modeId_ = (m: string) => `m:${m}`;
const kindId = (k: string) => `k:${k}`;
const consumerId = (c: string) => `c:${c}`;

export function buildPipelineGraph(
  modes: MultiplayerMode[],
  rules: EventRule[],
  catalog: CatalogAchievement[],
): { nodes: Node[]; edges: Edge[] } {
  const stats = (m: MultiplayerMode): string[] =>
    "scoringStats" in m.params ? m.params.scoringStats : [];

  // Camada 1 — modos do jogo.
  const modeNodes = modes.map((m) => ({
    id: modeId_(m.modeId),
    type: "pipelineMode",
    data: {
      modeId: m.modeId,
      archetypeLabel: ARCHETYPE_META[m.archetype].label,
      archetypeColor: ARCHETYPE_META[m.archetype].color,
      stats: stats(m),
    } satisfies PipelineModeData,
    h: 86,
  }));

  // Camada 2 — kinds de envelope realmente produzidos pelos modos do jogo.
  const kinds = [...new Set(modes.map((m) => KIND_BY_ARCHETYPE[m.archetype]))];
  const kindNodes = kinds.map((k) => ({
    id: kindId(k),
    type: "pipelineKind",
    data: { kind: k } satisfies PipelineKindData,
    h: 58,
  }));

  // Camada 3 — consumers com o que produzem deste jogo.
  const evAch = catalog.filter((a) => !a.statKey && a.eventType).length;
  const statAch = catalog.filter((a) => a.statKey).length;
  const eventTypes = new Set(rules.flatMap((r) => r.eventTypes)).size;
  const allStats = [...new Set(modes.flatMap(stats))];
  const consumers: Array<{ id: "ach" | "lb" | "an"; data: PipelineConsumerData }> = [
    {
      id: "ach",
      data: {
        title: "achievements",
        variant: "ach",
        lines: [
          `${rules.length} regras → ${eventTypes} event types`,
          `${evAch} achievements por evento · ${statAch} por stat`,
        ],
        hint: "Traduz o resultado via event rules (aba Event Rules) e aplica stats direto nos critérios por métrica.",
      },
    },
    {
      id: "lb",
      data: {
        title: "leaderboard",
        variant: "lb",
        lines:
          allStats.length > 0
            ? [`board score + ${allStats.length} board${allStats.length === 1 ? "" : "s"} por stat`, allStats.join(", ")]
            : ["board score", "sem stats declaradas nos modos"],
        hint: "Traduz o envelope pra números (rankings). Não conhece event rules — participation_event é descartado (nada a ranquear).",
      },
    },
    {
      id: "an",
      data: {
        title: "analytics",
        variant: "an",
        lines: ["agrega o envelope BRUTO", "pré-política (auditoria)"],
        hint: "Vê o dado cru, antes de qualquer regra — a história não muda quando as regras mudam.",
      },
    },
  ];
  const consumerNodes = consumers.map((c) => ({
    id: consumerId(c.id),
    type: "pipelineConsumer",
    data: c.data,
    h: 96,
  }));

  // Posições: colunas fixas, empilhamento simples, centralizado na mais alta.
  const columns = [modeNodes, kindNodes, consumerNodes];
  const heights = columns.map((col) => col.reduce((s, n) => s + n.h + GAP_Y, -GAP_Y));
  const maxH = Math.max(...heights, 0);
  const nodes: Node[] = columns.flatMap((col, li) => {
    let y = (maxH - heights[li]) / 2;
    return col.map((n) => {
      const node: Node = {
        id: n.id,
        type: n.type,
        position: { x: COL_X[li], y },
        data: n.data,
        connectable: false,
      };
      y += n.h + GAP_Y;
      return node;
    });
  });

  const edges: Edge[] = [];
  for (const m of modes) {
    edges.push({
      id: `${modeId_(m.modeId)}->${kindId(KIND_BY_ARCHETYPE[m.archetype])}`,
      source: modeId_(m.modeId),
      target: kindId(KIND_BY_ARCHETYPE[m.archetype]),
    });
  }
  for (const k of kinds) {
    for (const c of CONSUMERS_BY_KIND[k]) {
      edges.push({ id: `${kindId(k)}->${consumerId(c)}`, source: kindId(k), target: consumerId(c) });
    }
  }
  return { nodes, edges };
}

import type { Edge, Node } from "@xyflow/react";
import type { CatalogAchievement, EventRule } from "./api";

// rulesGraph monta o grafo de 4 camadas do editor visual de event rules
// (spec 043 / ADR-0028): facts -> regras -> event types -> achievements.
// Função pura (mesmo espírito do EvaluateRules no Go): entra o dado da API,
// sai nodes/edges com posição — nenhum estado de canvas persiste, o desenho é
// sempre derivado das regras.

export type FactNodeData = { factKey: string; factValue: string };
export type RuleNodeData = { rule: EventRule; draft?: boolean };
export type EventTypeNodeData = { eventType: string; listeners: number };
export type AchievementNodeData = { name: string; target: number; eventType: string };

export const factId = (k: string, v: string) => `f:${k}=${v}`;
export const ruleId = (r: EventRule) => `r:${r.id}`;
export const typeId = (et: string) => `t:${et}`;
export const achId = (id: string) => `a:${id}`;

// Larguras/alturas ESTIMADAS por tipo de nó — o layout roda antes do render,
// então não dá pra medir o DOM; estimativa conservadora pelo conteúdo.
const SIZE = {
  fact: { w: 190, h: 62 },
  eventType: { w: 210, h: 62 },
  achievement: { w: 230, h: 82 },
};
const ruleHeight = (r: EventRule) => 46 + 22 * Math.max(1, Object.entries(r.match).length) + 32;

// X fixo por camada (grafo estritamente em camadas — por isso layout próprio
// em vez de dagre/elkjs, ver plan 043) e respiro vertical entre nós.
const COL_X = [0, 280, 620, 920];
const GAP_Y = 18;

type Meta = { id: string; type: string; data: Record<string, unknown>; h: number; layer: number };

// stackColumn empilha os nós de uma camada em Y e devolve a altura total.
function stackColumn(items: Meta[], heights: Map<string, number>): number {
  let y = 0;
  for (const m of items) {
    heights.set(m.id, y);
    y += m.h + GAP_Y;
  }
  return Math.max(0, y - GAP_Y);
}

// barycenter ordena uma camada pela média dos índices dos vizinhos na camada
// anterior — heurística clássica pra reduzir cruzamento de edges.
function barycenter(items: Meta[], neighborIdx: Map<string, number[]>): Meta[] {
  return [...items].sort((a, b) => {
    const avg = (id: string) => {
      const idx = neighborIdx.get(id);
      if (!idx || idx.length === 0) return Number.MAX_SAFE_INTEGER; // sem vizinho: vai pro fim
      return idx.reduce((s, i) => s + i, 0) / idx.length;
    };
    return avg(a.id) - avg(b.id);
  });
}

// DraftRule é a regra EM CRIAÇÃO (drawer aberto, spec 044): entra no layout
// como pseudo-regra "__draft__" — mesmas ligações das regras reais, só que
// marcada como rascunho (nós/edges ghost) até o admin salvar.
export type DraftRule = { match: Record<string, string>; eventTypes: string[] };

export const DRAFT_RULE_ID = "r:__draft__";

export function buildRulesGraph(
  rules: EventRule[],
  catalog: CatalogAchievement[],
  draft?: DraftRule,
): { nodes: Node[]; edges: Edge[] } {
  // Facts/types que existem nas regras REAIS — o que só aparecer via draft é
  // ghost (some sem resíduo no cancelar, R3 da spec).
  const realFacts = new Set(rules.flatMap((r) => Object.entries(r.match).map(([k, v]) => factId(k, v))));
  const realTypes = new Set(rules.flatMap((r) => r.eventTypes.map((et) => typeId(et))));
  const allRules: EventRule[] = draft
    ? [
        ...rules,
        { id: "__draft__", scope: "game", archetype: rules[0]?.archetype ?? "queue_arena", ...draft },
      ]
    : rules;

  // Camada 2 — regras. Ordem: RASCUNHO primeiro (fica no topo da coluna,
  // visível mesmo com o drawer aberto cobrindo o pé da janela), depois
  // globais, depois do jogo (estável).
  const rank = (r: EventRule) => (r.id === "__draft__" ? 0 : r.scope === "global" ? 1 : 2);
  const ruleMetas: Meta[] = [...allRules]
    .sort((a, b) => rank(a) - rank(b))
    .map((r) => ({
      id: ruleId(r),
      type: "rule",
      data: { rule: r, draft: r.id === "__draft__" || undefined } satisfies RuleNodeData,
      h: ruleHeight(r),
      layer: 1,
    }));
  const ruleIdx = new Map(ruleMetas.map((m, i) => [m.id, i]));

  // Camada 1 — facts distintos (pares chave=valor dos matches).
  const factUses = new Map<string, { key: string; value: string; rules: number[] }>();
  for (const r of allRules) {
    for (const [k, v] of Object.entries(r.match)) {
      const id = factId(k, v);
      const u = factUses.get(id) ?? { key: k, value: v, rules: [] };
      u.rules.push(ruleIdx.get(ruleId(r))!);
      factUses.set(id, u);
    }
  }
  let factMetas: Meta[] = [...factUses.entries()].map(([id, u]) => ({
    id,
    type: "fact",
    data: { factKey: u.key, factValue: u.value } satisfies FactNodeData,
    h: SIZE.fact.h,
    layer: 0,
  }));
  factMetas = barycenter(
    factMetas,
    new Map([...factUses.entries()].map(([id, u]) => [id, u.rules])),
  );

  // Camada 3 — event types distintos emitidos pelas regras.
  const typeSources = new Map<string, number[]>();
  for (const r of allRules) {
    for (const et of r.eventTypes) {
      const list = typeSources.get(et) ?? [];
      list.push(ruleIdx.get(ruleId(r))!);
      typeSources.set(et, list);
    }
  }
  // Camada 4 — achievements do jogo que escutam um event type do grafo
  // (stat-criteria fica de fora: statKey não é disparado por event type).
  const listeners = new Map<string, CatalogAchievement[]>();
  for (const a of catalog) {
    if (!a.statKey && a.eventType && typeSources.has(a.eventType)) {
      listeners.set(a.eventType, [...(listeners.get(a.eventType) ?? []), a]);
    }
  }

  let typeMetas: Meta[] = [...typeSources.keys()].map((et) => ({
    id: typeId(et),
    type: "eventType",
    data: { eventType: et, listeners: listeners.get(et)?.length ?? 0 } satisfies EventTypeNodeData,
    h: SIZE.eventType.h,
    layer: 2,
  }));
  typeMetas = barycenter(
    typeMetas,
    new Map([...typeSources.entries()].map(([et, idx]) => [typeId(et), idx])),
  );
  const typeIdx = new Map(typeMetas.map((m, i) => [m.id, i]));

  let achMetas: Meta[] = [];
  const achNeighbor = new Map<string, number[]>();
  for (const [et, achs] of listeners) {
    for (const a of achs) {
      achMetas.push({
        id: achId(a.id),
        type: "achievement",
        data: { name: a.name, target: a.target, eventType: et } satisfies AchievementNodeData,
        h: SIZE.achievement.h,
        layer: 3,
      });
      achNeighbor.set(achId(a.id), [typeIdx.get(typeId(et))!]);
    }
  }
  achMetas = barycenter(achMetas, achNeighbor);

  // Posições: empilha cada coluna e centraliza verticalmente pela mais alta.
  const columns = [factMetas, ruleMetas, typeMetas, achMetas];
  const yByID = new Map<string, number>();
  const colHeights = columns.map((c) => stackColumn(c, yByID));
  const maxH = Math.max(...colHeights, 0);

  // Ghost = só existe por causa do draft (regra rascunho + facts/types
  // inéditos dela) — estilo tracejado, some sem resíduo no cancelar.
  const isGhost = (id: string) =>
    id === DRAFT_RULE_ID ||
    (id.startsWith("f:") && !realFacts.has(id)) ||
    (id.startsWith("t:") && !realTypes.has(id));

  const nodes: Node[] = columns.flatMap((col, li) =>
    col.map((m) => ({
      id: m.id,
      type: m.type,
      position: { x: COL_X[li], y: (yByID.get(m.id) ?? 0) + (maxH - colHeights[li]) / 2 },
      data: m.data,
      connectable: false,
      className: isGhost(m.id) ? "is-draft" : undefined,
    })),
  );

  const edges: Edge[] = [];
  const push = (source: string, target: string) =>
    edges.push({
      id: `${source}->${target}`,
      source,
      target,
      className: isGhost(source) || isGhost(target) ? "is-draft-edge" : undefined,
    });
  for (const [fid, u] of factUses) {
    for (const ri of u.rules) push(fid, ruleMetas[ri].id);
  }
  for (const [et, idx] of typeSources) {
    for (const ri of idx) push(ruleMetas[ri].id, typeId(et));
  }
  for (const [et, achs] of listeners) {
    for (const a of achs) push(typeId(et), achId(a.id));
  }

  return { nodes, edges };
}

// pathHighlight devolve os nós/edges alcançáveis a partir do selecionado, nos
// DOIS sentidos (upstream + downstream) — é o caminho completo fact -> regra
// -> event type -> achievement que a seleção destaca (R2 da spec).
export function pathHighlight(
  edges: Edge[],
  selectedId: string,
): { nodeIds: Set<string>; edgeIds: Set<string> } {
  const fwd = new Map<string, Edge[]>();
  const rev = new Map<string, Edge[]>();
  for (const e of edges) {
    fwd.set(e.source, [...(fwd.get(e.source) ?? []), e]);
    rev.set(e.target, [...(rev.get(e.target) ?? []), e]);
  }
  const nodeIds = new Set<string>([selectedId]);
  const edgeIds = new Set<string>();
  const walk = (adj: Map<string, Edge[]>, next: (e: Edge) => string) => {
    const queue = [selectedId];
    while (queue.length > 0) {
      const cur = queue.pop()!;
      for (const e of adj.get(cur) ?? []) {
        edgeIds.add(e.id);
        const n = next(e);
        if (!nodeIds.has(n)) {
          nodeIds.add(n);
          queue.push(n);
        }
      }
    }
  };
  walk(fwd, (e) => e.target);
  walk(rev, (e) => e.source);
  return { nodeIds, edgeIds };
}

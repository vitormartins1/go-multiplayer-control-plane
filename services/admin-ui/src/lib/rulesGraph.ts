import type { Edge, Node } from "@xyflow/react";
import type { CatalogAchievement, EventRule } from "./api";
import { condShort, type MatchValue } from "./matchCond";

// rulesGraph monta o grafo do editor de event rules (specs 043–048 /
// ADR-0028) no modelo de FONTES UNIFICADAS (spec 048): a camada 1 tem um nó
// por CHAVE (facts estruturais, `mode`, stats) e as CONDIÇÕES viram labels
// das edges fonte→regra. O critério por métrica dos achievements também é
// edge (stat→achievement) — métrica alimenta conquista DIRETO, sem regra.
// Função pura: entra o dado da API, sai nodes/edges com posição.

export type SourceKind = "structural" | "mode" | "stat";
export type SourceNodeData = {
  sourceKey: string; // a chave usada nas regras ("outcome", "mode", "stat.kills")
  kind: SourceKind;
  drift?: boolean; // stat usada por achievement mas não declarada em nenhum modo
  conds: string[]; // condições em que aparece (painel)
};
export type RuleNodeData = {
  rule: EventRule;
  redundantTypes?: string[];
  onEdit?: () => void; // só regras do jogo, fora de criação/edição (spec 050)
};
export type EventTypeNodeData = { eventType: string; listeners: number };
export type AchievementNodeData = {
  name: string;
  target: number;
  eventType?: string;
  statKey?: string;
  statMode?: string;
};
export type StatInfo = { key: string; declared: boolean };
export type SectionLabelData = { label: string };

export const srcId = (k: string) => `src:${k}`;
export const ruleId = (r: EventRule) => `r:${r.id}`;
export const typeId = (et: string) => `t:${et}`;
export const achId = (id: string) => `a:${id}`;

export const DRAFT_RULE_ID = "r:__draft__";
export const draftTypeId = (ghostId: string) => `t:__new__:${ghostId}`;

// FactStatPrefix espelha o prefixo do backend (spec achievements/015).
export const STAT_PREFIX = "stat.";

// Draft da regra em criação (spec 048, in-canvas): o build só precisa do
// suficiente pra POSICIONAR e LIGAR os ghosts — o conteúdo editável (linhas,
// callbacks) é injetado pela página por cima dos nós devolvidos.
export type DraftGraphInput = {
  conds: Array<{ key: string; label: string }>; // condições VÁLIDAS (edges ao vivo)
  linkedTypes: string[]; // event types existentes ligados
  newTypes: Array<{ id: string; value: string }>; // ghosts editáveis
  rows: number; // nº de linhas no editor (estimar altura)
  // Edição (spec 050): id da regra em edição — o nó DELA vira o form, na
  // posição original (não nasce nó extra), e o match/types persistidos são
  // substituídos pelas edges ao vivo do form. undefined = criação.
  editingRuleID?: string;
};

// Alturas ESTIMADAS pro layout (os nós renderizam depois do posicionamento).
// CALIBRADAS com offsetHeight real via Playwright (spec 050 T5) — se os cards
// mudarem de conteúdo/CSS, remedir, senão os gaps da coluna desandam.
const SIZE = {
  source: { h: 64 }, // drift +15 (linha do warning)
  eventType: { h: 64 },
  achievement: { h: 86 },
};
const ruleHeight = (r: EventRule) => 69 + 22 * Math.max(1, Object.entries(r.match).length);
// RECALIBRADA (spec 052): ações Cancelar/Salvar migraram pro header e o
// footer sumiu — offsetHeight real: 104px c/ 1 linha, 136px c/ 2 (medido
// via Playwright, 72 + 32/linha). Exportada: repackVisible (spec 056) precisa
// da mesma fórmula pra saber a altura real do node ruleDraft.
export const draftHeight = (rows: number) => 72 + 32 * Math.max(1, rows);
// Altura fixa do ghost de event type em edição (calibrada, spec 048/050) —
// ao contrário do ruleDraft, não cresce com conteúdo.
const EVENT_TYPE_DRAFT_H = 71;

const COL_X = [0, 300, 640, 920];
const GAP_Y = 18;
// DRAFT_COL_SHIFT: a caixinha em edição/criação (.flow-card--editing) tem
// 340px, contra os 260px máximos de uma regra normal — sem isso ela cola na
// coluna de event types (o gap de 340px entre COL_X[1]/COL_X[2] já é
// calibrado só pro card normal). Desloca as colunas à DIREITA da regra
// (event types + achievements) pra restaurar o mesmo respiro que existe fora
// do draft (achado do Vitor testando os fluxos de criar/editar).
const DRAFT_COL_SHIFT = 80;
const LABEL_H = 14;

// Ordem fixa dos grupos da coluna de fontes (spec 049).
const SOURCE_GROUPS: Array<{ kind: SourceKind; label: string }> = [
  { kind: "structural", label: "facts da partida" },
  { kind: "mode", label: "modo" },
  { kind: "stat", label: "métricas" },
];

type Meta = { id: string; type: string; data: Record<string, unknown>; h: number; gapBefore?: number };

function stackColumn(items: Meta[], yByID: Map<string, number>): number {
  let y = 0;
  for (const m of items) {
    y += m.gapBefore ?? 0;
    yByID.set(m.id, y);
    y += m.h + GAP_Y;
  }
  return Math.max(0, y - GAP_Y);
}

// barycenter reduz cruzamentos ordenando pela média dos índices dos vizinhos.
function barycenter(items: Meta[], neighborIdx: Map<string, number[]>): Meta[] {
  return [...items].sort((a, b) => {
    const avg = (id: string) => {
      const idx = neighborIdx.get(id);
      if (!idx || idx.length === 0) return Number.MAX_SAFE_INTEGER;
      return idx.reduce((s, i) => s + i, 0) / idx.length;
    };
    return avg(a.id) - avg(b.id);
  });
}

function sourceKind(key: string): SourceKind {
  if (key === "mode") return "mode";
  if (key.startsWith(STAT_PREFIX)) return "stat";
  return "structural";
}

export function buildRulesGraph(
  rules: EventRule[],
  catalog: CatalogAchievement[],
  opts?: { stats?: StatInfo[]; draft?: DraftGraphInput },
): { nodes: Node[]; edges: Edge[] } {
  const stats = opts?.stats;
  const draft = opts?.draft;

  // --- Camada 2: regras (rascunho primeiro — fica no topo, visível). ---
  // formNodeID: o nó-formulário ativo — o rascunho (criação) OU o nó da
  // própria regra em edição (spec 050).
  const formNodeID = draft ? (draft.editingRuleID ? `r:${draft.editingRuleID}` : DRAFT_RULE_ID) : null;
  const editing = (r: EventRule) => draft?.editingRuleID === r.id;
  const ruleMetas: Meta[] = [];
  if (draft && !draft.editingRuleID) {
    ruleMetas.push({ id: DRAFT_RULE_ID, type: "ruleDraft", data: {}, h: draftHeight(draft.rows) });
  }
  const rank = (r: EventRule) => (r.scope === "global" ? 0 : 1);
  for (const r of [...rules].sort((a, b) => rank(a) - rank(b))) {
    if (editing(r)) {
      // vira o form NO LUGAR — mesma posição relativa na coluna.
      ruleMetas.push({ id: ruleId(r), type: "ruleDraft", data: {}, h: draftHeight(draft!.rows) });
    } else {
      ruleMetas.push({ id: ruleId(r), type: "rule", data: { rule: r } satisfies RuleNodeData, h: ruleHeight(r) });
    }
  }
  const ruleIdx = new Map(ruleMetas.map((m, i) => [m.id, i]));

  // --- Camada 1: FONTES (um nó por chave). ---
  // chave -> {kind, conds usadas, índices de regras vizinhas}
  const sources = new Map<string, { kind: SourceKind; conds: string[]; rules: number[]; drift?: boolean }>();
  const touchSource = (key: string) => {
    let s = sources.get(key);
    if (!s) {
      s = { kind: sourceKind(key), conds: [], rules: [] };
      sources.set(key, s);
    }
    return s;
  };
  const srcRuleEdges: Array<{ key: string; ruleID: string; label: string }> = [];
  for (const r of rules) {
    if (editing(r)) continue; // o match persistido é substituído pelo do form
    for (const [key, v] of Object.entries(r.match)) {
      const s = touchSource(key);
      const label = condShort(v as MatchValue);
      if (!s.conds.includes(label)) s.conds.push(label);
      s.rules.push(ruleIdx.get(ruleId(r))!);
      srcRuleEdges.push({ key, ruleID: ruleId(r), label });
    }
  }
  if (draft) {
    for (const c of draft.conds) {
      const s = touchSource(c.key);
      s.rules.push(ruleIdx.get(formNodeID!)!);
      srcRuleEdges.push({ key: c.key, ruleID: formNodeID!, label: c.label });
    }
  }
  // stats declaradas/drift entram como fonte mesmo sem uso em regra (toggle
  // Stats OFF = opts.stats undefined: só sobram as usadas em regras).
  const metricAchs = new Map<string, CatalogAchievement[]>(); // statKey -> achievements por métrica
  if (stats) {
    for (const s of stats) {
      const src = touchSource(STAT_PREFIX + s.key);
      src.drift = !s.declared;
    }
    const known = new Set(stats.map((s) => s.key));
    for (const a of catalog) {
      if (a.statKey && known.has(a.statKey)) {
        metricAchs.set(a.statKey, [...(metricAchs.get(a.statKey) ?? []), a]);
      }
    }
  }
  // `mode` é fonte universal (todo envelope carrega) — sempre disponível.
  touchSource("mode");

  const allSourceMetas: Meta[] = [...sources.entries()].map(([key, s]) => ({
    id: srcId(key),
    type: "source",
    data: { sourceKey: key, kind: s.kind, drift: s.drift || undefined, conds: s.conds } satisfies SourceNodeData,
    h: SIZE.source.h + (s.drift ? 15 : 0),
  }));
  const sourceNeighbor = new Map([...sources.entries()].map(([key, s]) => [srcId(key), s.rules]));
  // Agrupamento fixo por kind (spec 049): facts → mode → stats, com rótulo de
  // seção não interativo por grupo. O baricentro só reordena DENTRO do grupo —
  // é o que impede o intercalado fact/stat que confundia a coluna.
  const sourceMetas: Meta[] = [];
  for (const g of SOURCE_GROUPS) {
    const items = barycenter(
      allSourceMetas.filter((m) => (m.data as SourceNodeData).kind === g.kind),
      sourceNeighbor,
    );
    if (items.length === 0) continue;
    sourceMetas.push({
      id: `lbl:${g.kind}`,
      type: "sectionLabel",
      data: { label: g.label } satisfies SectionLabelData,
      h: LABEL_H,
      gapBefore: sourceMetas.length > 0 ? GAP_Y : 0,
    });
    sourceMetas.push(...items);
  }

  // --- Camada 3: event types. ---
  const typeSources = new Map<string, number[]>();
  const touchType = (et: string, ruleI: number) => {
    typeSources.set(et, [...(typeSources.get(et) ?? []), ruleI]);
  };
  for (const r of rules) {
    if (editing(r)) continue; // os types persistidos são substituídos pelos do form
    for (const et of r.eventTypes) touchType(et, ruleIdx.get(ruleId(r))!);
  }
  if (draft) {
    for (const et of draft.linkedTypes) touchType(et, ruleIdx.get(formNodeID!)!);
  }
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
  }));
  // ghosts de type NOVO (editáveis) — data injetada pela página.
  if (draft) {
    for (const g of draft.newTypes) {
      typeMetas.push({ id: draftTypeId(g.id), type: "eventTypeDraft", data: {}, h: EVENT_TYPE_DRAFT_H });
    }
  }
  const typeNeighbor = new Map([...typeSources.entries()].map(([et, idx]) => [typeId(et), idx]));
  // Ghosts entram no baricentro como vizinhos do form (spec 050) — sem isso
  // caem no MAX_SAFE_INTEGER e afundam pro fim da coluna, longe do rascunho.
  if (draft) {
    for (const g of draft.newTypes) typeNeighbor.set(draftTypeId(g.id), [ruleIdx.get(formNodeID!)!]);
  }
  typeMetas = barycenter(typeMetas, typeNeighbor);
  const typeIdx = new Map(typeMetas.map((m, i) => [m.id, i]));

  // --- Camada 4: achievements (por evento + por métrica). ---
  let achMetas: Meta[] = [];
  const achNeighbor = new Map<string, number[]>();
  for (const [et, achs] of listeners) {
    for (const a of achs) {
      achMetas.push({
        id: achId(a.id),
        type: "achievement",
        data: { name: a.name, target: a.target, eventType: et } satisfies AchievementNodeData,
        h: SIZE.achievement.h,
      });
      achNeighbor.set(achId(a.id), [typeIdx.get(typeId(et))!]);
    }
  }
  for (const [key, achs] of metricAchs) {
    for (const a of achs) {
      achMetas.push({
        id: achId(a.id),
        type: "achievement",
        data: { name: a.name, target: a.target, statKey: key, statMode: a.statMode } satisfies AchievementNodeData,
        h: SIZE.achievement.h,
      });
      achNeighbor.set(achId(a.id), [typeMetas.length]); // depois dos types (embaixo)
    }
  }
  achMetas = barycenter(achMetas, achNeighbor);

  // --- Posições: colunas fixas, centralizadas pela mais alta. ---
  const columns = [sourceMetas, ruleMetas, typeMetas, achMetas];
  const yByID = new Map<string, number>();
  const colHeights = columns.map((c) => stackColumn(c, yByID));
  const maxH = Math.max(...colHeights, 0);

  const isGhost = (id: string) => id === formNodeID || id.startsWith("t:__new__:");
  const colX = (li: number) => COL_X[li] + (draft && li >= 2 ? DRAFT_COL_SHIFT : 0);

  const nodes: Node[] = columns.flatMap((col, li) =>
    col.map((m) => ({
      id: m.id,
      type: m.type,
      position: { x: colX(li), y: (yByID.get(m.id) ?? 0) + (maxH - colHeights[li]) / 2 },
      data: m.data,
      connectable: false,
      selectable: m.type !== "sectionLabel",
      draggable: m.type !== "sectionLabel",
      className: isGhost(m.id) ? "is-draft" : undefined,
      // draft: arrasta só pelo header (inputs dentro do nó, padrão nodrag).
      dragHandle: m.type === "ruleDraft" ? ".flow-card__header" : undefined,
    })),
  );

  const edges: Edge[] = [];
  const push = (source: string, target: string, label?: string, cls?: string) =>
    edges.push({
      id: `${source}->${target}${label ? `|${label}` : ""}`,
      source,
      target,
      label,
      className: isGhost(source) || isGhost(target) ? "is-draft-edge" : cls,
      labelStyle: { fill: "#9aa7b4", fontSize: 10, fontFamily: "ui-monospace, monospace" },
      labelBgStyle: { fill: "#0f141b", fillOpacity: 0.9 },
      labelBgPadding: [3, 2] as [number, number],
    });

  for (const e of srcRuleEdges) push(srcId(e.key), e.ruleID, e.label);
  // Edges regra→type de uma regra global suprimida (spec 017/051, V2b)
  // renderizam tracejadas — a regra continua existindo (achável pra
  // restaurar), só não está no caminho ativo desta avaliação.
  for (const [et, idx] of typeSources) {
    for (const ri of idx) {
      const meta = ruleMetas[ri];
      const suppressed = meta.type === "rule" && (meta.data as RuleNodeData).rule.suppressed;
      push(meta.id, typeId(et), undefined, suppressed ? "is-suppressed-edge" : undefined);
    }
  }
  if (draft) {
    for (const g of draft.newTypes) push(formNodeID!, draftTypeId(g.id));
  }
  for (const [et, achs] of listeners) {
    for (const a of achs) push(typeId(et), achId(a.id));
  }
  // Edges de métrica (stat→achievement, coluna 1→4) cruzam o canvas inteiro —
  // nascem discretas (spec 049): o critério mora no NÓ do achievement e a
  // edge só acende quando a fonte/achievement entra no caminho destacado.
  for (const [key, achs] of metricAchs) {
    for (const a of achs) {
      push(
        srcId(STAT_PREFIX + key),
        achId(a.id),
        `${a.statMode === "per_match" ? "por partida" : "acumulada"} ≥ ${a.target}`,
        "is-metric-edge",
      );
    }
  }

  return { nodes, edges };
}

// pathHighlight devolve nós/edges alcançáveis a partir do selecionado nos
// DOIS sentidos (upstream + downstream) — o caminho completo que a seleção
// destaca.
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

// siblingRuleIds devolve os ids de nós `rule`/`ruleDraft` que têm uma edge
// direta pra qualquer node em focusNodeIds (tipicamente o resultado de
// pathHighlight de uma regra) — usado pelo toggle "regras relacionadas"
// (spec 055): acha quem mais escuta a mesma source/eventType da regra
// focada. Um salto só, por design — não expande recursivamente (senão
// reintroduz a poluição visual que o foco existe pra evitar).
export function siblingRuleIds(edges: Edge[], focusNodeIds: Set<string>): Set<string> {
  const siblings = new Set<string>();
  for (const e of edges) {
    if (focusNodeIds.has(e.source) && e.target.startsWith("r:")) siblings.add(e.target);
    if (focusNodeIds.has(e.target) && e.source.startsWith("r:")) siblings.add(e.source);
  }
  return siblings;
}

// heightOf estima a altura de um node já posicionado (mesmas fórmulas de
// buildRulesGraph) — usado só por repackVisible. `overrides` cobre os casos
// dinâmicos que essa função sozinha não sabe calcular: no momento em que
// repackVisible roda (spec 055/056), o node ruleDraft ainda tem `data: {}`
// (o draftData real só é injetado depois, no useMemo do canvas) — sem
// override, `heightOf` não tem como saber quantas linhas de match o rascunho
// tem. Sem entrada no mapa, cai no mínimo de 1 linha (nunca fica pior que a
// estimativa antiga, só imprecisa quando há 2+ linhas).
function heightOf(n: Node, overrides?: Map<string, number>): number {
  const override = overrides?.get(n.id);
  if (override !== undefined) return override;
  switch (n.type) {
    case "source": {
      const d = n.data as unknown as SourceNodeData;
      return SIZE.source.h + (d.drift ? 15 : 0);
    }
    case "rule":
      return ruleHeight((n.data as unknown as RuleNodeData).rule);
    case "ruleDraft":
      return draftHeight(1);
    case "eventType":
      return SIZE.eventType.h;
    case "eventTypeDraft":
      return EVENT_TYPE_DRAFT_H;
    case "achievement":
      return SIZE.achievement.h;
    case "sectionLabel":
      return LABEL_H;
    default:
      return SIZE.achievement.h;
  }
}

// repackVisible recalcula as posições Y de um subconjunto já filtrado de nodes
// (spec 055 follow-up): buildRulesGraph posiciona pensando no arquétipo
// INTEIRO; filtrar por foco sem recalcular deixa os nodes visíveis nas
// coordenadas originais, com vãos enormes onde ficavam os nodes escondidos.
// Reempacota cada coluna (agrupada por x, que já reflete a camada) do zero,
// preservando a ordem relativa (Y original) e recentralizando pela coluna
// mais alta — mesmo efeito visual do build completo, só que pro subconjunto.
// `heightOverrides` (spec 056 follow-up): altura real do node ruleDraft em
// edição/criação (draftHeight(rows) do caller) — sem isso a coluna vizinha
// (event type novo, sources) não acompanha o card crescendo/encolhendo
// conforme o usuário adiciona/remove condições.
export function repackVisible(nodes: Node[], heightOverrides?: Map<string, number>): Node[] {
  const byX = new Map<number, Node[]>();
  for (const n of nodes) byX.set(n.position.x, [...(byX.get(n.position.x) ?? []), n]);

  const stacked = new Map<number, { col: Node[]; y: Map<string, number>; h: number }>();
  for (const [x, col] of byX) {
    const sorted = [...col].sort((a, b) => a.position.y - b.position.y);
    const y = new Map<string, number>();
    let cursor = 0;
    for (const n of sorted) {
      y.set(n.id, cursor);
      cursor += heightOf(n, heightOverrides) + GAP_Y;
    }
    stacked.set(x, { col: sorted, y, h: Math.max(0, cursor - GAP_Y) });
  }
  const maxH = Math.max(0, ...[...stacked.values()].map((s) => s.h));

  const out: Node[] = [];
  for (const [x, { col, y, h }] of stacked) {
    const offset = (maxH - h) / 2;
    for (const n of col) out.push({ ...n, position: { x, y: (y.get(n.id) ?? 0) + offset } });
  }
  return out;
}

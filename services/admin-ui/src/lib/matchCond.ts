// Condições do match das event rules (spec achievements/014, V2a do ADR-0028).
// O valor de cada chave é string (igualdade, formato clássico) OU um objeto
// {op, value} — este módulo concentra tipos, rótulos e a montagem a partir do
// form (pra tabela, canvas e painéis renderizarem igual).

export type MatchOp = "eq" | "ne" | "lt" | "lte" | "gt" | "gte" | "in";
export type MatchCondObj = { op: Exclude<MatchOp, "eq">; value: number | string | string[] };
export type MatchValue = string | MatchCondObj;

export const OP_SYMBOL: Record<MatchOp, string> = {
  eq: "=",
  ne: "≠",
  lt: "<",
  lte: "≤",
  gt: ">",
  gte: "≥",
  in: "in",
};

export const OP_OPTIONS: Array<{ value: MatchOp; label: string }> = [
  { value: "eq", label: "=" },
  { value: "ne", label: "≠" },
  { value: "lt", label: "<" },
  { value: "lte", label: "≤" },
  { value: "gt", label: ">" },
  { value: "gte", label: "≥" },
  { value: "in", label: "in" },
];

const NUMERIC_OPS: ReadonlySet<MatchOp> = new Set(["lt", "lte", "gt", "gte"]);

// MatchRow é uma linha do EDITOR de condições (spec 048, edição in-canvas):
// chave + operador + valor cru (texto). buildMatchValue converte pro wire.
export type MatchRow = { key: string; op: MatchOp; value: string };

export const EMPTY_ROW: MatchRow = { key: "", op: "eq", value: "" };

export function condOp(v: MatchValue): MatchOp {
  return typeof v === "string" ? "eq" : v.op;
}

// condLabel formata a condição pra exibição: "role = invader",
// "placement ≤ 10", "mode in [ranked, arena]".
export function condLabel(key: string, v: MatchValue): string {
  if (typeof v === "string") return `${key} = ${v}`;
  if (v.op === "in") return `${key} in [${(v.value as string[]).join(", ")}]`;
  return `${key} ${OP_SYMBOL[v.op]} ${v.value}`;
}

// condShort formata só operador+valor (a chave vira o nó de FONTE e a
// condição vira o label da edge, spec 048): "= win", "≤ 10", "in [a, b]".
export function condShort(v: MatchValue): string {
  if (typeof v === "string") return `= ${v}`;
  if (v.op === "in") return `in [${(v.value as string[]).join(", ")}]`;
  return `${OP_SYMBOL[v.op]} ${v.value}`;
}

// condRaw devolve o valor como string editável (pré-preencher o form a partir
// de uma condição existente): "invader", "10", "ranked, arena".
export function condRaw(v: MatchValue): string {
  if (typeof v === "string") return v;
  return Array.isArray(v.value) ? v.value.join(", ") : String(v.value);
}

// buildMatchValue monta o valor da API a partir do form (op + texto cru).
// null = inválido pro operador (numérico não-número; in sem itens) — o form
// bloqueia antes de mandar pro backend (que valida de novo, estrito).
export function buildMatchValue(op: MatchOp, raw: string): MatchValue | null {
  const t = raw.trim();
  if (t === "") return null;
  if (op === "eq") return t;
  if (op === "ne") return { op, value: t };
  if (op === "in") {
    const list = t.split(",").map((s) => s.trim()).filter(Boolean);
    return list.length > 0 ? { op, value: list } : null;
  }
  if (NUMERIC_OPS.has(op)) {
    const n = Number(t);
    return Number.isFinite(n) ? { op: op as MatchCondObj["op"], value: n } : null;
  }
  return null;
}

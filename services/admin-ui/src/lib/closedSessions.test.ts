import { describe, expect, it } from "vitest";
import { MAX_HISTORY_TRIES, awaitHistory, goneIds, mergeClosed } from "./closedSessions";

const s = (sessionId: string) => ({ sessionId });
const ids = (rows: { sessionId: string }[]) => rows.map((x) => x.sessionId);

describe("goneIds", () => {
  it("sem abertas antes, nada fechou", () => {
    expect(goneIds(new Set(), [s("a")])).toEqual([]);
  });
  it("a mesma lista, nada fechou", () => {
    expect(goneIds(new Set(["a", "b"]), [s("a"), s("b")])).toEqual([]);
  });
  it("uma nova aberta não é fechamento", () => {
    expect(goneIds(new Set(["a"]), [s("a"), s("b")])).toEqual([]);
  });
  it("uma aberta que sumiu fechou", () => {
    expect(goneIds(new Set(["a", "b"]), [s("b")])).toEqual(["a"]);
  });
  it("todas sumiram", () => {
    expect(goneIds(new Set(["a"]), [])).toEqual(["a"]);
  });
});

describe("awaitHistory", () => {
  it("sem nada fechando, não recarrega", () => {
    const r = awaitHistory(new Map(), [], new Set());
    expect(r.reload).toBe(false);
    expect(r.awaiting.size).toBe(0);
  });
  it("uma sessão fechou: recarrega", () => {
    const r = awaitHistory(new Map(), ["x"], new Set());
    expect(r.reload).toBe(true);
    expect(r.awaiting.get("x")).toBe(1);
  });
  it("ainda não gravada no histórico: recarrega de novo no ciclo seguinte", () => {
    const r = awaitHistory(new Map([["x", 1]]), [], new Set());
    expect(r.reload).toBe(true);
    expect(r.awaiting.get("x")).toBe(2);
  });
  it("apareceu no histórico: para de esperar", () => {
    const r = awaitHistory(new Map([["x", 1]]), [], new Set(["x"]));
    expect(r.reload).toBe(false);
    expect(r.awaiting.size).toBe(0);
  });
  it("esgotou as tentativas: desiste", () => {
    const r = awaitHistory(new Map([["x", MAX_HISTORY_TRIES]]), [], new Set());
    expect(r.reload).toBe(false);
    expect(r.awaiting.size).toBe(0);
  });
});

describe("mergeClosed", () => {
  it("junta as páginas na ordem", () => {
    expect(ids(mergeClosed([s("a"), s("b")], [s("c")]))).toEqual(["a", "b", "c"]);
  });
  it("a linha empurrada para fora da primeira página continua na lista", () => {
    // página 1 era a,b; "carregar mais" trouxe c,d. x fecha: a nova página 1 é x,a e b sai dela.
    const newFirst = [s("x"), s("a")];
    const oldFirst = [s("a"), s("b")];
    const extra = [s("c"), s("d")];
    expect(ids(mergeClosed(newFirst, oldFirst, extra))).toEqual(["x", "a", "b", "c", "d"]);
  });
  it("não repete sessão entre páginas", () => {
    expect(ids(mergeClosed([s("new"), s("a"), s("b")], [s("b"), s("c")]))).toEqual(["new", "a", "b", "c"]);
  });
});

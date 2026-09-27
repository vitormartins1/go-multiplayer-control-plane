import { describe, expect, it } from "vitest";
import { mergeClosed, someClosed } from "./closedSessions";

const s = (sessionId: string) => ({ sessionId });

describe("someClosed", () => {
  it("sem abertas antes, nada fechou", () => {
    expect(someClosed(new Set(), [s("a")])).toBe(false);
  });
  it("a mesma lista, nada fechou", () => {
    expect(someClosed(new Set(["a", "b"]), [s("a"), s("b")])).toBe(false);
  });
  it("uma nova aberta não é fechamento", () => {
    expect(someClosed(new Set(["a"]), [s("a"), s("b")])).toBe(false);
  });
  it("uma aberta que sumiu fechou", () => {
    expect(someClosed(new Set(["a", "b"]), [s("b")])).toBe(true);
  });
  it("todas sumiram", () => {
    expect(someClosed(new Set(["a"]), [])).toBe(true);
  });
});

describe("mergeClosed", () => {
  it("junta as páginas na ordem", () => {
    expect(mergeClosed([s("a"), s("b")], [s("c")]).map((x) => x.sessionId)).toEqual(["a", "b", "c"]);
  });
  it("não repete a linha empurrada para a página seguinte", () => {
    expect(mergeClosed([s("new"), s("a"), s("b")], [s("b"), s("c")]).map((x) => x.sessionId)).toEqual([
      "new",
      "a",
      "b",
      "c",
    ]);
  });
});

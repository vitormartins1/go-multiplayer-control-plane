import { describe, expect, it } from "vitest";
import { buildMatchValue, condLabel, condOp, condRaw, condShort } from "./matchCond";

describe("buildMatchValue", () => {
  it("igualdade vira string crua (formato clássico do backend)", () => {
    expect(buildMatchValue("eq", " win ")).toBe("win");
  });

  it("comparadores numéricos exigem número", () => {
    expect(buildMatchValue("lte", "10")).toEqual({ op: "lte", value: 10 });
    expect(buildMatchValue("gt", "dez")).toBeNull();
  });

  it("in quebra por vírgula e descarta vazios", () => {
    expect(buildMatchValue("in", "ranked, arena,")).toEqual({ op: "in", value: ["ranked", "arena"] });
    expect(buildMatchValue("in", " , ")).toBeNull();
  });

  it("valor vazio é inválido para qualquer operador", () => {
    expect(buildMatchValue("ne", "  ")).toBeNull();
  });
});

describe("formatação de condições", () => {
  it("rótulos da tabela e do canvas batem com o wire", () => {
    expect(condLabel("role", "invader")).toBe("role = invader");
    expect(condLabel("placement", { op: "lte", value: 10 })).toBe("placement ≤ 10");
    expect(condShort({ op: "in", value: ["a", "b"] })).toBe("in [a, b]");
    expect(condOp("win")).toBe("eq");
  });

  it("condRaw ida e volta com buildMatchValue", () => {
    const v = { op: "in" as const, value: ["ranked", "arena"] };
    expect(buildMatchValue("in", condRaw(v))).toEqual(v);
  });
});

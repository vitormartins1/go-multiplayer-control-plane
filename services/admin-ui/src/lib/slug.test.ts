import { describe, expect, it } from "vitest";
import { slugify } from "./slug";

describe("slugify", () => {
  it("remove acentos e troca separadores por hífen (regra do game-registry)", () => {
    expect(slugify("Folclore Souls: Boitatá!")).toBe("folclore-souls-boitata");
    expect(slugify("  --Ágil  ")).toBe("agil");
  });
});

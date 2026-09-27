import { describe, expect, it } from "vitest";
import type { GameSession } from "./api";
import { compareSessions } from "./sessionSort";

const s = (over: Partial<GameSession>): GameSession => ({
  sessionId: "s",
  matchId: "",
  state: "open",
  players: [],
  serverAddress: "",
  openedAt: "2026-09-27T12:00:00Z",
  ...over,
});

describe("compareSessions (admin-ui/092)", () => {
  it("tempo: só openedAt, nos dois sentidos", () => {
    const old = s({ openedAt: "2026-09-27T10:00:00Z" });
    const young = s({ openedAt: "2026-09-27T11:00:00Z" });
    expect(compareSessions(old, young, "time", "asc")).toBeLessThan(0);
    expect(compareSessions(old, young, "time", "desc")).toBeGreaterThan(0);
  });
  it("estado: abertas antes das fechadas", () => {
    expect(compareSessions(s({ state: "open" }), s({ state: "closed" }), "state", "asc")).toBeLessThan(0);
  });
  it("arquétipo desconhecido vai pro fim nos dois sentidos", () => {
    const known = s({ archetype: "queue_arena" });
    const unknown = s({ archetype: "xyz" });
    expect(compareSessions(unknown, known, "archetype", "asc")).toBe(1);
    expect(compareSessions(unknown, known, "archetype", "desc")).toBe(1);
  });
  it("jogo: usa o nome do mapa quando a sessão não traz", () => {
    const games = new Map([
      ["a", { id: "a", name: "Arena", slug: "a", createdAt: "" }],
      ["b", { id: "b", name: "Boitatá", slug: "b", createdAt: "" }],
    ]);
    expect(compareSessions(s({ gameId: "b" }), s({ gameId: "a" }), "game", "asc", games)).toBeGreaterThan(0);
  });
  it("modo vazio conta como default", () => {
    expect(compareSessions(s({ modeId: "" }), s({ modeId: "default" }), "mode", "asc")).toBe(0);
  });
});

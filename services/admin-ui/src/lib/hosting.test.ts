import { describe, expect, it } from "vitest";
import { heartbeatStatus, hostingOf, p2pOnly, shortConnection } from "./hosting";

describe("shortConnection (admin-ui/091)", () => {
  it("mantém o provedor e o fim do SteamID", () => {
    expect(shortConnection("steam:76561198000000042")).toBe("steam:…000042");
  });
  it("não mexe em id curto", () => {
    expect(shortConnection("host-1")).toBe("host-1");
  });
});

describe("p2pOnly (admin-ui/091)", () => {
  it("só quando todos os modos são p2p", () => {
    expect(p2pOnly([{ topology: "p2p" }, { topology: "p2p" }])).toBe(true);
    expect(p2pOnly([{ topology: "p2p" }, { topology: "dedicated" }])).toBe(false);
  });
  it("jogo sem modos não é p2p", () => {
    expect(p2pOnly([])).toBe(false);
    expect(p2pOnly(undefined)).toBe(false);
  });
});

describe("hostingOf (admin-ui/091)", () => {
  it("p2p pela topologia", () => {
    expect(hostingOf({ topology: "p2p", serverAddress: "" })).toBe("p2p");
  });
  it("dedicado pela topologia ou pelo endereço", () => {
    expect(hostingOf({ topology: "dedicated", serverAddress: "" })).toBe("dedicated");
    expect(hostingOf({ serverAddress: "10.0.0.1:7777" })).toBe("dedicated");
  });
  it("desconhecido sem topologia nem endereço (histórico persistido)", () => {
    expect(hostingOf({ serverAddress: "" })).toBe("unknown");
  });
});

describe("heartbeatStatus (session/032)", () => {
  const now = Date.parse("2026-09-27T12:02:00Z");
  it("sem heartbeat", () => {
    expect(heartbeatStatus(undefined, now)).toBe("none");
  });
  it("vivo até 45 s", () => {
    expect(heartbeatStatus("2026-09-27T12:01:15Z", now)).toBe("alive");
  });
  it("atrasado entre 45 s e 90 s", () => {
    expect(heartbeatStatus("2026-09-27T12:00:40Z", now)).toBe("late");
  });
  it("sem sinal depois de 90 s", () => {
    expect(heartbeatStatus("2026-09-27T12:00:00Z", now)).toBe("lost");
  });
});

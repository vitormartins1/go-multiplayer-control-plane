import { describe, expect, it } from "vitest";
import { serverLabel } from "./format";

describe("serverLabel (session/031)", () => {
  it("mostra o endereço do game server em dedicado", () => {
    expect(serverLabel({ serverAddress: "10.0.0.1:7777", topology: "dedicated" })).toBe("10.0.0.1:7777");
  });
  it("mostra P2P · host quando o jogo do host hospeda", () => {
    expect(serverLabel({ serverAddress: "", topology: "p2p" })).toBe("P2P · host");
  });
  it("cai no traço sem servidor e sem topologia (histórico persistido)", () => {
    expect(serverLabel({ serverAddress: "" })).toBe("—");
  });
});

import { describe, expect, it } from "vitest";
import {
  closeReasonMeta,
  participationOutcomeMeta,
  roleMeta,
  sessionStateLabel,
} from "./vocab";

describe("vocab", () => {
  it("cobre os 6 motivos de fechamento do session-service", () => {
    for (const r of ["result", "host_killed", "host_left", "admin", "ttl", "abandoned"]) {
      expect(closeReasonMeta(r).label).not.toBe(r);
    }
    expect(closeReasonMeta("ttl").category).toBe("abandoned");
    expect(closeReasonMeta("host_left").category).toBe("manual");
  });

  it("desconhecido cai no valor cru, sem quebrar", () => {
    expect(closeReasonMeta("novo_motivo").label).toBe("novo_motivo");
    expect(roleMeta("spectator").label).toBe("spectator");
  });

  it("desfecho de falha do host_world é FAIL (o que o backend emite)", () => {
    expect(participationOutcomeMeta("FAIL").label).toBe("Falha");
  });

  it("estado da sessão em português, com o motivo traduzido", () => {
    expect(sessionStateLabel("open")).toBe("ao vivo");
    expect(sessionStateLabel("closed", "host_left")).toBe("fechada · Host saiu");
  });
});

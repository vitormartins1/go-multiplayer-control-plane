import type { FleetStatus } from "./api";

// Localização de uma Fleet Agones (spec 023) — mesmo espírito de lib/status.ts,
// só que com um estado de localização em vez de só saudável/fora.
export type FleetLocation = "offline" | "local" | "pi5" | "unknown";

// fleetLocationOf deriva a localização a partir do FleetStatus real. "unknown"
// é reservado pro chamador (erro de rede) — este helper nunca devolve
// "unknown" a partir de um FleetStatus válido.
export function fleetLocationOf(status: FleetStatus): FleetLocation {
  if (!status.cluster) return "offline";
  if (status.cluster === "local") return "local";
  if (status.cluster === "pi5") return "pi5";
  return "offline"; // rótulo de cluster desconhecido: mais perto de "não sei onde" do que de um local novo
}

export function fleetLocationColor(loc: FleetLocation): string {
  switch (loc) {
    case "local":
      return "nominal";
    case "pi5":
      return "nominal";
    case "offline":
      return "critical";
    default:
      return "gray";
  }
}

export function fleetLocationLabel(loc: FleetLocation): string {
  switch (loc) {
    case "local":
      return "local (mac)";
    case "pi5":
      return "pi 5";
    case "offline":
      return "offline";
    default:
      return "desconhecido";
  }
}

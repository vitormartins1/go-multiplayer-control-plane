// Hospedagem e sinal de vida de um mundo (admin-ui/091, session/031 e 032).
// Regras puras — os componentes só escolhem cor e texto a partir daqui.

export type Hosting = "dedicated" | "p2p" | "unknown";

// hostingOf diz quem hospeda a sessão. Sem topologia (histórico persistido) e
// sem endereço não dá para saber.
export function hostingOf(s: { topology?: string; serverAddress: string }): Hosting {
  if (s.topology === "p2p") return "p2p";
  if (s.topology === "dedicated" || s.serverAddress) return "dedicated";
  return "unknown";
}

// p2pOnly: o jogo tem modos e todos são p2p — não precisa de fleet Agones.
export function p2pOnly(modes: { topology?: string }[] | undefined): boolean {
  return !!modes && modes.length > 0 && modes.every((m) => m.topology === "p2p");
}

// shortConnection encurta o id de conexão do host pelo MEIO: SteamIDs começam
// todos com os mesmos dígitos (7656119…), o que distingue um host é o fim.
export function shortConnection(conn: string): string {
  if (conn.length <= 16) return conn;
  const sep = conn.indexOf(":");
  const prefix = sep >= 0 && sep < 10 ? conn.slice(0, sep + 1) : "";
  return `${prefix}…${conn.slice(-6)}`;
}

export type HeartbeatStatus = "alive" | "late" | "lost" | "none";

// Limiares sobre o intervalo que o session sugere (30 s) e o TTL (90 s):
// até 1,5 intervalo está vivo; até o TTL está atrasado; depois disso o sweeper
// fecha o mundo no próximo minuto.
export const HEARTBEAT_ALIVE_MS = 45_000;
export const HEARTBEAT_TTL_MS = 90_000;

export function heartbeatStatus(lastHeartbeatAt: string | undefined, now: number): HeartbeatStatus {
  if (!lastHeartbeatAt) return "none";
  const age = now - Date.parse(lastHeartbeatAt);
  if (age <= HEARTBEAT_ALIVE_MS) return "alive";
  if (age <= HEARTBEAT_TTL_MS) return "late";
  return "lost";
}

export const HEARTBEAT_META: Record<HeartbeatStatus, { label: string; color: string; help: string }> = {
  alive: { label: "vivo", color: "nominal", help: "Heartbeat em dia." },
  late: { label: "atrasado", color: "yellow", help: "Heartbeat atrasado — o mundo fecha se passar de 90 s sem sinal." },
  lost: { label: "sem sinal", color: "critical", help: "Sem heartbeat há mais de 90 s — o sweeper fecha o mundo como abandonado." },
  none: {
    label: "sem heartbeat",
    color: "gray",
    help: "O jogo não manda heartbeat: o mundo fecha 30 min depois de aberto, jogando ou não.",
  },
};

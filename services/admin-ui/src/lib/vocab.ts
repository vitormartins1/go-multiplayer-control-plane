// Vocabulário do domínio numa fonte só (revisão 2026-09): motivos de
// fechamento, papéis, eventos e desfechos apareciam com rótulos e cores
// diferentes em Analytics, Rooms, Sessions e no detalhe da sessão (e o
// desfecho "falha" do host_world usava FAILURE, que o backend nunca emite —
// o certo é FAIL). Toda tela lê daqui.

type Meta = { label: string; color: string };

const lookup = (table: Record<string, Meta>, key: string | undefined, fallbackColor = "gray"): Meta =>
  (key && table[key]) || { label: key || "—", color: fallbackColor };

// --- Motivo de fechamento (session-service/domain/session.go: CloseReason) ---

export type CloseCategory = "concluded" | "manual" | "abandoned";

// categoria: concluded = a sessão chegou ao fim do jogo (resultado reportado,
// host morto em combate); manual = alguém encerrou (host saiu, operador);
// abandoned = ninguém fechou (servidor caiu, TTL sem report).
const CLOSE_REASONS: Record<string, Meta & { category: CloseCategory }> = {
  result: { label: "Resultado reportado", color: "green.6", category: "concluded" },
  host_killed: { label: "Host morto", color: "grape.5", category: "concluded" },
  host_left: { label: "Host saiu", color: "teal.5", category: "manual" },
  admin: { label: "Encerrada pelo operador", color: "blue.5", category: "manual" },
  ttl: { label: "Expirou (TTL, sem report)", color: "gray.5", category: "abandoned" },
  abandoned: { label: "Servidor caiu", color: "red.6", category: "abandoned" },
};

export const CLOSE_CATEGORY_LABELS: Record<CloseCategory, string> = {
  concluded: "Concluída",
  manual: "Encerrada manualmente",
  abandoned: "Abandonada",
};

export function closeReasonMeta(reason: string | undefined): Meta & { category: CloseCategory } {
  if (reason && CLOSE_REASONS[reason]) return CLOSE_REASONS[reason];
  return { label: reason || "—", color: "gray.4", category: "abandoned" };
}

// --- Estado da sessão (open/closed) ---

export function sessionStateLabel(state: string, closeReason?: string): string {
  if (state === "open") return "ao vivo";
  return closeReason ? `fechada · ${closeReasonMeta(closeReason).label}` : "fechada";
}

// --- Papéis do host_world_dropin ---

export const ROLE_ORDER = ["host", "coop", "invader"];
const ROLES: Record<string, Meta> = {
  host: { label: "Host", color: "blue" },
  coop: { label: "Co-op", color: "teal" },
  invader: { label: "Invasor", color: "red" },
};
export const roleMeta = (role: string | undefined): Meta => lookup(ROLES, role);

// --- Eventos de participação (ReportParticipationEvent) ---

const PARTICIPATION_EVENTS: Record<string, Meta> = {
  JOINED: { label: "Entrou", color: "blue.4" },
  BOSS_DEFEATED: { label: "Boss derrotado", color: "teal.4" },
  HOST_KILLED: { label: "Host morto", color: "grape.4" },
  DIED: { label: "Morreu", color: "orange.4" },
  LEFT: { label: "Saiu", color: "gray.4" },
};
export const participationEventMeta = (event: string | undefined): Meta =>
  lookup(PARTICIPATION_EVENTS, event, "gray.4");

// --- Desfechos ---

// host_world_dropin (por participante): SUCCESS | FAIL | NEUTRAL.
const PARTICIPATION_OUTCOMES: Record<string, Meta> = {
  SUCCESS: { label: "Sucesso", color: "green" },
  FAIL: { label: "Falha", color: "red" },
  NEUTRAL: { label: "Neutro", color: "gray" },
};
export const participationOutcomeMeta = (outcome: string | undefined): Meta =>
  lookup(PARTICIPATION_OUTCOMES, outcome);

// queue_arena (por jogador): win | loss | draw.
const MATCH_OUTCOMES: Record<string, Meta> = {
  win: { label: "Vitória", color: "green" },
  loss: { label: "Derrota", color: "red" },
  draw: { label: "Empate", color: "yellow" },
};
export const matchOutcomeMeta = (outcome: string | undefined): Meta => lookup(MATCH_OUTCOMES, outcome);

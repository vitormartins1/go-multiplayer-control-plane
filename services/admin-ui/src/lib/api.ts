// Cliente da API do admin-bff. Em dev, as chamadas a /api são repassadas pelo
// proxy do Vite ao BFF (:8088). Foco em features de produto — observabilidade
// detalhada fica no Grafana (ADR-0016/0006).

import type { MatchValue } from "./matchCond";

export interface ServiceNode {
  id: string;
  label: string;
  up: boolean | null; // null = desconhecido (Prometheus fora)
}

export interface Edge {
  from: string;
  to: string;
  label: string;
}

export interface Topology {
  nodes: ServiceNode[];
  edges: Edge[];
  sourceAvailable: boolean;
}

export interface Player {
  id: string;
  handle: string;
  createdAt: string;
}

export interface PlayerPage {
  total: number;
  players: Player[];
}

export interface PlayerAchievement {
  id: string;
  name: string;
  description: string;
  target: number;
  progress: number;
  unlocked: boolean;
  unlockedAt?: string;
}

export interface Game {
  id: string;
  slug: string;
  name: string;
  createdAt: string;
  // Capa (003-generated-cover): metadados; a imagem é gerada no cliente a partir
  // da seed. Opcionais p/ compatibilidade com respostas antigas.
  image?: string;
  imageSource?: string;
  generatedImageSeed?: string;
  generatedImageStyle?: string;
  // Modos de multiplayer (ADR-0024): resumo devolvido pelo GET /games/{id}.
  multiplayerModes?: ModeSummary[];
  // Fleet Agones do jogo (ADR-0026); "" = sem fleet cadastrada. Exposto pelo
  // BFF só a partir da spec 059 — o game-registry já devolvia desde a 012.
  fleet?: string;
  // Descrição curta do jogo (spec 013); "" = sem descrição.
  description?: string;
}

// FleetStatus é o snapshot real de localização/saúde de uma Fleet Agones
// (spec 023). cluster="" cobre tanto "sem fleet cadastrada" quanto
// "session-service sem allocator agones ativo" — distinto de cluster
// preenchido com fleetExists=false ("Agones ativo, mas a Fleet não está
// NESTE cluster").
export interface FleetStatus {
  cluster: string;
  fleetExists: boolean;
  desired: number;
  current: number;
  ready: number;
  allocated: number;
}

// Perfis de multiplayer do jogo (ADR-0023). Union discriminada por `archetype`.

export interface QueueArenaParams {
  structure: "ffa" | "teams";
  playerCount: number;
  teamCount: number;
  teamSize: number;
  winCondition: "first_to_score" | "highest_at_time" | "objective" | "elimination";
  scoreTarget: number;
  durationSeconds: number;
  matchStructure: "single" | "best_of_rounds";
  roundsToWin: number;
  allowDraw: boolean;
  scoringStats: string[];
}

export interface HostWorldDropInParams {
  enabledRoles: Array<"coop" | "invader">;
  coopSlots: number;
  invaderSlots: number;
  levelRangeEnabled: boolean;
  weaponTierEnabled: boolean;
  passwordEnabled: boolean;
  invasionEnabled: boolean;
  // scoringStats: achado usando o motor de event rules a fundo (jogo
  // Achievements Test) — host_world_dropin era o único arquétipo sem o
  // campo; fechado junto com achievements/018 (stats no participation_event).
  scoringStats: string[];
}

// battle_royale (ADR-0023): lobby grande + squads; colocação ordinal.
export type RespawnMode = "one_life" | "teammate_revive" | "arena_1v1" | "reboot_timer";

export interface BattleRoyaleParams {
  lobbySize: number;
  teamSize: number;
  respawnMode: RespawnMode;
  scoringStats: string[];
}

// coop_mission (ADR-0023): time pequeno vs. ambiente; resultado coletivo.
export interface CoopMissionParams {
  teamSize: number;
  missions: string[];
  difficulties: string[];
  botfillEnabled: boolean;
  scoringStats: string[];
}

export type MultiplayerProfile =
  | { archetype: "queue_arena"; topology: "dedicated" | "p2p"; params: QueueArenaParams }
  | { archetype: "host_world_dropin"; topology: "dedicated" | "p2p"; params: HostWorldDropInParams }
  | { archetype: "battle_royale"; topology: "dedicated" | "p2p"; params: BattleRoyaleParams }
  | { archetype: "coop_mission"; topology: "dedicated" | "p2p"; params: CoopMissionParams };

// Modos nomeados (ADR-0024): um jogo tem 1..N perfis, cada um com um id estável.
// O corpo do perfil (archetype/topology/params) é o union acima, inalterado.
export type MultiplayerMode = MultiplayerProfile & { modeId: string; displayName?: string };

export interface ModeSummary {
  modeId: string;
  displayName?: string;
  archetype: MultiplayerProfile["archetype"];
  topology?: "dedicated" | "p2p"; // admin-ui/091
}

// Regras do modeId (spec game-registry/009): slug estável, imutável na prática.
export const MODE_ID_RE = /^[a-z][a-z0-9-]{0,31}$/;
export const DEFAULT_MODE_ID = "default";

// Critério por métrica (ADR-0023 A1): statKey preenchido troca o critério de
// evento para stat; statMode é a agregação.
export type StatMode = "cumulative" | "per_match";

export interface CatalogAchievement {
  id: string;
  name: string;
  description: string;
  eventType: string;
  statKey?: string;
  statMode?: StatMode;
  target: number;
  steamApiName?: string; // "API Name" no Steamworks (achievements/019); vazio = não espelha
}

export interface NewAchievement {
  id: string;
  name: string;
  description: string;
  eventType: string;
  statKey?: string;
  statMode?: StatMode;
  target: number;
  steamApiName?: string; // "API Name" no Steamworks (achievements/019); vazio = não espelha
}

export interface ImportResult {
  row: number;
  id: string;
  status: "created" | "duplicate" | "invalid" | "error";
  error?: string;
}

export interface ImportResponse {
  created: number;
  failed: number;
  results: ImportResult[];
}

export interface LeaderEntry {
  rank: number;
  playerId: string;
  score: number;
  // handle (spec 088): resolvido best-effort pelo admin-bff (mesmo padrão de
  // GameSession.players[].handle) — vazio quando o identity não resolveu.
  handle?: string;
}

export interface AchievementProgressEntry {
  playerId: string;
  progress: number;
  unlocked: boolean;
  unlockedAt?: string;
}

// EventRule é uma regra de mapeamento evento->achievement-event-type
// (migration 008 do achievements-service). scope "global" = regra do
// arquétipo (read-only no painel); "game" = override específico deste jogo
// (editável/removível). match varia por arquétipo (role/event/outcome pro
// host_world_dropin; placement/tier pro battle_royale; outcome pros demais) —
// por isso é um mapa livre, não um shape fixo.
export interface EventRule {
  id: string;
  scope: "global" | "game";
  archetype: MultiplayerProfile["archetype"];
  // valor string = igualdade (formato clássico); objeto = comparador
  // (spec achievements/014) — ver lib/matchCond.ts.
  match: Record<string, MatchValue>;
  eventTypes: string[];
  // suppressed (spec achievements/017, V2b): sempre presente; só relevante
  // pra scope "global" — regra de jogo nunca é suprimível (é removível).
  suppressed: boolean;
}

export interface NewEventRule {
  archetype: MultiplayerProfile["archetype"];
  match: Record<string, MatchValue>;
  eventTypes: string[];
}

export interface PlayerDataEntry {
  namespace: string;
  key: string;
  value: unknown; // JSON opaco do save de gameplay
}

export interface CloudSave {
  contentType: string;
  size: number;
  updatedAt: string;
  blob: string; // base64
}

export interface GamePlayer {
  playerId: string;
  handle: string;
  firstSeen: string;
}

export interface SessionPlayer {
  id: string;
  handle: string; // vazio quando o identity não resolveu (best-effort do BFF)
}

// host_world_dropin (spec 020): ocupação de um papel num mundo (co-op X/2, etc.).
// occupants (session spec 019): quem está em cada papel — alimenta os cards do
// expand; ausente em payloads antigos (fallback: só contagem).
export interface RoleSlot {
  role: string;
  capacity: number;
  occupied: number;
  occupants?: string[];
}

export interface GameSession {
  sessionId: string;
  matchId: string;
  gameId?: string; // preenchido no board global (Rooms); vazio na aba por-jogo
  gameName?: string; // idem — resolvido pelo BFF
  state: "open" | "closed";
  modeId?: string; // modo do jogo (ADR-0024, M1); ausente em sessões legadas = "default"
  players: SessionPlayer[];
  serverAddress: string;
  openedAt: string;
  closedAt?: string;
  closeReason?: "result" | "ttl" | "admin" | "host_left" | "host_killed" | "abandoned";
  // host_world_dropin (spec 014/020): origem e slots por papel. queue_arena =
  // origin "match", sem host/roleSlots.
  origin?: "match" | "host_world";
  hostPlayerId?: string;
  roleSlots?: RoleSlot[];
  // Detalhe rico (session spec 018): snapshot do modo (params crus do
  // arquétipo), formação por time e resultado persistido (fechadas por result).
  archetype?: string;
  topology?: "dedicated" | "p2p"; // session/031; ausente em sessões fechadas antes da migration 004
  hostConnection?: string; // p2p: id de conexão do host (ex.: SteamID) — admin-ui/091
  lastHeartbeatAt?: string; // último heartbeat do mundo (session/032); ausente = o jogo não manda
  profileHash?: string;
  modeParams?: Record<string, unknown>;
  teams?: SessionTeam[];
  result?: MatchResult;
  // Participants é a base do histórico persistido (spec 025): um registro por
  // jogador com o papel ocupado. Presente tanto nas sessões em memória quanto
  // nas lidas do store (closedGameSessions/playerSessions).
  participants?: SessionParticipant[];
}

// SessionParticipant é um participante da sessão com o papel ocupado (spec
// 025) — a base do histórico persistido (distinto de SessionPlayer, que é só
// a lista simples mantida por compat).
export interface SessionParticipant {
  playerId: string;
  handle: string; // vazio quando o identity não resolveu (best-effort do BFF)
  role?: string;
  joinedAt: string;
  // lastEvent/lastOutcome (spec 027 do session-service): último evento de
  // participação conhecido — só existem em host_world_dropin.
  lastEvent?: string;
  lastOutcome?: "SUCCESS" | "FAIL" | "NEUTRAL";
}

// Um lado da formação registrada no match (rótulo canônico do session).
export interface SessionTeam {
  teamId: string;
  players: string[];
}

// Outcome polimórfico por arquétipo (tagged union: só o kind tem payload).
export interface MatchResult {
  kind: "win_loss_draw" | "placement" | "collective";
  winLossDraw?: PlayerOutcome[];
  placement?: PlacementEntry[];
  collective?: CollectiveResult;
}

export interface PlayerOutcome {
  playerId: string;
  teamId?: string;
  outcome: "win" | "loss" | "draw";
  score: number;
  stats?: Record<string, number>;
}

export interface PlacementEntry {
  playerId: string;
  teamId?: string;
  placement: number;
  survivedMs: number;
  stats?: Record<string, number>;
}

export interface CollectiveResult {
  outcome: string;
  players?: { playerId: string; teamId?: string; stats?: Record<string, number> }[];
}

export interface GameSessions {
  active: GameSession[];
  closed: GameSession[];
}

// ClosedSessionsPage é uma página de sessões fechadas lidas do store
// persistido (spec 025): nextBefore é o cursor da próxima página (ausente =
// não há mais páginas).
export interface ClosedSessionsPage {
  closed: GameSession[];
  nextBefore?: string;
}

// Analytics: overview cross-game (ADR-0021).
export interface AnalyticsGameStat {
  gameId: string;
  gameName: string;
  matches: number;
  uniquePlayers: number;
  wins: number;
  losses: number;
  draws: number;
  avgScore: number;
}

export interface AnalyticsDaily {
  day: string;
  matches: number;
}

// Overview cross-game (analytics-service/016): a medida comum a todos os
// arquétipos é a sessão fechada; matches/wins/... são só do queue_arena.
export interface AnalyticsOverviewGame extends AnalyticsGameStat {
  sessions: number;
  avgSessionMs: number;
  abandoned: number; // fechadas por abandoned (server caiu) ou ttl (ninguém reportou)
}

export interface AnalyticsOverview {
  totals: { sessions: number; matches: number; uniquePlayers: number; games: number };
  games: AnalyticsOverviewGame[];
  daily: { day: string; sessions: number }[]; // sessões fechadas por dia
}

// host_world_dropin (spec 004/020): contagem por modo/papel/evento/outcome no
// detalhe do jogo. modeId="" agrupa dados anteriores à spec 009 (sem modo).
export interface ParticipationStat {
  modeId: string;
  role: string;
  event: string;
  outcome: string;
  count: number;
}

// Detalhe de analytics por jogo (GET /api/analytics/games/{id}).
export interface GameAnalyticsDetail {
  gameId: string;
  gameName: string;
  // modeId é o eco do filtro efetivamente aplicado (analytics-service/012,
  // admin-bff/026); ausente = todos os modos.
  modeId?: string;
  matches: number;
  uniquePlayers: number;
  wins: number;
  losses: number;
  draws: number;
  avgScore: number;
  daily: AnalyticsDaily[];
  roleParticipations: ParticipationStat[];
  placement?: PlacementDistribution; // battle_royale (BR4); ausente fora do BR
  mission?: MissionDistribution; // coop_mission (C4); ausente fora do coop

  // Séries diárias dos 3 arquétipos que não tinham (analytics-service/010) —
  // vazias fora do arquétipo correspondente, mesmo padrão de placement/mission.
  placementDaily: AnalyticsDaily[];
  missionDaily: AnalyticsDaily[];
  participationDaily: AnalyticsDaily[];
  // Resumo agregado de stats brutos por stat_key (analytics-service/011) —
  // vazio se o jogo nunca reportou stats (sem scoringStats no perfil).
  stats: StatSummary[];
  // Distribuição por motivo de fechamento (analytics-service/013) — vazio
  // se o jogo não tem nenhum session_closed processado ainda.
  sessionHealth: SessionCloseStat[];
  // Série "sessões fechadas por dia" (analytics-service/015) — mesmo
  // padrão das outras séries diárias, sem quebra por closeReason.
  sessionCloseDaily: AnalyticsDaily[];
}

// SessionCloseStat é a distribuição agregada por motivo de fechamento
// (analytics-service/013): conta + duração total (avgDurationMs já vem
// calculado do backend).
export interface SessionCloseStat {
  modeId?: string;
  archetype?: string;
  closeReason: string;
  count: number;
  durationSumMs: number;
  avgDurationMs: number;
}

// StatSummary é o panorama agregado de uma métrica do jogo (analytics-service/011).
export interface StatSummary {
  key: string;
  sum: number;
  count: number;
  avg: number;
}

// PlacementDistribution é o histograma de colocação do battle_royale (BR3/BR4).
export interface PlacementDistribution {
  winners: number;
  topN: number;
  rest: number;
  entries: number;
  avgSurvivedMs: number;
}

// MissionDistribution é a distribuição de desfecho da missão do coop_mission (C3/C4).
export interface MissionDistribution {
  successes: number;
  failures: number;
  successRate: number;
}

// LiveMetrics: contadores operacionais do gameplay loop. null = métrica ausente/fonte fora.
export interface LiveMetrics {
  queueSize: number | null;
  activeSessions: number | null;
  matchesTotal: number | null;
  matchesFailed: number | null;
}

// FanoutHealth: sinais do fan-out durável (ADR-0020). null = métrica ausente/fonte fora.
export interface FanoutHealth {
  publishOk: number | null;
  publishError: number | null;
  consumeOk: number | null;
  consumeDup: number | null;
  consumeError: number | null;
}

// HttpError carrega o status para a UI distinguir 502/409/400 etc. O code é o
// campo "error" do corpo (quando o BFF o envia) — útil p/ mensagens estruturadas.
export class HttpError extends Error {
  constructor(
    public status: number,
    public code?: string,
  ) {
    super(code ? `HTTP ${status}: ${code}` : `HTTP ${status}`);
  }
}

// errorCode lê o campo "error" do corpo (best-effort; body pode não ser JSON).
async function errorCode(resp: Response): Promise<string | undefined> {
  try {
    const body = (await resp.json()) as { error?: string };
    return body.error;
  } catch {
    return undefined;
  }
}

async function getJSON<T>(path: string, signal?: AbortSignal): Promise<T> {
  const resp = await fetch(path, { signal });
  if (!resp.ok) throw new HttpError(resp.status);
  return (await resp.json()) as T;
}

async function postJSON<T>(path: string, body: unknown): Promise<T> {
  return sendJSON<T>("POST", path, body);
}

async function putJSON<T>(path: string, body: unknown): Promise<T> {
  return sendJSON<T>("PUT", path, body);
}

async function sendJSON<T>(method: string, path: string, body: unknown): Promise<T> {
  const resp = await fetch(path, {
    method,
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!resp.ok) throw new HttpError(resp.status, await errorCode(resp));
  return (await resp.json()) as T;
}

async function del(path: string): Promise<void> {
  const resp = await fetch(path, { method: "DELETE" });
  if (!resp.ok) throw new HttpError(resp.status);
}

// postEmpty é o POST sem corpo/sem resposta (204) — mesmo shape de del, só
// muda o verbo (usado pelo suppressEventRule, spec 017).
async function postEmpty(path: string): Promise<void> {
  const resp = await fetch(path, { method: "POST" });
  if (!resp.ok) throw new HttpError(resp.status);
}

export const api = {
  topology: (signal?: AbortSignal) => getJSON<Topology>("/api/topology", signal),
  liveMetrics: (signal?: AbortSignal) => getJSON<LiveMetrics>("/api/metrics/live", signal),
  analyticsOverview: (signal?: AbortSignal) =>
    getJSON<AnalyticsOverview>("/api/analytics/overview", signal),
  // modeId (spec 064): filtra o detalhe pelo modo escolhido; ausente = todos
  // os modos (comportamento atual, default).
  gameAnalytics: (gameId: string, opts?: { modeId?: string }, signal?: AbortSignal) => {
    const q = new URLSearchParams();
    if (opts?.modeId) q.set("modeId", opts.modeId);
    const qs = q.toString();
    return getJSON<GameAnalyticsDetail>(
      `/api/analytics/games/${encodeURIComponent(gameId)}${qs ? `?${qs}` : ""}`,
      signal,
    );
  },

  players: (limit: number, offset: number, signal?: AbortSignal) =>
    getJSON<PlayerPage>(`/api/players?limit=${limit}&offset=${offset}`, signal),
  // Um jogador pelo id (spec 040): handle do header do PlayerDetail. 404 quando
  // não existe (ex.: guest) — o chamador trata como fallback, não como erro fatal.
  player: (playerID: string, signal?: AbortSignal) =>
    getJSON<Player>(`/api/players/${encodeURIComponent(playerID)}`, signal),

  games: (signal?: AbortSignal) => getJSON<Game[]>("/api/games", signal),
  createGame: (slug: string, name: string) => postJSON<Game>("/api/games", { slug, name }),
  renameGame: (gameId: string, name: string) =>
    putJSON<Game>(`/api/games/${encodeURIComponent(gameId)}`, { name }),
  // Fleet Agones do jogo (ADR-0026, aba Fleet — spec 059).
  setFleet: (gameId: string, fleet: string) =>
    putJSON<Game>(`/api/games/${encodeURIComponent(gameId)}/fleet`, { fleet }),
  // Status real da Fleet: onde está rodando + réplicas (aba Fleet — spec 023).
  fleetStatus: (gameId: string, signal?: AbortSignal) =>
    getJSON<FleetStatus>(`/api/games/${encodeURIComponent(gameId)}/fleet-status`, signal),
  // Descrição do jogo (spec 013).
  setGameDescription: (gameId: string, description: string) =>
    putJSON<Game>(`/api/games/${encodeURIComponent(gameId)}/description`, { description }),
  // Soft delete no registry (ADR-0025): o jogo some do console; dados nos
  // demais serviços ficam órfãos e inacessíveis. 404 = já removido.
  deleteGame: (gameId: string) => del(`/api/games/${encodeURIComponent(gameId)}`),
  // Perfil de multiplayer (ADR-0023). getMultiplayerProfile -> null em 404
  // (jogo ainda não configurado); demais erros propagam.
  getMultiplayerProfile: async (
    gameId: string,
    signal?: AbortSignal,
  ): Promise<MultiplayerProfile | null> => {
    try {
      return await getJSON<MultiplayerProfile>(
        `/api/games/${encodeURIComponent(gameId)}/multiplayer-profile`,
        signal,
      );
    } catch (err) {
      if (err instanceof HttpError && err.status === 404) return null;
      throw err;
    }
  },
  // Modos nomeados (ADR-0024, spec admin-ui/023).
  listMultiplayerModes: (gameId: string, signal?: AbortSignal) =>
    getJSON<MultiplayerMode[]>(
      `/api/games/${encodeURIComponent(gameId)}/multiplayer-profiles`,
      signal,
    ),
  setMultiplayerMode: (gameId: string, modeId: string, mode: MultiplayerProfile & { displayName?: string }) =>
    putJSON<MultiplayerMode>(
      `/api/games/${encodeURIComponent(gameId)}/multiplayer-profiles/${encodeURIComponent(modeId)}`,
      mode,
    ),
  deleteMultiplayerMode: (gameId: string, modeId: string) =>
    del(
      `/api/games/${encodeURIComponent(gameId)}/multiplayer-profiles/${encodeURIComponent(modeId)}`,
    ),
  // Hash "atual" de cada modo (spec 022) — mesmo hash que uma sessão aberta
  // agora congelaria; mapa modeId -> hash.
  modeSnapshotHashes: (gameId: string, signal?: AbortSignal) =>
    getJSON<Record<string, string>>(
      `/api/games/${encodeURIComponent(gameId)}/mode-snapshot-hashes`,
      signal,
    ),
  gameCatalog: (gameId: string, signal?: AbortSignal) =>
    getJSON<CatalogAchievement[]>(`/api/games/${encodeURIComponent(gameId)}/achievements`, signal),
  createAchievement: (gameId: string, a: NewAchievement) =>
    postJSON<CatalogAchievement>(`/api/games/${encodeURIComponent(gameId)}/achievements`, a),
  updateAchievement: (gameId: string, id: string, a: Omit<NewAchievement, "id">) =>
    putJSON<CatalogAchievement>(
      `/api/games/${encodeURIComponent(gameId)}/achievements/${encodeURIComponent(id)}`,
      a,
    ),
  deleteAchievement: (gameId: string, id: string) =>
    del(`/api/games/${encodeURIComponent(gameId)}/achievements/${encodeURIComponent(id)}`),
  importAchievements: (gameId: string, achievements: NewAchievement[]) =>
    postJSON<ImportResponse>(`/api/games/${encodeURIComponent(gameId)}/achievements/import`, {
      achievements,
    }),
  playerAchievements: (gameId: string, playerID: string, signal?: AbortSignal) =>
    getJSON<PlayerAchievement[]>(
      `/api/games/${encodeURIComponent(gameId)}/players/${encodeURIComponent(playerID)}/achievements`,
      signal,
    ),
  // board seleciona o ranking (score ou um por stat, ADR-0023); window a janela
  // de tempo e offset a página (spec 039). Omitidos -> score / all-time / 0.
  gameLeaderboard: (
    gameId: string,
    opts?: { board?: string; window?: string; limit?: number; offset?: number },
    signal?: AbortSignal,
  ) => {
    const q = new URLSearchParams({ limit: String(opts?.limit ?? 20) });
    if (opts?.board) q.set("board", opts.board);
    if (opts?.window && opts.window !== "alltime") q.set("window", opts.window);
    if (opts?.offset) q.set("offset", String(opts.offset));
    return getJSON<LeaderEntry[]>(
      `/api/games/${encodeURIComponent(gameId)}/leaderboard?${q.toString()}`,
      signal,
    );
  },
  setLeaderboardScore: (gameId: string, playerId: string, score: number, board?: string) => {
    const q = board ? `?board=${encodeURIComponent(board)}` : "";
    return putJSON<LeaderEntry>(
      `/api/games/${encodeURIComponent(gameId)}/leaderboard/${encodeURIComponent(playerId)}/score${q}`,
      { score },
    );
  },
  achievementProgress: (gameId: string, achievementId: string, signal?: AbortSignal) =>
    getJSON<{ progress: AchievementProgressEntry[] }>(
      `/api/games/${encodeURIComponent(gameId)}/achievements/${encodeURIComponent(achievementId)}/progress`,
      signal,
    ),
  // Regras de mapeamento evento->achievement (migration 008): globais do
  // arquétipo (read-only) + overrides do jogo (CRUD).
  gameEventRules: (gameId: string, signal?: AbortSignal) =>
    getJSON<EventRule[]>(`/api/games/${encodeURIComponent(gameId)}/event-rules`, signal),
  createEventRule: (gameId: string, r: NewEventRule) =>
    postJSON<EventRule>(`/api/games/${encodeURIComponent(gameId)}/event-rules`, r),
  // archetype não é editável (spec achievements/016) — o PUT leva só o conteúdo.
  updateEventRule: (gameId: string, id: string, r: Omit<NewEventRule, "archetype">) =>
    putJSON<EventRule>(
      `/api/games/${encodeURIComponent(gameId)}/event-rules/${encodeURIComponent(id)}`,
      r,
    ),
  deleteEventRule: (gameId: string, id: string) =>
    del(`/api/games/${encodeURIComponent(gameId)}/event-rules/${encodeURIComponent(id)}`),
  // Supressão de regra global por jogo (spec achievements/017, V2b).
  suppressEventRule: (gameId: string, id: string) =>
    postEmpty(
      `/api/games/${encodeURIComponent(gameId)}/event-rules/${encodeURIComponent(id)}/suppress`,
    ),
  unsuppressEventRule: (gameId: string, id: string) =>
    del(
      `/api/games/${encodeURIComponent(gameId)}/event-rules/${encodeURIComponent(id)}/suppress`,
    ),
  playerData: (gameId: string, playerID: string, signal?: AbortSignal) =>
    getJSON<PlayerDataEntry[]>(
      `/api/games/${encodeURIComponent(gameId)}/players/${encodeURIComponent(playerID)}/data`,
      signal,
    ),
  cloudSave: (gameId: string, playerID: string, signal?: AbortSignal) =>
    getJSON<CloudSave>(
      `/api/games/${encodeURIComponent(gameId)}/players/${encodeURIComponent(playerID)}/save`,
      signal,
    ),
  gamePlayers: (gameId: string, signal?: AbortSignal) =>
    getJSON<GamePlayer[]>(`/api/games/${encodeURIComponent(gameId)}/players`, signal),
  gamesOfPlayer: (playerID: string, signal?: AbortSignal) =>
    getJSON<Game[]>(`/api/players/${encodeURIComponent(playerID)}/games`, signal),
  gameSessions: (gameId: string, signal?: AbortSignal) =>
    getJSON<GameSessions>(`/api/games/${encodeURIComponent(gameId)}/sessions`, signal),

  // Histórico persistido de sessões fechadas (spec 025) — sobrevive a restart
  // do session-service, ao contrário do ring in-memory de `gameSessions`.
  // `before` é o cursor RFC3339 (nextBefore da página anterior).
  closedGameSessions: (
    gameId: string,
    opts?: { limit?: number; before?: string },
    signal?: AbortSignal,
  ) => {
    const q = new URLSearchParams({ state: "closed" });
    if (opts?.limit) q.set("limit", String(opts.limit));
    if (opts?.before) q.set("before", opts.before);
    return getJSON<ClosedSessionsPage>(
      `/api/games/${encodeURIComponent(gameId)}/sessions?${q.toString()}`,
      signal,
    );
  },
  // Histórico do jogador no jogo (spec 025, aba History): sessões fechadas em
  // que ele participou, mesma paginação por cursor.
  playerSessions: (
    playerId: string,
    gameId: string,
    opts?: { limit?: number; before?: string },
    signal?: AbortSignal,
  ) => {
    const q = new URLSearchParams({ gameId });
    if (opts?.limit) q.set("limit", String(opts.limit));
    if (opts?.before) q.set("before", opts.before);
    return getJSON<ClosedSessionsPage>(
      `/api/players/${encodeURIComponent(playerId)}/sessions?${q.toString()}`,
      signal,
    );
  },

  // Rooms Console (board global de salas + force-close + saúde do fan-out).
  rooms: (signal?: AbortSignal) => getJSON<GameSessions>("/api/rooms", signal),
  closeRoom: (matchId: string) =>
    postJSON<GameSession>(`/api/rooms/${encodeURIComponent(matchId)}/close`, {}),
  fanoutHealth: (signal?: AbortSignal) => getJSON<FanoutHealth>("/api/health/fanout", signal),
};

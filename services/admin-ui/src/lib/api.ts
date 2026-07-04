// Cliente da API do admin-bff. Em dev, as chamadas a /api são repassadas pelo
// proxy do Vite ao BFF (:8088). Foco em features de produto — observabilidade
// detalhada fica no Grafana (ADR-0016/0006).

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
}

export interface CatalogAchievement {
  id: string;
  name: string;
  description: string;
  eventType: string;
  target: number;
}

export interface NewAchievement {
  id: string;
  name: string;
  description: string;
  eventType: string;
  target: number;
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
}

export interface AchievementProgressEntry {
  playerId: string;
  progress: number;
  unlocked: boolean;
  unlockedAt?: string;
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

export interface GameSession {
  sessionId: string;
  matchId: string;
  gameId?: string; // preenchido no board global (Rooms); vazio na aba por-jogo
  gameName?: string; // idem — resolvido pelo BFF
  state: "open" | "closed";
  players: SessionPlayer[];
  serverAddress: string;
  openedAt: string;
  closedAt?: string;
  closeReason?: "result" | "ttl" | "admin";
}

export interface GameSessions {
  active: GameSession[];
  closed: GameSession[];
}

// Analytics: overview cross-game (ADR-0021).
export interface AnalyticsGameStat {
  gameId: string;
  gameName: string;
  matches: number;
  uniquePlayers: number;
  wins: number;
  avgScore: number;
}

export interface AnalyticsDaily {
  day: string;
  matches: number;
}

export interface AnalyticsOverview {
  totals: { matches: number; uniquePlayers: number; games: number };
  games: AnalyticsGameStat[];
  daily: AnalyticsDaily[];
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

// HttpError carrega o status para a UI distinguir 502/409/400 etc.
export class HttpError extends Error {
  constructor(public status: number) {
    super(`HTTP ${status}`);
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
  if (!resp.ok) throw new HttpError(resp.status);
  return (await resp.json()) as T;
}

async function del(path: string): Promise<void> {
  const resp = await fetch(path, { method: "DELETE" });
  if (!resp.ok) throw new HttpError(resp.status);
}

export const api = {
  topology: (signal?: AbortSignal) => getJSON<Topology>("/api/topology", signal),
  liveMetrics: (signal?: AbortSignal) => getJSON<LiveMetrics>("/api/metrics/live", signal),
  analyticsOverview: (signal?: AbortSignal) =>
    getJSON<AnalyticsOverview>("/api/analytics/overview", signal),

  players: (limit: number, offset: number, signal?: AbortSignal) =>
    getJSON<PlayerPage>(`/api/players?limit=${limit}&offset=${offset}`, signal),

  games: (signal?: AbortSignal) => getJSON<Game[]>("/api/games", signal),
  createGame: (slug: string, name: string) => postJSON<Game>("/api/games", { slug, name }),
  renameGame: (gameId: string, name: string) =>
    putJSON<Game>(`/api/games/${encodeURIComponent(gameId)}`, { name }),
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
  gameLeaderboard: (gameId: string, limit = 20, signal?: AbortSignal) =>
    getJSON<LeaderEntry[]>(
      `/api/games/${encodeURIComponent(gameId)}/leaderboard?limit=${limit}`,
      signal,
    ),
  setLeaderboardScore: (gameId: string, playerId: string, score: number) =>
    putJSON<LeaderEntry>(
      `/api/games/${encodeURIComponent(gameId)}/leaderboard/${encodeURIComponent(playerId)}/score`,
      { score },
    ),
  achievementProgress: (gameId: string, achievementId: string, signal?: AbortSignal) =>
    getJSON<{ progress: AchievementProgressEntry[] }>(
      `/api/games/${encodeURIComponent(gameId)}/achievements/${encodeURIComponent(achievementId)}/progress`,
      signal,
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

  // Rooms Console (board global de salas + force-close + saúde do fan-out).
  rooms: (signal?: AbortSignal) => getJSON<GameSessions>("/api/rooms", signal),
  closeRoom: (matchId: string) =>
    postJSON<GameSession>(`/api/rooms/${encodeURIComponent(matchId)}/close`, {}),
  fanoutHealth: (signal?: AbortSignal) => getJSON<FanoutHealth>("/api/health/fanout", signal),
};

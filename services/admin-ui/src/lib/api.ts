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
}

export interface CatalogAchievement {
  id: string;
  name: string;
  description: string;
  target: number;
}

export interface NewAchievement {
  id: string;
  name: string;
  description: string;
  eventType: string;
  target: number;
}

export interface LeaderEntry {
  rank: number;
  playerId: string;
  score: number;
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
  const resp = await fetch(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!resp.ok) throw new HttpError(resp.status);
  return (await resp.json()) as T;
}

export const api = {
  topology: (signal?: AbortSignal) => getJSON<Topology>("/api/topology", signal),

  players: (limit: number, offset: number, signal?: AbortSignal) =>
    getJSON<PlayerPage>(`/api/players?limit=${limit}&offset=${offset}`, signal),

  games: (signal?: AbortSignal) => getJSON<Game[]>("/api/games", signal),
  createGame: (slug: string, name: string) => postJSON<Game>("/api/games", { slug, name }),
  gameCatalog: (gameId: string, signal?: AbortSignal) =>
    getJSON<CatalogAchievement[]>(`/api/games/${encodeURIComponent(gameId)}/achievements`, signal),
  createAchievement: (gameId: string, a: NewAchievement) =>
    postJSON<CatalogAchievement>(`/api/games/${encodeURIComponent(gameId)}/achievements`, a),
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
};

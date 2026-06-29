// Cliente da API do admin-bff. Em dev, as chamadas a /api são repassadas pelo
// proxy do Vite ao BFF (:8088), evitando CORS.

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

export interface LiveMetrics {
  queueSize: number | null;
  activeSessions: number | null;
  matchesTotal: number | null;
  matchesFailed: number | null;
}

export interface ServiceMetrics {
  id: string;
  up: boolean | null;
  requestRate: number | null;
  errorRate: number | null;
  latencyP95: number | null;
  goroutines: number | null;
  memoryBytes: number | null;
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

// HttpError carrega o status para a UI distinguir 502/503/404 etc.
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

export const api = {
  topology: (signal?: AbortSignal) => getJSON<Topology>("/api/topology", signal),
  liveMetrics: (signal?: AbortSignal) => getJSON<LiveMetrics>("/api/metrics/live", signal),
  serviceMetrics: (id: string, signal?: AbortSignal) =>
    getJSON<ServiceMetrics>(`/api/services/${encodeURIComponent(id)}/metrics`, signal),
  playerAchievements: (playerID: string, signal?: AbortSignal) =>
    getJSON<PlayerAchievement[]>(`/api/players/${encodeURIComponent(playerID)}/achievements`, signal),
};

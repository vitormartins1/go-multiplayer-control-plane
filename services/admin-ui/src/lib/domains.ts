// Catálogo de domínios do painel (dirigido por dados — ver plan). Adicionar ou
// ajustar um domínio é editar este arquivo, não criar telas novas.
//
// - id: casa com o job/serviço do Prometheus (e com /api/services/{id}/metrics)
// - counters: contadores de produto a exibir, lidos do /api/metrics/live
// - iter2: dado profundo ainda não disponível (placeholder visível, roadmap)
// - planned: domínio cujo serviço ainda não existe (ex.: analytics)

import type { LiveMetrics } from "./api";

export interface DomainCounter {
  label: string;
  pick: (live: LiveMetrics) => number | null;
}

export interface Domain {
  id: string; // "" = visão geral
  label: string;
  kind: "overview" | "service" | "planned";
  counters?: DomainCounter[];
  iter2?: string[];
  hasAchievements?: boolean;
  hasUsers?: boolean;
}

export const DOMAINS: Domain[] = [
  { id: "", label: "Visão geral", kind: "overview" },
  {
    id: "identity-service",
    label: "User base",
    kind: "service",
    hasUsers: true,
  },
  {
    id: "achievements-service",
    label: "Achievements",
    kind: "service",
    hasAchievements: true,
    iter2: ["Analytics agregada (taxa de unlock por conquista)"],
  },
  {
    id: "matchmaking-service",
    label: "Matchmaking",
    kind: "service",
    counters: [
      { label: "Fila", pick: (l) => l.queueSize },
      { label: "Matches", pick: (l) => l.matchesTotal },
      { label: "Falhos", pick: (l) => l.matchesFailed },
    ],
    iter2: ["Salas / tickets ao vivo"],
  },
  {
    id: "session-service",
    label: "Sessions",
    kind: "service",
    counters: [{ label: "Sessions ativas", pick: (l) => l.activeSessions }],
    iter2: ["Sessions ao vivo detalhadas"],
  },
  {
    id: "player-data-service",
    label: "Player data",
    kind: "service",
    iter2: ["Dados por jogador"],
  },
  {
    id: "cloud-save-service",
    label: "Cloud save",
    kind: "service",
    iter2: ["Saves e tamanho por jogador"],
  },
  {
    id: "leaderboard-service",
    label: "Leaderboard",
    kind: "service",
    iter2: ["Top-N por ranking"],
  },
  { id: "analytics-service", label: "Game analytics", kind: "planned" },
];

import type { MultiplayerMode } from "./api";

// modeScoringStats extrai as stats declaradas por um modo (mesmo campo nos 4
// arquétipos) — usada no mapa de dados (GamePipeline) e na tabela de modos
// (MultiplayerSettings).
export function modeScoringStats(m: MultiplayerMode): string[] {
  return "scoringStats" in m.params ? m.params.scoringStats : [];
}

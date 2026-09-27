import type { Game, GameSession } from "./api";
import { archetypeMetaOf } from "./archetypeMeta";

// Ordenação das tabelas de sessão (spec 060, compartilhada por Rooms e pela aba
// Sessões do jogo desde a admin-ui/092).
export type SortKey = "game" | "archetype" | "mode" | "state" | "players" | "time";
export type SortDir = "asc" | "desc";

// compareSessions é o comparador puro. `gamesById` só é necessário para o
// sortBy "game" (o nome do jogo pode não vir na própria sessão).
export function compareSessions(
  a: GameSession,
  b: GameSession,
  sortBy: SortKey,
  sortDir: SortDir,
  gamesById: Map<string, Game> = new Map(),
): number {
  const dir = sortDir === "asc" ? 1 : -1;
  switch (sortBy) {
    case "game": {
      const an = a.gameName || gamesById.get(a.gameId ?? "")?.name || a.gameId || "";
      const bn = b.gameName || gamesById.get(b.gameId ?? "")?.name || b.gameId || "";
      return an.localeCompare(bn) * dir;
    }
    case "archetype": {
      // sem arquétipo conhecido vai pro fim SEMPRE, nos dois sentidos (spec 060, item 13)
      const al = archetypeMetaOf(a.archetype)?.label;
      const bl = archetypeMetaOf(b.archetype)?.label;
      if (!al && !bl) return 0;
      if (!al) return 1;
      if (!bl) return -1;
      return al.localeCompare(bl) * dir;
    }
    case "mode":
      return (a.modeId || "default").localeCompare(b.modeId || "default") * dir;
    case "state": {
      // abertas antes das fechadas (item 12) — não alfabético
      const rank = (s: GameSession) => (s.state === "open" ? 0 : 1);
      return (rank(a) - rank(b)) * dir;
    }
    case "players":
      return (a.players.length - b.players.length) * dir;
    case "time":
      // só openedAt, sempre (item 15): duração e "aberta há" não se comparam
      return (Date.parse(a.openedAt) - Date.parse(b.openedAt)) * dir;
    default:
      return 0;
  }
}

import { createContext, useContext, useState, type ReactNode } from "react";
import { api, type Game } from "./api";
import { useAsync } from "./useAsync";

const STORAGE_KEY = "admin.activeGameId";

interface GamesContextValue {
  games: Game[];
  loading: boolean;
  error: unknown;
  reload: () => void;
  activeGameId: string | null;
  activeGame: Game | null;
  setActiveGameId: (id: string | null) => void;
}

const Ctx = createContext<GamesContextValue | null>(null);

// GameProvider é a fonte única da lista de jogos (carrega api.games uma vez) e
// do "jogo ativo" global do painel (persistido em localStorage). Header,
// biblioteca de Games e abas do jogador consomem este contexto.
export function GameProvider({ children }: { children: ReactNode }) {
  const { data, loading, error, reload } = useAsync((s) => api.games(s), []);
  const [activeGameId, setActiveGameIdState] = useState<string | null>(() =>
    localStorage.getItem(STORAGE_KEY),
  );

  const setActiveGameId = (id: string | null) => {
    setActiveGameIdState(id);
    if (id) localStorage.setItem(STORAGE_KEY, id);
    else localStorage.removeItem(STORAGE_KEY);
  };

  const games = data ?? [];
  // activeGame resolve para null se o id salvo não existe mais (jogo removido).
  const activeGame = games.find((g) => g.id === activeGameId) ?? null;

  return (
    <Ctx.Provider value={{ games, loading, error, reload, activeGameId, activeGame, setActiveGameId }}>
      {children}
    </Ctx.Provider>
  );
}

// useGames expõe o contexto de jogos. Lança se usado fora do GameProvider.
export function useGames(): GamesContextValue {
  const v = useContext(Ctx);
  if (!v) throw new Error("useGames deve ser usado dentro de <GameProvider>");
  return v;
}

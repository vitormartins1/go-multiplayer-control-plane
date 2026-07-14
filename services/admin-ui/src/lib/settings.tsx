import { createContext, useContext, useEffect, useState, type ReactNode } from "react";

// Settings do control plane (spec 082) — preferências de UI, não dado de
// domínio: hoje só o estilo das capas geradas (DiceBear) dos jogos, com
// mais chegando "com o tempo" (pedido do Vitor). Guardado no localStorage
// (por navegador/dispositivo) — não é um campo por-jogo: `Game.
// generatedImageStyle` continua existindo no backend (default "shapes",
// game-registry/internal/service/game_service.go), mas a RENDERIZAÇÃO no
// admin-ui agora obedece essa preferência global, não o campo por-jogo —
// não existe hoje nenhum jeito de setar esse campo diferente por jogo
// (createGame não aceita o parâmetro, não existe endpoint de update), então
// uma preferência global client-side é o que resolve o pedido sem inventar
// uma capacidade de backend que ninguém pediu ainda.
export const GAME_COVER_STYLES = [
  { value: "shapes", label: "Shapes" },
  { value: "stripes", label: "Stripes" },
  { value: "triangles", label: "Triangles" },
  { value: "shape-grid", label: "Shape Grid" },
  { value: "initials", label: "Initials" },
  { value: "glass", label: "Glass" },
  { value: "disco", label: "Disco" },
] as const;

export type GameCoverStyle = (typeof GAME_COVER_STYLES)[number]["value"];
const DEFAULT_GAME_COVER_STYLE: GameCoverStyle = "shapes";
const STORAGE_KEY = "mcp.settings.gameCoverStyle";

function isGameCoverStyle(v: string | null): v is GameCoverStyle {
  return GAME_COVER_STYLES.some((s) => s.value === v);
}

function readStored(): GameCoverStyle {
  if (typeof window === "undefined") return DEFAULT_GAME_COVER_STYLE;
  const v = window.localStorage.getItem(STORAGE_KEY);
  return isGameCoverStyle(v) ? v : DEFAULT_GAME_COVER_STYLE;
}

interface SettingsValue {
  gameCoverStyle: GameCoverStyle;
  setGameCoverStyle: (style: GameCoverStyle) => void;
}

const SettingsContext = createContext<SettingsValue | null>(null);

// SettingsProvider envolve o App inteiro (main.tsx/App.tsx) — GameCover é
// consumido em muitos pontos (Games, GameDetail, GameCreateWizard,
// MultiplayerSettings, Fleet) e todos precisam reagir na hora quando a
// preferência muda, sem reload de página.
export function SettingsProvider({ children }: { children: ReactNode }) {
  const [gameCoverStyle, setGameCoverStyleState] = useState<GameCoverStyle>(readStored);

  function setGameCoverStyle(style: GameCoverStyle) {
    setGameCoverStyleState(style);
    window.localStorage.setItem(STORAGE_KEY, style);
  }

  // Sincroniza entre abas (achado: sem isso, mudar em uma aba não reflete
  // nas outras já abertas até um reload).
  useEffect(() => {
    function onStorage(e: StorageEvent) {
      if (e.key === STORAGE_KEY && isGameCoverStyle(e.newValue)) setGameCoverStyleState(e.newValue);
    }
    window.addEventListener("storage", onStorage);
    return () => window.removeEventListener("storage", onStorage);
  }, []);

  return (
    <SettingsContext.Provider value={{ gameCoverStyle, setGameCoverStyle }}>{children}</SettingsContext.Provider>
  );
}

export function useSettings(): SettingsValue {
  const ctx = useContext(SettingsContext);
  if (!ctx) throw new Error("useSettings: fora de um SettingsProvider");
  return ctx;
}

import { createContext, useContext, useEffect, useState, type ReactNode } from "react";

// Settings do control plane (spec 082/084) — preferências de UI, não dado de
// domínio. Guardado no localStorage (por navegador/dispositivo), sincroniza
// entre abas abertas via evento "storage". Cada preferência usa
// createStoredEnum (genérico desde a 2ª+3ª preferência — mesmo critério já
// usado pra extrair ChannelStrip/MockZone nesta sessão: generalizar quando
// o padrão se repete, não antes).

function createStoredEnum<T extends string | number>(key: string, allowed: readonly T[], fallback: T) {
  const isValid = (v: unknown): v is T => allowed.includes(v as T);

  function read(): T {
    if (typeof window === "undefined") return fallback;
    const raw = window.localStorage.getItem(key);
    if (raw === null) return fallback;
    // number-valued settings (pageSize, pollingMs) chegam como string do
    // localStorage — precisa coagir antes de validar contra `allowed`.
    const parsed = typeof fallback === "number" ? Number(raw) : raw;
    return isValid(parsed) ? parsed : fallback;
  }

  function write(value: T) {
    window.localStorage.setItem(key, String(value));
  }

  return { key, read, write, isValid };
}

// --- Estilo das capas dos jogos (spec 082) ---
// `Game.generatedImageStyle` continua existindo no backend (default
// "shapes", game-registry/internal/service/game_service.go) mas é sempre
// fixo — não existe endpoint de update. A RENDERIZAÇÃO no admin-ui obedece
// esta preferência global, não o campo por-jogo.
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
const gameCoverStyleSetting = createStoredEnum<GameCoverStyle>(
  "mcp.settings.gameCoverStyle",
  GAME_COVER_STYLES.map((s) => s.value),
  "shapes",
);

// --- Itens por página (spec 084) ---
// Achado revisando o console: Players.tsx usava PAGE=50, GameLeaderboards/
// GameSessions/PlayerHistory usavam 20 — 3 números diferentes pra "quantos
// itens numa página", sem motivo real de divergir. Um valor global.
export const PAGE_SIZE_OPTIONS = [10, 20, 50, 100] as const;
export type PageSize = (typeof PAGE_SIZE_OPTIONS)[number];
const pageSizeSetting = createStoredEnum<PageSize>("mcp.settings.pageSize", PAGE_SIZE_OPTIONS, 20);

// --- Intervalo de atualização automática (spec 084) ---
// Achado: HealthStrip/FleetNavBadge (App.tsx) e Overview.tsx usavam 10s,
// GameSessions/Rooms usavam 5s — mesma inconsistência sem motivo real. 0 =
// pausado (pedido do Vitor: "poder congelar pra debugar").
export const POLLING_OPTIONS = [
  { value: 5000, label: "5s" },
  { value: 10000, label: "10s" },
  { value: 30000, label: "30s" },
  { value: 0, label: "Pausado" },
] as const;
export type PollingMs = (typeof POLLING_OPTIONS)[number]["value"];
const pollingMsSetting = createStoredEnum<PollingMs>(
  "mcp.settings.pollingMs",
  POLLING_OPTIONS.map((o) => o.value),
  5000,
);

interface SettingsValue {
  gameCoverStyle: GameCoverStyle;
  setGameCoverStyle: (style: GameCoverStyle) => void;
  pageSize: PageSize;
  setPageSize: (size: PageSize) => void;
  pollingMs: PollingMs;
  setPollingMs: (ms: PollingMs) => void;
}

const SettingsContext = createContext<SettingsValue | null>(null);

// SettingsProvider envolve o App inteiro (main.tsx) — consumido em muitos
// pontos (GameCover, toda página paginada, todo polling ao vivo) que
// precisam reagir na hora quando a preferência muda, sem reload de página.
export function SettingsProvider({ children }: { children: ReactNode }) {
  const [gameCoverStyle, setGameCoverStyleState] = useState(gameCoverStyleSetting.read);
  const [pageSize, setPageSizeState] = useState(pageSizeSetting.read);
  const [pollingMs, setPollingMsState] = useState(pollingMsSetting.read);

  function setGameCoverStyle(v: GameCoverStyle) {
    setGameCoverStyleState(v);
    gameCoverStyleSetting.write(v);
  }
  function setPageSize(v: PageSize) {
    setPageSizeState(v);
    pageSizeSetting.write(v);
  }
  function setPollingMs(v: PollingMs) {
    setPollingMsState(v);
    pollingMsSetting.write(v);
  }

  useEffect(() => {
    function onStorage(e: StorageEvent) {
      if (e.key === gameCoverStyleSetting.key && gameCoverStyleSetting.isValid(e.newValue)) {
        setGameCoverStyleState(e.newValue as GameCoverStyle);
      } else if (e.key === pageSizeSetting.key && pageSizeSetting.isValid(Number(e.newValue))) {
        setPageSizeState(Number(e.newValue) as PageSize);
      } else if (e.key === pollingMsSetting.key && pollingMsSetting.isValid(Number(e.newValue))) {
        setPollingMsState(Number(e.newValue) as PollingMs);
      }
    }
    window.addEventListener("storage", onStorage);
    return () => window.removeEventListener("storage", onStorage);
  }, []);

  return (
    <SettingsContext.Provider
      value={{ gameCoverStyle, setGameCoverStyle, pageSize, setPageSize, pollingMs, setPollingMs }}
    >
      {children}
    </SettingsContext.Provider>
  );
}

export function useSettings(): SettingsValue {
  const ctx = useContext(SettingsContext);
  if (!ctx) throw new Error("useSettings: fora de um SettingsProvider");
  return ctx;
}

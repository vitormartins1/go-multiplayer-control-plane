// dashboardLayout.ts: persistência client-only (localStorage) do layout
// configurável da aba Analytics (spec 062) — posição/tamanho/visibilidade dos
// widgets, por jogo. Decisão deliberada (YAGNI): sem backend, sem
// sincronização entre dispositivos — se aparecer essa necessidade real, migra
// pra um endpoint no admin-bff sem quebrar o formato (é aditivo).

export interface WidgetLayoutItem {
  i: string; // id do widget (mesmo id do catálogo)
  x: number;
  y: number;
  w: number;
  h: number;
}

export interface DashboardLayoutState {
  items: WidgetLayoutItem[];
  hidden: string[]; // ids escondidos pelo operador
}

function storageKey(gameId: string): string {
  return `analytics-dashboard-layout:${gameId}`;
}

// defaultLayoutItem posiciona os widgets em uma coluna só, empilhados na
// ordem do catálogo — layout inicial simples, o operador reorganiza como
// quiser depois.
export function defaultLayout(catalog: { id: string; defaultH: number }[]): DashboardLayoutState {
  let y = 0;
  const items = catalog.map((w) => {
    const item: WidgetLayoutItem = { i: w.id, x: 0, y, w: 12, h: w.defaultH };
    y += w.defaultH;
    return item;
  });
  return { items, hidden: [] };
}

// loadLayout lê o localStorage e reconcilia com o catálogo atual: widgets
// novos no catálogo (nunca vistos, ou indisponíveis antes) entram no fim com
// o defaultH; entradas salvas cujo widget sumiu do catálogo são descartadas.
export function loadLayout(gameId: string, catalog: { id: string; defaultH: number }[]): DashboardLayoutState {
  const catalogIds = new Set(catalog.map((w) => w.id));
  const raw = localStorage.getItem(storageKey(gameId));
  if (!raw) return defaultLayout(catalog);

  try {
    const saved = JSON.parse(raw) as DashboardLayoutState;
    const savedIds = new Set(saved.items.map((it) => it.i));
    const items = saved.items.filter((it) => catalogIds.has(it.i));
    let maxY = items.reduce((m, it) => Math.max(m, it.y + it.h), 0);
    for (const w of catalog) {
      if (!savedIds.has(w.id)) {
        items.push({ i: w.id, x: 0, y: maxY, w: 12, h: w.defaultH });
        maxY += w.defaultH;
      }
    }
    return { items, hidden: saved.hidden.filter((id) => catalogIds.has(id)) };
  } catch {
    return defaultLayout(catalog);
  }
}

export function saveLayout(gameId: string, state: DashboardLayoutState): void {
  localStorage.setItem(storageKey(gameId), JSON.stringify(state));
}

export function clearLayout(gameId: string): void {
  localStorage.removeItem(storageKey(gameId));
}

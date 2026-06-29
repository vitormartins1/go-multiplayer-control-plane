import { useState } from "react";
import { Sidebar } from "./components/Sidebar";
import { RefreshControls } from "./components/RefreshControls";
import { Overview } from "./components/Overview";
import { DomainPage } from "./components/DomainPage";
import { GamesPanel } from "./components/GamesPanel";
import { DOMAINS } from "./lib/domains";
import type { RefreshOptions } from "./lib/useLive";
import type { Game } from "./lib/api";

export function App() {
  const [active, setActive] = useState(""); // "" = Visão geral
  const [refresh, setRefresh] = useState<RefreshOptions>({ enabled: true, intervalMs: 5000 });
  // jogo selecionado: estado compartilhado (Games <-> Achievements). ADR-0017.
  const [selectedGame, setSelectedGame] = useState<Game | null>(null);

  const domain = DOMAINS.find((d) => d.id === active) ?? DOMAINS[0];

  return (
    <div className="app">
      <Sidebar active={active} onSelect={setActive} />
      <main className="content">
        <div className="content-header">
          <h2>{domain.label}</h2>
          <RefreshControls opts={refresh} onChange={setRefresh} />
        </div>

        {domain.kind === "overview" ? (
          <Overview refresh={refresh} onSelectDomain={setActive} />
        ) : domain.kind === "games" ? (
          <GamesPanel selectedGame={selectedGame} onSelectGame={setSelectedGame} />
        ) : domain.kind === "planned" ? (
          <p className="muted">Em breve — o serviço de {domain.label.toLowerCase()} ainda não existe.</p>
        ) : (
          // key força remontar ao trocar de domínio (estado limpo por página)
          <DomainPage key={domain.id} domain={domain} refresh={refresh} selectedGame={selectedGame} />
        )}
      </main>
    </div>
  );
}

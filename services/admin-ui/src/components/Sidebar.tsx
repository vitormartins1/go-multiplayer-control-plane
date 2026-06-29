import { DOMAINS } from "../lib/domains";

// Sidebar: navegação por domínio. A seção ativa é o id do domínio
// ("" = Visão geral). "planned" (ex.: analytics) fica desabilitado.
export function Sidebar({
  active,
  onSelect,
}: {
  active: string;
  onSelect: (id: string) => void;
}) {
  return (
    <nav className="sidebar">
      <h1>Admin Console</h1>
      {DOMAINS.map((d) => {
        const planned = d.kind === "planned";
        const isActive = d.id === active;
        return (
          <button
            key={d.id || "overview"}
            className={`nav-item ${isActive ? "active" : ""} ${planned ? "disabled" : ""}`}
            onClick={() => !planned && onSelect(d.id)}
            disabled={planned}
          >
            {d.label}
            {planned && <span className="nav-badge">em breve</span>}
          </button>
        );
      })}
    </nav>
  );
}

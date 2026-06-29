import { useState } from "react";
import { api, HttpError, type PlayerAchievement } from "../lib/api";

type State =
  | { kind: "idle" }
  | { kind: "loading" }
  | { kind: "error"; message: string }
  | { kind: "loaded"; items: PlayerAchievement[] };

// AchievementsPanel: busca os achievements de um jogador (sob demanda).
export function AchievementsPanel() {
  const [playerID, setPlayerID] = useState("");
  const [state, setState] = useState<State>({ kind: "idle" });

  async function search() {
    const id = playerID.trim();
    if (!id) return;
    setState({ kind: "loading" });
    try {
      const items = await api.playerAchievements(id);
      setState({ kind: "loaded", items });
    } catch (e) {
      const msg =
        e instanceof HttpError && e.status === 502
          ? "Serviço de achievements indisponível."
          : "Falha ao buscar achievements.";
      setState({ kind: "error", message: msg });
    }
  }

  return (
    <div className="iter2" style={{ borderStyle: "solid", marginTop: 16 }}>
      <h4>Buscar achievements de um jogador</h4>
      <div className="search-row">
        <input
          placeholder="playerID (id da conta / sub — não o handle)"
          value={playerID}
          onChange={(e) => setPlayerID(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && search()}
          style={{ flex: 1, maxWidth: 420 }}
        />
        <button onClick={search}>Buscar</button>
      </div>

      {state.kind === "loading" && <p className="muted">Buscando…</p>}
      {state.kind === "error" && <p style={{ color: "var(--down)" }}>{state.message}</p>}
      {state.kind === "loaded" && state.items.length === 0 && (
        <p className="muted">Nenhum progresso para este jogador.</p>
      )}
      {state.kind === "loaded" && state.items.length > 0 && (
        <table>
          <thead>
            <tr>
              <th>Conquista</th>
              <th>Progresso</th>
              <th>Estado</th>
            </tr>
          </thead>
          <tbody>
            {state.items.map((a) => (
              <tr key={a.id}>
                <td>{a.name}</td>
                <td>
                  {a.progress}/{a.target}
                </td>
                <td>
                  {a.unlocked ? (
                    <span style={{ color: "var(--up)" }}>
                      desbloqueada{a.unlockedAt ? ` · ${new Date(a.unlockedAt).toLocaleString("pt-BR")}` : ""}
                    </span>
                  ) : (
                    <span className="muted">bloqueada</span>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}

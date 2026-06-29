import { useEffect, useState } from "react";
import { api, HttpError, type PlayerPage } from "../lib/api";

const PAGE = 50;

type State =
  | { kind: "loading" }
  | { kind: "error"; message: string }
  | { kind: "loaded"; data: PlayerPage };

// UsersPanel: lista paginada de usuários (User base) via /api/players.
export function UsersPanel() {
  const [offset, setOffset] = useState(0);
  const [state, setState] = useState<State>({ kind: "loading" });

  useEffect(() => {
    const ac = new AbortController();
    setState({ kind: "loading" });
    api
      .players(PAGE, offset, ac.signal)
      .then((data) => setState({ kind: "loaded", data }))
      .catch((e) => {
        if (ac.signal.aborted) return;
        const msg =
          e instanceof HttpError && e.status === 502
            ? "Serviço de identity indisponível."
            : "Falha ao carregar usuários.";
        setState({ kind: "error", message: msg });
      });
    return () => ac.abort();
  }, [offset]);

  if (state.kind === "error") {
    return <p style={{ color: "var(--down)" }}>{state.message}</p>;
  }

  const data = state.kind === "loaded" ? state.data : undefined;
  const total = data?.total ?? 0;
  const canPrev = offset > 0;
  const canNext = offset + PAGE < total;

  return (
    <div>
      <div className="cards">
        <div className="card">
          <div className="label">Usuários cadastrados</div>
          <div className="value">{state.kind === "loading" ? "…" : total}</div>
        </div>
      </div>

      {state.kind === "loading" ? (
        <p className="muted" style={{ marginTop: 12 }}>
          Carregando usuários…
        </p>
      ) : (
        <>
          <table>
            <thead>
              <tr>
                <th>Handle</th>
                <th>ID</th>
                <th>Criado em</th>
              </tr>
            </thead>
            <tbody>
              {data!.players.map((p) => (
                <tr key={p.id}>
                  <td>{p.handle}</td>
                  <td className="muted">{p.id.slice(0, 8)}…</td>
                  <td>{new Date(p.createdAt).toLocaleString("pt-BR")}</td>
                </tr>
              ))}
              {data!.players.length === 0 && (
                <tr>
                  <td colSpan={3} className="muted">
                    Nenhum usuário cadastrado.
                  </td>
                </tr>
              )}
            </tbody>
          </table>

          <div className="search-row" style={{ marginTop: 12 }}>
            <button disabled={!canPrev} onClick={() => setOffset(Math.max(0, offset - PAGE))}>
              ← Anterior
            </button>
            <span className="muted">
              {Math.min(offset + 1, total)}–{Math.min(offset + PAGE, total)} de {total}
            </span>
            <button disabled={!canNext} onClick={() => setOffset(offset + PAGE)}>
              Próxima →
            </button>
          </div>
        </>
      )}
    </div>
  );
}

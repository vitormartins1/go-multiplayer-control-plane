import { useEffect, useState } from "react";
import { api, HttpError, type Game, type CatalogAchievement } from "../lib/api";

// GamesPanel: hub do multi-tenant — registrar/listar/selecionar jogo e, do jogo
// selecionado, ver o catálogo + cadastrar conquista.
export function GamesPanel({
  selectedGame,
  onSelectGame,
}: {
  selectedGame: Game | null;
  onSelectGame: (g: Game | null) => void;
}) {
  const [games, setGames] = useState<Game[] | undefined>();
  const [gamesErr, setGamesErr] = useState<string>();

  function loadGames(select?: Game) {
    api
      .games()
      .then((gs) => {
        setGames(gs);
        if (select) onSelectGame(select);
      })
      .catch(() => setGamesErr("Falha ao carregar jogos (game-registry indisponível?)."));
  }
  useEffect(() => {
    loadGames();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div>
      <div style={{ display: "flex", gap: 24, flexWrap: "wrap" }}>
        <div style={{ flex: "1 1 320px" }}>
          <h3>Jogos</h3>
          {gamesErr && <p style={{ color: "var(--down)" }}>{gamesErr}</p>}
          {!games && !gamesErr && <p className="muted">Carregando…</p>}
          {games && games.length === 0 && <p className="muted">Nenhum jogo registrado ainda.</p>}
          {games && games.length > 0 && (
            <table>
              <thead>
                <tr>
                  <th>Nome</th>
                  <th>Slug</th>
                </tr>
              </thead>
              <tbody>
                {games.map((g) => (
                  <tr
                    key={g.id}
                    onClick={() => onSelectGame(g)}
                    style={{
                      cursor: "pointer",
                      background: selectedGame?.id === g.id ? "var(--bg-elev2)" : undefined,
                    }}
                  >
                    <td>{g.name}</td>
                    <td className="muted">{g.slug}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
          <RegisterGame onCreated={(g) => loadGames(g)} />
        </div>

        <div style={{ flex: "1 1 360px" }}>
          {selectedGame ? (
            <GameCatalog game={selectedGame} />
          ) : (
            <p className="muted">Selecione um jogo para ver e gerir o catálogo de conquistas.</p>
          )}
        </div>
      </div>
    </div>
  );
}

function RegisterGame({ onCreated }: { onCreated: (g: Game) => void }) {
  const [slug, setSlug] = useState("");
  const [name, setName] = useState("");
  const [err, setErr] = useState<string>();
  const [busy, setBusy] = useState(false);

  async function submit() {
    setErr(undefined);
    setBusy(true);
    try {
      const g = await api.createGame(slug.trim(), name.trim());
      setSlug("");
      setName("");
      onCreated(g);
    } catch (e) {
      setErr(messageFor(e, "jogo"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="iter2" style={{ borderStyle: "solid", marginTop: 16 }}>
      <h4>Registrar jogo</h4>
      <div className="search-row" style={{ flexWrap: "wrap" }}>
        <input placeholder="slug (ex.: meu-jogo)" value={slug} onChange={(e) => setSlug(e.target.value)} />
        <input placeholder="nome" value={name} onChange={(e) => setName(e.target.value)} />
        <button disabled={busy || !slug || !name} onClick={submit}>
          Registrar
        </button>
      </div>
      {err && <p style={{ color: "var(--down)", marginBottom: 0 }}>{err}</p>}
    </div>
  );
}

function GameCatalog({ game }: { game: Game }) {
  const [catalog, setCatalog] = useState<CatalogAchievement[] | undefined>();
  const [err, setErr] = useState<string>();

  function load() {
    setErr(undefined);
    api
      .gameCatalog(game.id)
      .then(setCatalog)
      .catch(() => setErr("Falha ao carregar o catálogo (achievements indisponível?)."));
  }
  useEffect(() => {
    setCatalog(undefined);
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [game.id]);

  return (
    <div>
      <h3>
        Catálogo — <span className="muted">{game.name}</span>
      </h3>
      {err && <p style={{ color: "var(--down)" }}>{err}</p>}
      {!catalog && !err && <p className="muted">Carregando…</p>}
      {catalog && catalog.length === 0 && <p className="muted">Nenhuma conquista neste jogo ainda.</p>}
      {catalog && catalog.length > 0 && (
        <table>
          <thead>
            <tr>
              <th>Id</th>
              <th>Nome</th>
              <th>Meta</th>
            </tr>
          </thead>
          <tbody>
            {catalog.map((a) => (
              <tr key={a.id}>
                <td className="muted">{a.id}</td>
                <td>{a.name}</td>
                <td>{a.target}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      <AddAchievement gameId={game.id} onAdded={load} />
    </div>
  );
}

function AddAchievement({ gameId, onAdded }: { gameId: string; onAdded: () => void }) {
  const [f, setF] = useState({ id: "", name: "", description: "", eventType: "", target: 1 });
  const [err, setErr] = useState<string>();
  const [busy, setBusy] = useState(false);

  async function submit() {
    setErr(undefined);
    setBusy(true);
    try {
      await api.createAchievement(gameId, { ...f, target: Number(f.target) || 1 });
      setF({ id: "", name: "", description: "", eventType: "", target: 1 });
      onAdded();
    } catch (e) {
      setErr(messageFor(e, "conquista"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="iter2" style={{ borderStyle: "solid", marginTop: 16 }}>
      <h4>Adicionar conquista</h4>
      <div style={{ display: "grid", gap: 8, maxWidth: 420 }}>
        <input placeholder="id (ex.: first_win)" value={f.id} onChange={(e) => setF({ ...f, id: e.target.value })} />
        <input placeholder="nome" value={f.name} onChange={(e) => setF({ ...f, name: e.target.value })} />
        <input placeholder="descrição" value={f.description} onChange={(e) => setF({ ...f, description: e.target.value })} />
        <input placeholder="eventType (ex.: match_won)" value={f.eventType} onChange={(e) => setF({ ...f, eventType: e.target.value })} />
        <input
          type="number"
          min={1}
          placeholder="target"
          value={f.target}
          onChange={(e) => setF({ ...f, target: Number(e.target.value) })}
        />
        <button disabled={busy || !f.id || !f.name || !f.eventType} onClick={submit}>
          Adicionar
        </button>
      </div>
      {err && <p style={{ color: "var(--down)", marginBottom: 0 }}>{err}</p>}
    </div>
  );
}

function messageFor(e: unknown, what: string): string {
  if (e instanceof HttpError) {
    if (e.status === 409) return what === "jogo" ? "Slug já existe." : "Id já existe neste jogo.";
    if (e.status === 400) return "Dados inválidos (slug/id minúsculo, sem espaços; campos obrigatórios).";
    if (e.status === 502) return "Serviço indisponível.";
  }
  return "Falha ao salvar.";
}

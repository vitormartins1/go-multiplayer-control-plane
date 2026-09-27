import { useEffect, useMemo, useState } from "react";
import {
  Alert,
  Badge,
  Button,
  Group,
  Loader,
  Modal,
  SegmentedControl,
  Select,
  Stack,
  Text,
  Title,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useLocalStorage } from "@mantine/hooks";
import { IconDoorEnter } from "@tabler/icons-react";
import { api, type GameSession, type MultiplayerProfile, type Game } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { usePolling } from "../lib/usePolling";
import { useSettings } from "../lib/settings";
import { shortId } from "../lib/format";
import { hostingOf } from "../lib/hosting";
import { compareSessions, type SortDir, type SortKey } from "../lib/sessionSort";
import { closeEffect } from "../components/Hosting";
import { EmptyState } from "../components/EmptyState";
import { GameCover } from "../components/GameCover";
import { SessionTable } from "../components/SessionTable";

export function Rooms() {
  const { data, loading, error, reload } = useAsync((s) => api.rooms(s), []);
  // Lista de jogos (uma vez, sem polling) só pra montar a capa (GameCover) da
  // 1ª coluna — spec 022. Diferente do cache de `profiles` abaixo (que é
  // per-gameId e sob demanda): aqui já existe um "listar todos" (Fleet.tsx faz
  // o mesmo), não vale a pena buscar jogo por jogo.
  const { data: games } = useAsync((s) => api.games(s), []);
  const gamesById = useMemo(() => new Map((games ?? []).map((g) => [g.id, g])), [games]);
  // Visão (Geral/Por jogo) + ordenação: uma preferência só, persistida
  // (spec 060) — sobrevive a reload, não é reset por padrão a cada visita.
  const [view, setView] = useLocalStorage<"unified" | "grouped">({
    key: "rooms:view",
    defaultValue: "unified",
  });
  const [sortBy, setSortBy] = useLocalStorage<SortKey>({ key: "rooms:sort-by", defaultValue: "time" });
  const [sortDir, setSortDir] = useLocalStorage<SortDir>({ key: "rooms:sort-dir", defaultValue: "desc" });
  function toggleSort(key: SortKey) {
    if (key === sortBy) {
      setSortDir(sortDir === "asc" ? "desc" : "asc");
    } else {
      setSortBy(key);
      setSortDir("asc");
    }
  }
  const [filter, setFilter] = useState<"all" | "open" | "closed">("open");
  const [modeFilter, setModeFilter] = useState<string | null>(null);
  // filtro de hospedagem (admin-ui/091): só aparece quando há mundo p2p.
  const [hostingFilter, setHostingFilter] = useState<"all" | "dedicated" | "p2p">("all");
  const [lastUpdate, setLastUpdate] = useState<Date | null>(null);
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [closing, setClosing] = useState<GameSession | null>(null);
  const [closeLoading, setCloseLoading] = useState(false);
  // cache de perfis por gameId (battle_royale mostra lobby/modo na sala, BR4).
  const [profiles, setProfiles] = useState<Record<string, MultiplayerProfile | null>>({});

  const { pollingMs } = useSettings();
  usePolling(reload, pollingMs);

  useEffect(() => {
    if (data) setLastUpdate(new Date());
  }, [data]);

  // busca o perfil dos jogos presentes (uma vez por gameId; cacheado). Só o
  // battle_royale usa o rótulo; os demais viram null e não pagam nova busca.
  useEffect(() => {
    if (!data) return;
    const ids = new Set<string>();
    for (const s of [...data.active, ...data.closed]) if (s.gameId) ids.add(s.gameId);
    const missing = [...ids].filter((id) => !(id in profiles));
    if (missing.length === 0) return;
    let cancelled = false;
    Promise.all(
      missing.map((id) =>
        api
          .getMultiplayerProfile(id)
          .then((p) => [id, p] as const)
          .catch(() => [id, null] as const),
      ),
    ).then((pairs) => {
      if (cancelled) return;
      setProfiles((prev) => {
        const next = { ...prev };
        for (const [id, p] of pairs) next[id] = p;
        return next;
      });
    });
    return () => {
      cancelled = true;
    };
  }, [data, profiles]);

  // brInfo devolve o rótulo de lobby/modo se o jogo da sala é battle_royale.
  function brInfo(s: GameSession): { lobbySize: number; mode: string } | null {
    const p = s.gameId ? profiles[s.gameId] : null;
    if (!p || p.archetype !== "battle_royale") return null;
    const teamSize = p.params.teamSize;
    return { lobbySize: p.params.lobbySize, mode: teamSize === 1 ? "Solo/FFA" : `Squads de ${teamSize}` };
  }

  // coopInfo devolve o rótulo do time se o jogo da sala é coop_mission.
  function coopInfo(s: GameSession): { teamSize: number } | null {
    const p = s.gameId ? profiles[s.gameId] : null;
    if (!p || p.archetype !== "coop_mission") return null;
    return { teamSize: p.params.teamSize };
  }

  // modo de cada sessão (ADR-0024, 028): ausente em sessões legadas = default.
  const modeOf = (s: GameSession) => s.modeId || "default";

  const allRows = useMemo(() => {
    if (!data) return [];
    const active = filter === "closed" ? [] : data.active;
    const closed = filter === "open" ? [] : data.closed;
    return [...active, ...closed];
  }, [data, filter]);

  // filtro por modo só aparece com ≥2 modos distintos na lista (sem ruído).
  const distinctModes = useMemo(() => [...new Set(allRows.map(modeOf))].sort(), [allRows]);
  const hasP2P = useMemo(() => allRows.some((s) => hostingOf(s) === "p2p"), [allRows]);
  const rows = useMemo(() => {
    const byMode = modeFilter ? allRows.filter((s) => modeOf(s) === modeFilter) : allRows;
    const filtered =
      hasP2P && hostingFilter !== "all" ? byMode.filter((s) => hostingOf(s) === hostingFilter) : byMode;
    // Array.prototype.sort é estável (ECMA2019+) — itens iguais no critério
    // mantêm a ordem original da API, de propósito (spec 060, item 11).
    return [...filtered].sort((a, b) => compareSessions(a, b, sortBy, sortDir, gamesById));
  }, [allRows, modeFilter, hasP2P, hostingFilter, sortBy, sortDir, gamesById]);

  // Agrupamento por jogo (spec 060, visão "Por jogo") — só sobre `rows` já
  // filtradas/ordenadas; jogo sem sessão no filtro atual não gera seção
  // (item 3). Seções ordenadas por contagem desc, empate por nome (item 5).
  const groupedRows = useMemo(() => {
    const byGame = new Map<string, { game: Game | undefined; gameName: string; sessions: GameSession[] }>();
    for (const s of rows) {
      const gid = s.gameId || "—";
      const entry = byGame.get(gid);
      if (entry) {
        entry.sessions.push(s);
      } else {
        byGame.set(gid, {
          game: s.gameId ? gamesById.get(s.gameId) : undefined,
          gameName: s.gameName || s.gameId || "Jogo desconhecido",
          sessions: [s],
        });
      }
    }
    return [...byGame.values()].sort(
      (a, b) => b.sessions.length - a.sessions.length || a.gameName.localeCompare(b.gameName),
    );
  }, [rows, gamesById]);

  async function confirmClose() {
    if (!closing) return;
    setCloseLoading(true);
    try {
      // host_world_dropin nunca tem matchId (identidade = sessionId) — sem o
      // fallback, o force-close dessas salas nunca resolvia (spec 024).
      await api.closeRoom(closing.matchId || closing.sessionId);
      notifications.show({ color: "nominal", message: "Sala encerrada." });
      setClosing(null);
      reload();
    } catch {
      notifications.show({ color: "critical", message: "Falha ao encerrar (sala já fechada ou serviço fora?)." });
    } finally {
      setCloseLoading(false);
    }
  }

  // props comuns às tabelas (visão Geral e cada seção Por jogo)
  const tableProps = {
    gamesById,
    sort: { by: sortBy, dir: sortDir, onSort: toggleSort },
    modeExtra: roomModeExtra,
    capacityOf: (s: GameSession) => brInfo(s)?.lobbySize ?? coopInfo(s)?.teamSize,
    expandedId,
    onToggle: (id: string) => setExpandedId((prev) => (prev === id ? null : id)),
    onClose: setClosing,
  };

  // rótulos de lobby (battle_royale) e time (coop_mission) abaixo do modo
  function roomModeExtra(s: GameSession) {
    const br = brInfo(s);
    const coop = coopInfo(s);
    return (
      <>
        {br && (
          <Badge size="xs" variant="light" color="grape">
            lobby {s.players.length}/{br.lobbySize} · {br.mode}
          </Badge>
        )}
        {coop && (
          <Badge size="xs" variant="light" color="teal">
            co-op {s.players.length}/{coop.teamSize}
          </Badge>
        )}
      </>
    );
  }

  return (
    <Stack>
      <Group justify="space-between">
        <div>
          {/* Title order={3} (padronização, 2026-07-13): mesma convenção das
              outras páginas do nav (Games/Players/Analytics) — antes usava
              Text size="lg", visivelmente menor (corpo, não a fonte de
              display do tema). */}
          <Title order={3}>Rooms</Title>
          <Text size="sm" c="dimmed" mt={-4}>
            Salas ao vivo de todos os jogos{" "}
            {data ? `· ${data.active.length} aberta(s), ${data.closed.length} fechada(s)` : ""}
          </Text>
        </div>
        <Group gap="sm">
          {lastUpdate && (
            <Text size="xs" c="dimmed">
              atualizado {lastUpdate.toLocaleTimeString("pt-BR")}
            </Text>
          )}
          {distinctModes.length > 1 && (
            <Select
              size="xs"
              w={170}
              placeholder="Todos os modos"
              clearable
              value={modeFilter}
              onChange={setModeFilter}
              data={distinctModes}
            />
          )}
          {hasP2P && (
            <SegmentedControl
              size="xs"
              value={hostingFilter}
              onChange={(v) => setHostingFilter(v as typeof hostingFilter)}
              data={[
                { label: "Toda hospedagem", value: "all" },
                { label: "Dedicado", value: "dedicated" },
                { label: "P2P", value: "p2p" },
              ]}
            />
          )}
          <SegmentedControl
            size="xs"
            value={filter}
            onChange={(v) => setFilter(v as typeof filter)}
            data={[
              { label: "Abertas", value: "open" },
              { label: "Fechadas", value: "closed" },
              { label: "Todas", value: "all" },
            ]}
          />
          <SegmentedControl
            size="xs"
            value={view}
            onChange={(v) => setView(v as typeof view)}
            data={[
              { label: "Geral", value: "unified" },
              { label: "Por jogo", value: "grouped" },
            ]}
          />
        </Group>
      </Group>

      {loading && !data && <Loader />}
      {!!error && <Alert color="critical">salas indisponíveis (session-service fora?).</Alert>}
      {data && rows.length === 0 && (
        <EmptyState
          icon={IconDoorEnter}
          message={`Nenhuma sala ${filter === "open" ? "aberta" : filter === "closed" ? "fechada" : "por aqui"} — salas aparecem ao vivo quando um match é formado.`}
        />
      )}
      {rows.length > 0 && view === "unified" && (
        <SessionTable rows={rows} showGame showArchetype {...tableProps} />
      )}
      {rows.length > 0 && view === "grouped" && (
        <Stack gap="lg">
          {groupedRows.map((g) => (
            <Stack key={g.game?.id ?? g.gameName} gap="xs">
              <Group gap="sm">
                {g.game && <GameCover game={g.game} variant="mini" />}
                <Text fw={600}>{g.gameName}</Text>
                <Badge variant="light" color="gray">
                  {g.sessions.length}
                </Badge>
              </Group>
              <SessionTable rows={g.sessions} showArchetype {...tableProps} />
            </Stack>
          ))}
        </Stack>
      )}

      <Modal opened={!!closing} onClose={() => setClosing(null)} title="Encerrar sala" centered>
        <Stack>
          <Text size="sm">
            Encerrar {closing?.origin === "host_world" ? "o mundo" : "a sala do match"}{" "}
            <Text span ff="monospace">
              {closing ? shortId(closing.matchId || closing.sessionId) : ""}
            </Text>
            {closing?.gameName ? (
              <>
                {" "}do jogo <Text span fw={600}>{closing.gameName}</Text>
              </>
            ) : null}
            ? {closing ? closeEffect(closing) : ""}
          </Text>
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setClosing(null)} disabled={closeLoading}>
              Cancelar
            </Button>
            <Button color="critical" onClick={confirmClose} loading={closeLoading}>
              Encerrar sala
            </Button>
          </Group>
        </Stack>
      </Modal>
    </Stack>
  );
}

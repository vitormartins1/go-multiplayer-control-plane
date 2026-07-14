import { useEffect, useMemo, useState } from "react";
import {
  Alert,
  Badge,
  Box,
  Button,
  Collapse,
  Group,
  Loader,
  Modal,
  SegmentedControl,
  Select,
  Stack,
  Table,
  Text,
  Title,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useLocalStorage } from "@mantine/hooks";
import { IconChevronDown, IconChevronUp } from "@tabler/icons-react";
import { api, type GameSession, type RoleSlot, type MultiplayerProfile, type Game } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { closeReasonLabel, formatDuration, truncate } from "../lib/format";
import { SessionAvatar } from "../components/SessionAvatar";
import { GameCover } from "../components/GameCover";
import { PlayersSummary } from "../components/PlayersSummary";
import { ROLE_META, SessionMatchDetail } from "../components/SessionMatchDetail";
import { archetypeMetaOf } from "../lib/archetypeMeta";

const POLL_MS = 5000;

// Ordenação (spec 060): única, compartilhada entre a visão Geral e cada
// seção da visão Por jogo (item 9 da spec) — evita um estado de sort por
// jogo, que combinaria mal com N jogos.
type SortKey = "game" | "archetype" | "mode" | "state" | "players" | "time";
type SortDir = "asc" | "desc";

// compareSessions é o comparador puro usado tanto pela tabela Geral quanto
// por cada seção Por jogo. `gamesById` só é necessário pro sortBy "game"
// (nome do jogo pode não estar na própria sessão em toda resposta antiga).
function compareSessions(
  a: GameSession,
  b: GameSession,
  sortBy: SortKey,
  sortDir: SortDir,
  gamesById: Map<string, Game>,
): number {
  const dir = sortDir === "asc" ? 1 : -1;
  switch (sortBy) {
    case "game": {
      const an = a.gameName || gamesById.get(a.gameId ?? "")?.name || a.gameId || "";
      const bn = b.gameName || gamesById.get(b.gameId ?? "")?.name || b.gameId || "";
      return an.localeCompare(bn) * dir;
    }
    case "archetype": {
      // Item 13 da spec: sem arquétipo conhecido vai pro fim SEMPRE, nos dois
      // sentidos — não inverte com a direção (por isso não multiplica por `dir`).
      const al = archetypeMetaOf(a.archetype)?.label;
      const bl = archetypeMetaOf(b.archetype)?.label;
      if (!al && !bl) return 0;
      if (!al) return 1;
      if (!bl) return -1;
      return al.localeCompare(bl) * dir;
    }
    case "mode":
      return (a.modeId || "default").localeCompare(b.modeId || "default") * dir;
    case "state": {
      // Item 12: open antes de closed, não alfabético ("closed" < "open" seria
      // contra-intuitivo — quem abre a aba geralmente quer as vivas primeiro).
      const rank = (s: GameSession) => (s.state === "open" ? 0 : 1);
      return (rank(a) - rank(b)) * dir;
    }
    case "players":
      return (a.players.length - b.players.length) * dir;
    case "time":
      // Item 15: só openedAt, sempre — nunca compara duração (fechada) com
      // "há quanto tempo está aberta" (unidades diferentes, ver spec/plan).
      return (Date.parse(a.openedAt) - Date.parse(b.openedAt)) * dir;
    default:
      return 0;
  }
}

// SortableTh é um <Table.Th> clicável com seta de direção quando ativo —
// usado nos 6 cabeçalhos ordenáveis (spec 060, item 7/8).
function SortableTh({
  label,
  sortKey,
  active,
  dir,
  onSort,
}: {
  label: string;
  sortKey: SortKey;
  active: SortKey;
  dir: SortDir;
  onSort: (key: SortKey) => void;
}) {
  const isActive = active === sortKey;
  return (
    <Table.Th
      style={{ cursor: "pointer", userSelect: "none" }}
      onClick={() => onSort(sortKey)}
    >
      <Group gap={4} wrap="nowrap">
        <Text size="sm" fw={isActive ? 700 : undefined}>
          {label}
        </Text>
        {isActive && (dir === "asc" ? <IconChevronUp size={12} /> : <IconChevronDown size={12} />)}
      </Group>
    </Table.Th>
  );
}

// Rooms é o board global de salas do console: as sessões ativas de TODOS os
// jogos (spec 009), com detalhe e a ação de encerrar uma sala travada. A aba
// Sessions por-jogo (dentro de GameDetail) segue como recorte por jogo.
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
  const [lastUpdate, setLastUpdate] = useState<Date | null>(null);
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [closing, setClosing] = useState<GameSession | null>(null);
  const [closeLoading, setCloseLoading] = useState(false);
  // cache de perfis por gameId (battle_royale mostra lobby/modo na sala, BR4).
  const [profiles, setProfiles] = useState<Record<string, MultiplayerProfile | null>>({});

  useEffect(() => {
    const id = setInterval(reload, POLL_MS);
    return () => clearInterval(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

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
  const rows = useMemo(() => {
    const filtered = modeFilter ? allRows.filter((s) => modeOf(s) === modeFilter) : allRows;
    // Array.prototype.sort é estável (ECMA2019+) — itens iguais no critério
    // mantêm a ordem original da API, de propósito (spec 060, item 11).
    return [...filtered].sort((a, b) => compareSessions(a, b, sortBy, sortDir, gamesById));
  }, [allRows, modeFilter, sortBy, sortDir, gamesById]);

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
        <Text c="dimmed">
          Nenhuma sala {filter === "open" ? "aberta" : filter === "closed" ? "fechada" : "por aqui"} — salas
          aparecem ao vivo quando um match é formado.
        </Text>
      )}
      {rows.length > 0 && view === "unified" && (
        <RoomsTable
          rows={rows}
          showGameColumn
          sortBy={sortBy}
          sortDir={sortDir}
          onSort={toggleSort}
          gamesById={gamesById}
          brInfo={brInfo}
          coopInfo={coopInfo}
          expandedId={expandedId}
          onToggle={(id) => setExpandedId((prev) => (prev === id ? null : id))}
          onCloseRequest={setClosing}
        />
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
              <RoomsTable
                rows={g.sessions}
                showGameColumn={false}
                sortBy={sortBy}
                sortDir={sortDir}
                onSort={toggleSort}
                gamesById={gamesById}
                brInfo={brInfo}
                coopInfo={coopInfo}
                expandedId={expandedId}
                onToggle={(id) => setExpandedId((prev) => (prev === id ? null : id))}
                onCloseRequest={setClosing}
              />
            </Stack>
          ))}
        </Stack>
      )}

      <Modal opened={!!closing} onClose={() => setClosing(null)} title="Encerrar sala" centered>
        <Stack>
          <Text size="sm">
            Encerrar a sala do match <Text span ff="monospace">{closing ? truncate(closing.matchId) : ""}</Text>
            {closing?.gameName ? (
              <>
                {" "}do jogo <Text span fw={600}>{closing.gameName}</Text>
              </>
            ) : null}
            ? O game server é liberado e os jogadores conectados caem.
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

// RoomsTable é a tabela reutilizável (spec 060) — mesma renderização servindo
// a visão Geral (`showGameColumn=true`) e cada seção da visão Por jogo
// (`showGameColumn=false`, sem repetir a coluna "Jogo" já no cabeçalho da seção).
function RoomsTable({
  rows,
  showGameColumn,
  sortBy,
  sortDir,
  onSort,
  gamesById,
  brInfo,
  coopInfo,
  expandedId,
  onToggle,
  onCloseRequest,
}: {
  rows: GameSession[];
  showGameColumn: boolean;
  sortBy: SortKey;
  sortDir: SortDir;
  onSort: (key: SortKey) => void;
  gamesById: Map<string, Game>;
  brInfo: (s: GameSession) => { lobbySize: number; mode: string } | null;
  coopInfo: (s: GameSession) => { teamSize: number } | null;
  expandedId: string | null;
  onToggle: (id: string) => void;
  onCloseRequest: (s: GameSession) => void;
}) {
  return (
    <Table.ScrollContainer minWidth={720}>
      <Table striped withTableBorder>
        <Table.Thead>
          <Table.Tr>
            <Table.Th />
            {showGameColumn && (
              <SortableTh label="Jogo" sortKey="game" active={sortBy} dir={sortDir} onSort={onSort} />
            )}
            <SortableTh label="Arquétipo" sortKey="archetype" active={sortBy} dir={sortDir} onSort={onSort} />
            <SortableTh label="Modo" sortKey="mode" active={sortBy} dir={sortDir} onSort={onSort} />
            <Table.Th>Match</Table.Th>
            <SortableTh label="Estado" sortKey="state" active={sortBy} dir={sortDir} onSort={onSort} />
            <SortableTh label="Players" sortKey="players" active={sortBy} dir={sortDir} onSort={onSort} />
            <Table.Th>Server</Table.Th>
            <SortableTh label="Tempos" sortKey="time" active={sortBy} dir={sortDir} onSort={onSort} />
            <Table.Th />
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {rows.map((s) => (
            <RoomRow
              key={s.sessionId}
              session={s}
              game={s.gameId ? gamesById.get(s.gameId) : undefined}
              showGameColumn={showGameColumn}
              br={brInfo(s)}
              coop={coopInfo(s)}
              expanded={expandedId === s.sessionId}
              onToggle={() => onToggle(s.sessionId)}
              onClose={() => onCloseRequest(s)}
            />
          ))}
        </Table.Tbody>
      </Table>
    </Table.ScrollContainer>
  );
}

function RoomRow({
  session: s,
  game,
  showGameColumn,
  br,
  coop,
  expanded,
  onToggle,
  onClose,
}: {
  session: GameSession;
  game: Game | undefined;
  showGameColumn: boolean;
  br: { lobbySize: number; mode: string } | null;
  coop: { teamSize: number } | null;
  expanded: boolean;
  onToggle: () => void;
  onClose: () => void;
}) {
  const modeMeta = archetypeMetaOf(s.archetype);
  const colSpan = showGameColumn ? 10 : 9;
  return (
    <>
      <Table.Tr
        style={{ cursor: "pointer" }}
        bg={expanded ? "var(--mantine-color-default-hover)" : undefined}
        onClick={onToggle}
      >
        <Table.Td>
          {game ? (
            <Box style={{ position: "relative", width: 28, height: 28 }}>
              <GameCover game={game} variant="mini" />
              <Box
                style={{
                  position: "absolute",
                  bottom: -4,
                  right: -4,
                  borderRadius: 8,
                  boxShadow: "0 0 0 2px var(--mantine-color-body)",
                }}
              >
                <SessionAvatar seed={s.profileHash || s.sessionId} size={16} />
              </Box>
            </Box>
          ) : (
            <SessionAvatar seed={s.profileHash || s.sessionId} size={16} />
          )}
        </Table.Td>
        {showGameColumn && (
          <Table.Td>
            <Text size="sm" fw={500}>
              {s.gameName || truncate(s.gameId || "—")}
            </Text>
          </Table.Td>
        )}
        <Table.Td>
          {modeMeta ? (
            <Badge size="xs" variant="light" color="gray" leftSection={<modeMeta.icon size={10} />}>
              {modeMeta.label}
            </Badge>
          ) : (
            <Text size="sm" c="dimmed">
              —
            </Text>
          )}
        </Table.Td>
        <Table.Td>
          <Badge size="xs" variant="light" color="gray">
            {s.modeId || "default"}
          </Badge>
          {br && (
            <Badge size="xs" variant="light" color="grape" mt={2}>
              lobby {s.players.length}/{br.lobbySize} · {br.mode}
            </Badge>
          )}
          {coop && (
            <Badge size="xs" variant="light" color="teal" mt={2}>
              co-op {s.players.length}/{coop.teamSize}
            </Badge>
          )}
        </Table.Td>
        <Table.Td>
          <Group gap={6} wrap="nowrap">
            <Text ff="monospace" size="sm">
              {truncate(s.matchId)}
            </Text>
          </Group>
        </Table.Td>
        <Table.Td>
          {s.state === "open" ? (
            <Badge color="nominal" variant="light">
              ao vivo
            </Badge>
          ) : (
            <Badge color="gray" variant="light">
              closed · {s.closeReason ?? "?"}
            </Badge>
          )}
        </Table.Td>
        <Table.Td>
          {s.origin === "host_world" ? (
            <RoleSlots slots={s.roleSlots ?? []} />
          ) : (
            // resumo compacto (034): a lista completa vive no painel expandido.
            <PlayersSummary players={s.players} capacity={br?.lobbySize ?? coop?.teamSize} />
          )}
        </Table.Td>
        <Table.Td>
          {/* fallback "—" (mesmo achado do GameSessions.tsx): coluna ficava em
              branco em sessão fechada — servidor já desalocado. */}
          <Text ff="monospace" size="sm" c={s.serverAddress ? undefined : "dimmed"}>
            {s.serverAddress ? truncate(s.serverAddress) : "—"}
          </Text>
        </Table.Td>
        <Table.Td>
          {s.state === "open" ? (
            <Text size="sm">há {formatDuration(Date.now() - Date.parse(s.openedAt))}</Text>
          ) : (
            <Text size="sm">
              durou {s.closedAt ? formatDuration(Date.parse(s.closedAt) - Date.parse(s.openedAt)) : "?"}
            </Text>
          )}
        </Table.Td>
        <Table.Td onClick={(e) => e.stopPropagation()}>
          {s.state === "open" && (
            <Button size="compact-xs" color="critical" variant="light" onClick={onClose}>
              Encerrar
            </Button>
          )}
        </Table.Td>
      </Table.Tr>
      {expanded && (
        <Table.Tr>
          <Table.Td colSpan={colSpan} p={0}>
            <Collapse in={expanded}>
              <Box p="md" style={{ borderTop: "1px solid var(--mantine-color-default-border)" }}>
                <Stack gap="xs">
                  <Group gap="xl" wrap="wrap">
                    <SessionAvatar seed={s.profileHash || s.sessionId} size={40} />
                    <Field label="JOGO" value={s.gameName || s.gameId || "—"} />
                    <Field label="MODO" value={s.modeId || "default"} mono />
                    <Field label={s.origin === "host_world" ? "MUNDO" : "MATCH ID"} value={s.matchId || s.sessionId} mono />
                    <Field label="SESSION ID" value={s.sessionId} mono />
                    <Field label="SERVIDOR" value={s.serverAddress || "—"} mono />
                    {s.origin === "host_world" && s.hostPlayerId && (
                      <Field label="HOST" value={s.hostPlayerId} mono />
                    )}
                  </Group>

                  {/* roster/slots/formação/resultado vivem no painel por
                      arquétipo (spec 019) — host_world vira cards por papel. */}
                  <SessionMatchDetail session={s} />

                  <Group gap="xl" wrap="wrap">
                    <Field label="ABERTA EM" value={new Date(s.openedAt).toLocaleString("pt-BR")} />
                    {s.closedAt && (
                      <Field label="FECHADA EM" value={new Date(s.closedAt).toLocaleString("pt-BR")} />
                    )}
                    {s.closeReason && <Field label="MOTIVO" value={closeReasonLabel(s.closeReason)} />}
                  </Group>
                </Stack>
              </Box>
            </Collapse>
          </Table.Td>
        </Table.Tr>
      )}
    </>
  );
}

// RoleSlots mostra a ocupação por papel (co-op X/2, invasor Y/1) — a leitura do
// board para mundos host_world (spec 020); ROLE_META compartilhado com o painel
// expandido (SessionMatchDetail). Slot cheio fica destacado.
function RoleSlots({ slots }: { slots: RoleSlot[] }) {
  if (slots.length === 0) return <Text size="sm" c="dimmed">—</Text>;
  return (
    <Group gap="xs" wrap="wrap">
      {slots.map((slot) => {
        const meta = ROLE_META[slot.role] ?? { label: slot.role, color: "gray" };
        const full = slot.occupied >= slot.capacity;
        return (
          <Badge key={slot.role} color={full ? "gray" : meta.color} variant={full ? "outline" : "light"}>
            {meta.label} {slot.occupied}/{slot.capacity}
          </Badge>
        );
      })}
    </Group>
  );
}

function Field({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <Stack gap={2}>
      <Text size="xs" c="dimmed" fw={600}>
        {label}
      </Text>
      <Text ff={mono ? "monospace" : undefined} size="xs">
        {value}
      </Text>
    </Stack>
  );
}

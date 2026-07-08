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
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { api, type GameSession, type RoleSlot, type MultiplayerProfile } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { closeReasonLabel, formatDuration, truncate } from "../lib/format";
import { PlayerAvatar } from "../components/PlayerAvatar";
import { GuestBadge } from "../components/GuestBadge";
import { SessionAvatar } from "../components/SessionAvatar";
import { PlayersSummary } from "../components/PlayersSummary";
import { SessionMatchDetail } from "../components/SessionMatchDetail";

const POLL_MS = 5000;

// Rooms é o board global de salas do console: as sessões ativas de TODOS os
// jogos (spec 009), com detalhe e a ação de encerrar uma sala travada. A aba
// Sessions por-jogo (dentro de GameDetail) segue como recorte por jogo.
export function Rooms() {
  const { data, loading, error, reload } = useAsync((s) => api.rooms(s), []);
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
  const rows = useMemo(
    () => (modeFilter ? allRows.filter((s) => modeOf(s) === modeFilter) : allRows),
    [allRows, modeFilter],
  );

  async function confirmClose() {
    if (!closing) return;
    setCloseLoading(true);
    try {
      await api.closeRoom(closing.matchId);
      notifications.show({ color: "green", message: "Sala encerrada." });
      setClosing(null);
      reload();
    } catch {
      notifications.show({ color: "red", message: "Falha ao encerrar (sala já fechada ou serviço fora?)." });
    } finally {
      setCloseLoading(false);
    }
  }

  return (
    <Stack>
      <Group justify="space-between">
        <div>
          <Text fw={600} size="lg">
            Rooms
          </Text>
          <Text size="sm" c="dimmed">
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
        </Group>
      </Group>

      {loading && !data && <Loader />}
      {!!error && <Alert color="red">salas indisponíveis (session-service fora?).</Alert>}
      {data && rows.length === 0 && (
        <Text c="dimmed">
          Nenhuma sala {filter === "open" ? "aberta" : filter === "closed" ? "fechada" : "por aqui"} — salas
          aparecem ao vivo quando um match é formado.
        </Text>
      )}
      {rows.length > 0 && (
        <Table striped withTableBorder>
          <Table.Thead>
            <Table.Tr>
              <Table.Th>Jogo</Table.Th>
              <Table.Th>Match</Table.Th>
              <Table.Th>Estado</Table.Th>
              <Table.Th>Players</Table.Th>
              <Table.Th>Server</Table.Th>
              <Table.Th>Tempos</Table.Th>
              <Table.Th />
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {rows.map((s) => (
              <RoomRow
                key={s.sessionId}
                session={s}
                br={brInfo(s)}
                coop={coopInfo(s)}
                expanded={expandedId === s.sessionId}
                onToggle={() => setExpandedId((prev) => (prev === s.sessionId ? null : s.sessionId))}
                onClose={() => setClosing(s)}
              />
            ))}
          </Table.Tbody>
        </Table>
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
            <Button color="red" onClick={confirmClose} loading={closeLoading}>
              Encerrar sala
            </Button>
          </Group>
        </Stack>
      </Modal>
    </Stack>
  );
}

function RoomRow({
  session: s,
  br,
  coop,
  expanded,
  onToggle,
  onClose,
}: {
  session: GameSession;
  br: { lobbySize: number; mode: string } | null;
  coop: { teamSize: number } | null;
  expanded: boolean;
  onToggle: () => void;
  onClose: () => void;
}) {
  return (
    <>
      <Table.Tr
        style={{ cursor: "pointer" }}
        bg={expanded ? "var(--mantine-color-default-hover)" : undefined}
        onClick={onToggle}
      >
        <Table.Td>
          <Text size="sm" fw={500}>
            {s.gameName || truncate(s.gameId || "—")}
          </Text>
          <Badge size="xs" variant="light" color="indigo" mt={2} mr={4}>
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
            <SessionAvatar seed={s.sessionId} />
            <Text ff="monospace" size="sm">
              {truncate(s.matchId)}
            </Text>
          </Group>
        </Table.Td>
        <Table.Td>
          {s.state === "open" ? (
            <Badge color="green" variant="light">
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
          <Text ff="monospace" size="sm">
            {truncate(s.serverAddress)}
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
            <Button size="compact-xs" color="red" variant="light" onClick={onClose}>
              Encerrar
            </Button>
          )}
        </Table.Td>
      </Table.Tr>
      {expanded && (
        <Table.Tr>
          <Table.Td colSpan={7} p={0}>
            <Collapse in={expanded}>
              <Box p="md" style={{ borderTop: "1px solid var(--mantine-color-default-border)" }}>
                <Stack gap="xs">
                  <Group gap="xl" wrap="wrap">
                    <SessionAvatar seed={s.sessionId} size={40} />
                    <Field label="JOGO" value={s.gameName || s.gameId || "—"} />
                    <Field label="MODO" value={s.modeId || "default"} mono />
                    <Field label={s.origin === "host_world" ? "MUNDO" : "MATCH ID"} value={s.matchId || s.sessionId} mono />
                    <Field label="SESSION ID" value={s.sessionId} mono />
                    <Field label="SERVIDOR" value={s.serverAddress || "—"} mono />
                    {s.origin === "host_world" && s.hostPlayerId && (
                      <Field label="HOST" value={s.hostPlayerId} mono />
                    )}
                  </Group>

                  {s.origin === "host_world" && (s.roleSlots?.length ?? 0) > 0 && (
                    <Stack gap={2}>
                      <Text size="xs" c="dimmed" fw={600}>
                        SLOTS POR PAPEL
                      </Text>
                      <RoleSlots slots={s.roleSlots ?? []} />
                    </Stack>
                  )}

                  <Stack gap={2}>
                    <Text size="xs" c="dimmed" fw={600}>
                      PLAYERS
                    </Text>
                    <Group gap="md" wrap="wrap">
                      {s.players.map((p) => (
                        <Group key={p.id} gap={6} wrap="nowrap">
                          <PlayerAvatar seed={p.id} size={24} />
                          <Stack gap={0}>
                            <Group gap={4} wrap="nowrap">
                              <Text size="xs" fw={500}>
                                {p.handle || "—"}
                              </Text>
                              <GuestBadge playerId={p.id} />
                            </Group>
                            <Text ff="monospace" size="xs" c="dimmed">
                              {p.id}
                            </Text>
                          </Stack>
                        </Group>
                      ))}
                    </Group>
                  </Stack>

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

// rótulos e cores dos papéis do host_world_dropin.
const ROLE_META: Record<string, { label: string; color: string }> = {
  coop: { label: "co-op", color: "teal" },
  invader: { label: "invasor", color: "red" },
  host: { label: "host", color: "blue" },
};

// RoleSlots mostra a ocupação por papel (co-op X/2, invasor Y/1) — a leitura do
// board para mundos host_world (spec 020). Slot cheio fica destacado.
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

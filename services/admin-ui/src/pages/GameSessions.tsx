import { useEffect, useMemo, useState } from "react";
import {
  Alert,
  Badge,
  Box,
  Collapse,
  Group,
  Loader,
  SegmentedControl,
  Stack,
  Table,
  Text,
  Tooltip,
} from "@mantine/core";
import { api, type GameSession } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { PlayerAvatar } from "../components/PlayerAvatar";

const POLL_MS = 5000;

// GameSessions é a aba de operação/debug das partidas do jogo (spec 007):
// abertas ao vivo + histórico de fechadas (em memória no session-service).
// Polling de 5s enquanto a aba está montada (GameDetail usa keepMounted=false,
// então sair da aba desmonta e para o polling).
export function GameSessions({ gameId }: { gameId: string }) {
  const { data, loading, error, reload } = useAsync((s) => api.gameSessions(gameId, s), [gameId]);
  const [filter, setFilter] = useState<"all" | "open" | "closed">("all");
  const [lastUpdate, setLastUpdate] = useState<Date | null>(null);
  const [expandedId, setExpandedId] = useState<string | null>(null);

  useEffect(() => {
    const id = setInterval(reload, POLL_MS);
    return () => clearInterval(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [gameId]);

  useEffect(() => {
    if (data) setLastUpdate(new Date());
  }, [data]);

  const rows = useMemo(() => {
    if (!data) return [];
    const active = filter === "closed" ? [] : data.active;
    const closed = filter === "open" ? [] : data.closed;
    return [...active, ...closed];
  }, [data, filter]);

  return (
    <Stack>
      <Group justify="space-between">
        <Text fw={600}>
          Partidas{" "}
          {data ? (
            <Text span c="dimmed">
              · {data.active.length} aberta(s), {data.closed.length} fechada(s)
            </Text>
          ) : null}
        </Text>
        <Group gap="sm">
          {lastUpdate && (
            <Text size="xs" c="dimmed">
              atualizado {lastUpdate.toLocaleTimeString("pt-BR")}
            </Text>
          )}
          <SegmentedControl
            size="xs"
            value={filter}
            onChange={(v) => setFilter(v as typeof filter)}
            data={[
              { label: "Todas", value: "all" },
              { label: "Abertas", value: "open" },
              { label: "Fechadas", value: "closed" },
            ]}
          />
        </Group>
      </Group>

      {loading && !data && <Loader />}
      {!!error && <Alert color="red">sessões indisponíveis (session-service fora?).</Alert>}
      {data && rows.length === 0 && (
        <Text c="dimmed">
          Nenhuma partida por aqui — abertas aparecem ao vivo quando um match é formado. O histórico de
          fechadas vive em memória no session-service (some no restart).
        </Text>
      )}
      {rows.length > 0 && (
        <Table striped withTableBorder>
          <Table.Thead>
            <Table.Tr>
              <Table.Th>Match</Table.Th>
              <Table.Th>Estado</Table.Th>
              <Table.Th>Players</Table.Th>
              <Table.Th>Server</Table.Th>
              <Table.Th>Tempos</Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {rows.map((s) => (
              <SessionRow
                key={s.sessionId}
                session={s}
                expanded={expandedId === s.sessionId}
                onToggle={() => setExpandedId((prev) => (prev === s.sessionId ? null : s.sessionId))}
              />
            ))}
          </Table.Tbody>
        </Table>
      )}
    </Stack>
  );
}

function SessionRow({
  session: s,
  expanded,
  onToggle,
}: {
  session: GameSession;
  expanded: boolean;
  onToggle: () => void;
}) {
  return (
    <>
      <Table.Tr
        style={{ cursor: "pointer" }}
        bg={expanded ? "var(--mantine-color-default-hover)" : undefined}
        onClick={onToggle}
      >
        <Table.Td>
          <Text ff="monospace" size="sm">
            {truncate(s.matchId)}
          </Text>
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
          <Group gap="xs" wrap="nowrap">
            {s.players.map((p) => (
              <Tooltip key={p.id} label={p.handle ? `${p.handle} (${p.id})` : p.id}>
                <Group gap={6} wrap="nowrap">
                  <PlayerAvatar seed={p.id} />
                  <Text size="sm">{p.handle || truncate(p.id)}</Text>
                </Group>
              </Tooltip>
            ))}
          </Group>
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
              durou {s.closedAt ? formatDuration(Date.parse(s.closedAt) - Date.parse(s.openedAt)) : "?"} · fechou{" "}
              {s.closedAt ? new Date(s.closedAt).toLocaleTimeString("pt-BR") : "?"}
            </Text>
          )}
        </Table.Td>
      </Table.Tr>
      {expanded && (
        <Table.Tr>
          <Table.Td colSpan={5} p={0}>
            <Collapse in={expanded}>
              <Box p="md" style={{ borderTop: "1px solid var(--mantine-color-default-border)" }}>
                <Stack gap="xs">
                  <Group gap="xl" wrap="wrap">
                    <Stack gap={2}>
                      <Text size="xs" c="dimmed" fw={600}>SESSION ID</Text>
                      <Text ff="monospace" size="xs">{s.sessionId}</Text>
                    </Stack>
                    <Stack gap={2}>
                      <Text size="xs" c="dimmed" fw={600}>MATCH ID</Text>
                      <Text ff="monospace" size="xs">{s.matchId}</Text>
                    </Stack>
                    <Stack gap={2}>
                      <Text size="xs" c="dimmed" fw={600}>SERVIDOR</Text>
                      <Text ff="monospace" size="xs">{s.serverAddress || "—"}</Text>
                    </Stack>
                  </Group>

                  <Stack gap={2}>
                    <Text size="xs" c="dimmed" fw={600}>PLAYERS</Text>
                    <Group gap="md" wrap="wrap">
                      {s.players.map((p) => (
                        <Group key={p.id} gap={6} wrap="nowrap">
                          <PlayerAvatar seed={p.id} size={24} />
                          <Stack gap={0}>
                            <Text size="xs" fw={500}>{p.handle || "—"}</Text>
                            <Text ff="monospace" size="xs" c="dimmed">{p.id}</Text>
                          </Stack>
                        </Group>
                      ))}
                    </Group>
                  </Stack>

                  <Group gap="xl" wrap="wrap">
                    <Stack gap={2}>
                      <Text size="xs" c="dimmed" fw={600}>ABERTA EM</Text>
                      <Text size="xs">{new Date(s.openedAt).toLocaleString("pt-BR")}</Text>
                    </Stack>
                    {s.closedAt && (
                      <Stack gap={2}>
                        <Text size="xs" c="dimmed" fw={600}>FECHADA EM</Text>
                        <Text size="xs">{new Date(s.closedAt).toLocaleString("pt-BR")}</Text>
                      </Stack>
                    )}
                    {s.closeReason && (
                      <Stack gap={2}>
                        <Text size="xs" c="dimmed" fw={600}>MOTIVO</Text>
                        <Text size="xs">
                          {s.closeReason === "ttl" ? "TTL expirado (sessão abandonada)" : "Resultado reportado"}
                        </Text>
                      </Stack>
                    )}
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

function truncate(id: string): string {
  return id.length > 12 ? `${id.slice(0, 12)}…` : id;
}

function formatDuration(ms: number): string {
  const totalSec = Math.max(0, Math.floor(ms / 1000));
  const min = Math.floor(totalSec / 60);
  const sec = totalSec % 60;
  if (min >= 60) return `${Math.floor(min / 60)}h ${min % 60}min`;
  if (min > 0) return `${min}min ${sec}s`;
  return `${sec}s`;
}

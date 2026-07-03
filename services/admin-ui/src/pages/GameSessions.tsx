import { useEffect, useMemo, useState } from "react";
import {
  Alert,
  Badge,
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
        <Table striped highlightOnHover withTableBorder>
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
              <SessionRow key={s.sessionId} session={s} />
            ))}
          </Table.Tbody>
        </Table>
      )}
    </Stack>
  );
}

function SessionRow({ session: s }: { session: GameSession }) {
  return (
    <Table.Tr>
      <Table.Td>
        <Tooltip label={`match ${s.matchId} · session ${s.sessionId}`}>
          <Text ff="monospace" size="sm">
            {truncate(s.matchId)}
          </Text>
        </Tooltip>
      </Table.Td>
      <Table.Td>
        {s.state === "open" ? (
          <Badge color="green" variant="light">
            ao vivo
          </Badge>
        ) : (
          <Tooltip label={s.closeReason === "ttl" ? "fechada por TTL (abandonada)" : "fechada por resultado reportado"}>
            <Badge color="gray" variant="light">
              closed · {s.closeReason ?? "?"}
            </Badge>
          </Tooltip>
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
          {s.serverAddress}
        </Text>
      </Table.Td>
      <Table.Td>
        {s.state === "open" ? (
          <Text size="sm">há {formatDuration(Date.now() - Date.parse(s.openedAt))}</Text>
        ) : (
          <Tooltip label={`aberta ${new Date(s.openedAt).toLocaleString("pt-BR")}`}>
            <Text size="sm">
              durou {s.closedAt ? formatDuration(Date.parse(s.closedAt) - Date.parse(s.openedAt)) : "?"} · fechou{" "}
              {s.closedAt ? new Date(s.closedAt).toLocaleTimeString("pt-BR") : "?"}
            </Text>
          </Tooltip>
        )}
      </Table.Td>
    </Table.Tr>
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

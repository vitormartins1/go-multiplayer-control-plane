import { Alert, Loader, SimpleGrid, Stack, Table, Text, Title } from "@mantine/core";
import { api } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { StatCard } from "../components/StatCard";

// rótulos amigáveis dos eventos de participação (host_world_dropin).
const EVENT_LABELS: Record<string, string> = {
  JOINED: "Entrou",
  BOSS_DEFEATED: "Boss derrotado",
  HOST_KILLED: "Host morto",
  DIED: "Morreu",
  LEFT: "Saiu",
};

const ROLE_LABELS: Record<string, string> = {
  host: "Host",
  coop: "Co-op",
  invader: "Invasor",
};

export function GameAnalytics({ gameId }: { gameId: string }) {
  const { data, loading, error } = useAsync((s) => api.gameAnalytics(gameId, s), [gameId]);

  const hasData = data && data.matches > 0;
  const participations = data?.roleParticipations ?? [];

  return (
    <Stack>
      {loading && !data && <Loader />}
      {!!error && !data && <Alert color="red">Analytics indisponível (analytics-service fora?).</Alert>}

      {data && !hasData && participations.length === 0 && (
        <Text c="dimmed">
          Sem dados ainda — os agregados aparecem conforme as partidas terminam e reportam resultado.
        </Text>
      )}

      {hasData && (
        <SimpleGrid cols={{ base: 2, sm: 3 }}>
          <StatCard label="Partidas" value={String(data.matches)} />
          <StatCard label="Jogadores únicos" value={String(data.uniquePlayers)} />
          <StatCard label="Score médio" value={data.avgScore.toFixed(1)} />
          <StatCard label="Vitórias" value={String(data.wins)} />
          <StatCard label="Derrotas" value={String(data.losses)} />
          <StatCard label="Empates" value={String(data.draws)} />
        </SimpleGrid>
      )}

      {/* host_world_dropin (spec 020): participações por papel/evento. Só aparece
          quando o jogo reportou eventos de participação. */}
      {participations.length > 0 && (
        <Stack gap="xs">
          <Title order={5}>Participações por papel</Title>
          <Table striped withTableBorder>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Papel</Table.Th>
                <Table.Th>Evento</Table.Th>
                <Table.Th ta="right">Total</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {participations.map((p) => (
                <Table.Tr key={`${p.role}-${p.event}`}>
                  <Table.Td>{ROLE_LABELS[p.role] ?? p.role}</Table.Td>
                  <Table.Td>{EVENT_LABELS[p.event] ?? p.event}</Table.Td>
                  <Table.Td ta="right">{p.count}</Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </Stack>
      )}
    </Stack>
  );
}

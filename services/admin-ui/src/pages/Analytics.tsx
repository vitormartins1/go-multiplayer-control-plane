import { Alert, Group, Loader, SimpleGrid, Stack, Table, Text, Title } from "@mantine/core";
import { api } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { StatCard } from "../components/StatCard";

// Analytics: overview cross-game alimentado pelo analytics-service (ADR-0021).
// Agregados do resultado das partidas (fan-out) — totais + ranking de jogos.
export function Analytics() {
  const { data, loading, error } = useAsync((s) => api.analyticsOverview(s), []);

  return (
    <Stack>
      <Title order={3}>Analytics</Title>
      <Text size="sm" c="dimmed" mt={-8}>
        Agregados das partidas por jogo (worker pool, ADR-0021).
      </Text>

      {loading && !data && <Loader />}
      {!!error && !data && (
        <Alert color="red">Analytics indisponível (analytics-service fora?).</Alert>
      )}

      {data && (
        <>
          <SimpleGrid cols={{ base: 1, sm: 3 }}>
            <StatCard label="Partidas" value={String(data.totals.matches)} />
            <StatCard label="Jogadores únicos" value={String(data.totals.uniquePlayers)} />
            <StatCard label="Jogos com atividade" value={String(data.totals.games)} />
          </SimpleGrid>

          {data.games.length === 0 ? (
            <Text c="dimmed">
              Sem dados ainda — os agregados aparecem conforme as partidas terminam e reportam resultado.
            </Text>
          ) : (
            <>
              <Title order={5}>Jogos por partidas</Title>
              <Table striped highlightOnHover withTableBorder>
                <Table.Thead>
                  <Table.Tr>
                    <Table.Th>Jogo</Table.Th>
                    <Table.Th ta="right">Partidas</Table.Th>
                    <Table.Th ta="right">Jogadores únicos</Table.Th>
                    <Table.Th ta="right">Vitórias</Table.Th>
                    <Table.Th ta="right">Score médio</Table.Th>
                  </Table.Tr>
                </Table.Thead>
                <Table.Tbody>
                  {data.games.map((g) => (
                    <Table.Tr key={g.gameId}>
                      <Table.Td>
                        <Group gap="xs">
                          <Text fw={500}>{g.gameName || g.gameId}</Text>
                        </Group>
                      </Table.Td>
                      <Table.Td ta="right">{g.matches}</Table.Td>
                      <Table.Td ta="right">{g.uniquePlayers}</Table.Td>
                      <Table.Td ta="right">{g.wins}</Table.Td>
                      <Table.Td ta="right">{g.avgScore.toFixed(1)}</Table.Td>
                    </Table.Tr>
                  ))}
                </Table.Tbody>
              </Table>
            </>
          )}
        </>
      )}
    </Stack>
  );
}

import { Alert, Button, Group, Loader, Stack, Table, Text } from "@mantine/core";
import { IconRefresh } from "@tabler/icons-react";
import { api } from "../lib/api";
import { useAsync } from "../lib/useAsync";

export function GameLeaderboards({ gameId }: { gameId: string }) {
  const { data, loading, error, reload } = useAsync((s) => api.gameLeaderboard(gameId, 20, s), [gameId]);

  return (
    <Stack>
      <Group justify="space-between">
        <Text fw={600}>Ranking do jogo (top 20)</Text>
        <Button variant="default" size="xs" leftSection={<IconRefresh size={14} />} onClick={reload}>
          Atualizar
        </Button>
      </Group>

      {loading && !data && <Loader />}
      {!!error && <Alert color="red">leaderboard indisponível.</Alert>}
      {data && data.length === 0 && (
        <Text c="dimmed">Nenhum score ainda — as partidas reportadas (ReportResult) alimentam o ranking.</Text>
      )}
      {data && data.length > 0 && (
        <Table striped highlightOnHover withTableBorder>
          <Table.Thead>
            <Table.Tr>
              <Table.Th w={80}>#</Table.Th>
              <Table.Th>Jogador</Table.Th>
              <Table.Th>Score</Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {data.map((e) => (
              <Table.Tr key={e.playerId}>
                <Table.Td>{e.rank}</Table.Td>
                <Table.Td>{e.playerId}</Table.Td>
                <Table.Td>{e.score}</Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      )}
    </Stack>
  );
}

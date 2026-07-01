import { Alert, Button, Group, Loader, Stack, Table, Text } from "@mantine/core";
import { IconRefresh } from "@tabler/icons-react";
import { api } from "../lib/api";
import { useAsync } from "../lib/useAsync";

// PlayerAchievements: progresso do jogador no jogo ativo (gameId vem do contexto
// global, via PlayerDetail). Sem gameId -> orienta a escolher no header.
export function PlayerAchievements({ playerId, gameId }: { playerId: string; gameId: string | null }) {
  const { data, loading, error, reload } = useAsync(
    (s) => (gameId ? api.playerAchievements(gameId, playerId, s) : Promise.resolve([])),
    [gameId, playerId],
  );

  if (!gameId) return <Text c="dimmed">Selecione um jogo no topo para ver o progresso do jogador.</Text>;

  return (
    <Stack>
      <Group justify="flex-end">
        <Button variant="default" size="xs" leftSection={<IconRefresh size={14} />} onClick={reload}>
          Atualizar
        </Button>
      </Group>

      {loading && !data && <Loader />}
      {!!error && <Alert color="red">Falha ao carregar (achievements indisponível?).</Alert>}
      {data && data.length === 0 && <Text c="dimmed">Nenhum progresso para este jogador neste jogo.</Text>}
      {data && data.length > 0 && (
        <Table striped highlightOnHover withTableBorder>
          <Table.Thead>
            <Table.Tr>
              <Table.Th>Conquista</Table.Th>
              <Table.Th>Progresso</Table.Th>
              <Table.Th>Estado</Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {data.map((a) => (
              <Table.Tr key={a.id}>
                <Table.Td>{a.name}</Table.Td>
                <Table.Td>
                  {a.progress}/{a.target}
                </Table.Td>
                <Table.Td>
                  {a.unlocked ? (
                    <Text span c="green">
                      desbloqueada
                      {a.unlockedAt ? ` · ${new Date(a.unlockedAt).toLocaleString("pt-BR")}` : ""}
                    </Text>
                  ) : (
                    <Text span c="dimmed">
                      bloqueada
                    </Text>
                  )}
                </Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      )}
    </Stack>
  );
}

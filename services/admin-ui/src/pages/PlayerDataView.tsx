import { Alert, Button, Code, Group, Loader, Stack, Table, Text } from "@mantine/core";
import { IconRefresh } from "@tabler/icons-react";
import { api } from "../lib/api";
import { useAsync } from "../lib/useAsync";

// PlayerDataView: dado de gameplay do jogador no jogo ativo (gameId vem do
// contexto global, via PlayerDetail). Sem gameId -> orienta a escolher no header.
export function PlayerDataView({ playerId, gameId }: { playerId: string; gameId: string | null }) {
  const { data, loading, error, reload } = useAsync(
    (s) => (gameId ? api.playerData(gameId, playerId, s) : Promise.resolve([])),
    [gameId, playerId],
  );

  if (!gameId) return <Text c="dimmed">Selecione um jogo no topo para ver o dado do jogador.</Text>;

  return (
    <Stack>
      <Group justify="flex-end">
        <Button variant="default" size="xs" leftSection={<IconRefresh size={14} />} onClick={reload}>
          Atualizar
        </Button>
      </Group>

      {loading && !data && <Loader />}
      {!!error && <Alert color="red">Falha ao carregar (player-data indisponível?).</Alert>}
      {data && data.length === 0 && <Text c="dimmed">Nenhum dado para este jogador neste jogo.</Text>}
      {data && data.length > 0 && (
        <Table striped highlightOnHover withTableBorder>
          <Table.Thead>
            <Table.Tr>
              <Table.Th>Namespace</Table.Th>
              <Table.Th>Key</Table.Th>
              <Table.Th>Value</Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {data.map((e) => {
              const isPrimitive = e.value === null || typeof e.value !== "object";
              const formatted = isPrimitive ? String(e.value) : JSON.stringify(e.value, null, 2);
              return (
                <Table.Tr key={`${e.namespace}/${e.key}`}>
                  <Table.Td>{e.namespace}</Table.Td>
                  <Table.Td>{e.key}</Table.Td>
                  <Table.Td>
                    {isPrimitive ? (
                      <Code>{formatted}</Code>
                    ) : (
                      <Code block style={{ maxHeight: 200, overflow: "auto" }}>
                        {formatted}
                      </Code>
                    )}
                  </Table.Td>
                </Table.Tr>
              );
            })}
          </Table.Tbody>
        </Table>
      )}
    </Stack>
  );
}

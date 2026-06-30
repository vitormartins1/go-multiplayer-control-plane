import { Alert, Button, Code, Group, Loader, Select, Stack, Table, Text } from "@mantine/core";
import { IconRefresh } from "@tabler/icons-react";
import { useState } from "react";
import { api } from "../lib/api";
import { useAsync } from "../lib/useAsync";

// PlayerDataView: dado de gameplay do jogador num jogo (player-data). Seletor de
// jogo (o jogador é global; o dado é por jogo), carregamento on-open + Atualizar.
export function PlayerDataView({ playerId }: { playerId: string }) {
  const { data: games } = useAsync((s) => api.games(s), []);
  const [gameId, setGameId] = useState<string | null>(null);

  const { data, loading, error, reload } = useAsync(
    (s) => (gameId ? api.playerData(gameId, playerId, s) : Promise.resolve([])),
    [gameId],
  );

  return (
    <Stack>
      <Group justify="space-between">
        <Select
          label="Jogo"
          placeholder="Selecione um jogo"
          data={(games ?? []).map((g) => ({ value: g.id, label: g.name }))}
          value={gameId}
          onChange={setGameId}
          w={280}
          searchable
        />
        {gameId && (
          <Button variant="default" size="xs" leftSection={<IconRefresh size={14} />} onClick={reload}>
            Atualizar
          </Button>
        )}
      </Group>

      {!gameId && <Text c="dimmed">Selecione um jogo para ver o dado do jogador.</Text>}
      {gameId && loading && !data && <Loader />}
      {gameId && !!error && <Alert color="red">Falha ao carregar (player-data indisponível?).</Alert>}
      {gameId && data && data.length === 0 && (
        <Text c="dimmed">Nenhum dado para este jogador neste jogo.</Text>
      )}
      {gameId && data && data.length > 0 && (
        <Table striped highlightOnHover withTableBorder>
          <Table.Thead>
            <Table.Tr>
              <Table.Th>Namespace</Table.Th>
              <Table.Th>Key</Table.Th>
              <Table.Th>Value</Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {data.map((e) => (
              <Table.Tr key={`${e.namespace}/${e.key}`}>
                <Table.Td>{e.namespace}</Table.Td>
                <Table.Td>{e.key}</Table.Td>
                <Table.Td>
                  <Code>{JSON.stringify(e.value)}</Code>
                </Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      )}
    </Stack>
  );
}

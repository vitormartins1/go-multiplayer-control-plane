import { Alert, Button, Code, Group, Loader, Stack, Table, Text } from "@mantine/core";
import { IconRefresh } from "@tabler/icons-react";
import { api } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { JsonTree } from "../components/JsonTree";
import { JsonMaterialized } from "../components/JsonMaterialized";

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
      {!!error && <Alert color="critical">Falha ao carregar (player-data indisponível?).</Alert>}
      {data && data.length === 0 && <Text c="dimmed">Nenhum dado para este jogador neste jogo.</Text>}
      {data && data.length > 0 && (
        // sem highlightOnHover: o hover mudando a cor atrapalha a leitura das
        // células com JSON (spec 042). Estrutura (tree) e Leitura consolidada
        // lado a lado, como no cloud save.
        <Table striped withTableBorder verticalSpacing="sm" style={{ tableLayout: "fixed" }}>
          <Table.Thead>
            <Table.Tr>
              <Table.Th w={130}>Namespace</Table.Th>
              <Table.Th w={130}>Key</Table.Th>
              <Table.Th>Estrutura (JSON)</Table.Th>
              <Table.Th>Leitura consolidada</Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {data.map((e) => {
              const isPrimitive = e.value === null || typeof e.value !== "object";
              return (
                <Table.Tr key={`${e.namespace}/${e.key}`}>
                  <Table.Td ff="monospace">{e.namespace}</Table.Td>
                  <Table.Td ff="monospace">{e.key}</Table.Td>
                  <Table.Td>
                    {isPrimitive ? (
                      <Code>{String(e.value)}</Code>
                    ) : (
                      <JsonTree value={e.value} />
                    )}
                  </Table.Td>
                  <Table.Td style={{ verticalAlign: "top" }}>
                    <JsonMaterialized value={e.value} />
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

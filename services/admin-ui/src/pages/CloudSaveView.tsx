import { Alert, Button, Card, Code, Group, Loader, SimpleGrid, Stack, Table, Text } from "@mantine/core";
import { IconRefresh } from "@tabler/icons-react";
import { api, HttpError, type CloudSave } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { JsonTree } from "../components/JsonTree";
import { JsonMaterialized } from "../components/JsonMaterialized";

// decodeBlob classifica o blob base64 (spec 042): JSON parseado -> tree +
// leitura consolidada; texto -> cru; binário -> só metadados.
type DecodedBlob =
  | { kind: "json"; value: unknown }
  | { kind: "text"; text: string }
  | { kind: "binary" };

function decodeBlob(contentType: string, blobB64: string): DecodedBlob {
  let text: string;
  try {
    text = atob(blobB64);
  } catch {
    return { kind: "binary" };
  }
  if (contentType.includes("json")) {
    try {
      return { kind: "json", value: JSON.parse(text) };
    } catch {
      return { kind: "text", text };
    }
  }
  if (contentType.startsWith("text/")) return { kind: "text", text };
  return { kind: "binary" };
}

// CloudSaveView: save do jogador no jogo ativo (gameId vem do contexto global,
// via PlayerDetail). Sem gameId -> orienta a escolher; 404 (sem save) = vazio.
export function CloudSaveView({ playerId, gameId }: { playerId: string; gameId: string | null }) {
  const { data, loading, error, reload } = useAsync(
    (s) => (gameId ? api.cloudSave(gameId, playerId, s) : Promise.resolve<CloudSave | null>(null)),
    [gameId, playerId],
  );

  if (!gameId) return <Text c="dimmed">Selecione um jogo no topo para ver o save do jogador.</Text>;

  const notFound = error instanceof HttpError && error.status === 404;
  const otherError = !!error && !notFound;
  const decoded = data ? decodeBlob(data.contentType, data.blob) : null;

  return (
    <Stack>
      <Group justify="flex-end">
        <Button variant="default" size="xs" leftSection={<IconRefresh size={14} />} onClick={reload}>
          Atualizar
        </Button>
      </Group>

      {loading && !data && <Loader />}
      {otherError && <Alert color="critical">Falha ao carregar (cloud-save indisponível?).</Alert>}
      {notFound && <Text c="dimmed">Nenhum save para este jogador neste jogo.</Text>}
      {data && decoded && (
        <Stack gap="xs">
          <Table withTableBorder>
            <Table.Tbody>
              <Table.Tr>
                <Table.Th w={160}>Content-Type</Table.Th>
                <Table.Td>{data.contentType}</Table.Td>
              </Table.Tr>
              <Table.Tr>
                <Table.Th>Tamanho</Table.Th>
                <Table.Td ff="monospace">{data.size} bytes</Table.Td>
              </Table.Tr>
              <Table.Tr>
                <Table.Th>Atualizado em</Table.Th>
                <Table.Td>{new Date(data.updatedAt).toLocaleString("pt-BR")}</Table.Td>
              </Table.Tr>
            </Table.Tbody>
          </Table>

          {decoded.kind === "json" && (
            // duas leituras do mesmo blob (spec 042): a estrutura navegável e a
            // versão materializada para bater o olho.
            <SimpleGrid cols={{ base: 1, lg: 2 }} spacing="md">
              <Card withBorder padding="md">
                <Text size="sm" fw={600} mb="xs">
                  Estrutura (JSON)
                </Text>
                <JsonTree value={decoded.value} collapsed={false} />
              </Card>
              <Card withBorder padding="md">
                <Text size="sm" fw={600} mb="xs">
                  Leitura consolidada
                </Text>
                <JsonMaterialized value={decoded.value} />
              </Card>
            </SimpleGrid>
          )}
          {decoded.kind === "text" && <Code block>{decoded.text}</Code>}
          {decoded.kind === "binary" && (
            <Text c="dimmed" size="sm">
              Conteúdo binário ({data.size} bytes) — preview indisponível.
            </Text>
          )}
        </Stack>
      )}
    </Stack>
  );
}

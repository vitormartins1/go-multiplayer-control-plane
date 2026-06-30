import { Alert, Button, Code, Group, Loader, Stack, Table, Text } from "@mantine/core";
import { IconRefresh } from "@tabler/icons-react";
import { api, HttpError, type CloudSave } from "../lib/api";
import { useAsync } from "../lib/useAsync";

// decodePreview tenta decodificar o blob base64 para texto legível. JSON é
// reindentado; texto vai cru; binário vira null (mostramos só os metadados).
function decodePreview(contentType: string, blobB64: string): string | null {
  let text: string;
  try {
    text = atob(blobB64);
  } catch {
    return null;
  }
  if (contentType.includes("json")) {
    try {
      return JSON.stringify(JSON.parse(text), null, 2);
    } catch {
      return text;
    }
  }
  if (contentType.startsWith("text/")) return text;
  return null; // binário/desconhecido
}

// CloudSaveView: save do jogador no jogo ativo (gameId vem do contexto global,
// via PlayerDetail). Sem gameId -> orienta a escolher; 404 (sem save) = vazio.
export function CloudSaveView({ playerId, gameId }: { playerId: string; gameId: string | null }) {
  const { data, loading, error, reload } = useAsync(
    (s) => (gameId ? api.cloudSave(gameId, playerId, s) : Promise.resolve<CloudSave | null>(null)),
    [gameId],
  );

  if (!gameId) return <Text c="dimmed">Selecione um jogo no topo para ver o save do jogador.</Text>;

  const notFound = error instanceof HttpError && error.status === 404;
  const otherError = !!error && !notFound;
  const preview = data ? decodePreview(data.contentType, data.blob) : null;

  return (
    <Stack>
      <Group justify="flex-end">
        <Button variant="default" size="xs" leftSection={<IconRefresh size={14} />} onClick={reload}>
          Atualizar
        </Button>
      </Group>

      {loading && !data && <Loader />}
      {otherError && <Alert color="red">Falha ao carregar (cloud-save indisponível?).</Alert>}
      {notFound && <Text c="dimmed">Nenhum save para este jogador neste jogo.</Text>}
      {data && (
        <Stack gap="xs">
          <Table withTableBorder>
            <Table.Tbody>
              <Table.Tr>
                <Table.Th w={160}>Content-Type</Table.Th>
                <Table.Td>{data.contentType}</Table.Td>
              </Table.Tr>
              <Table.Tr>
                <Table.Th>Tamanho</Table.Th>
                <Table.Td>{data.size} bytes</Table.Td>
              </Table.Tr>
              <Table.Tr>
                <Table.Th>Atualizado em</Table.Th>
                <Table.Td>{new Date(data.updatedAt).toLocaleString("pt-BR")}</Table.Td>
              </Table.Tr>
            </Table.Tbody>
          </Table>
          {preview !== null ? (
            <Code block>{preview}</Code>
          ) : (
            <Text c="dimmed" size="sm">
              Conteúdo binário ({data.size} bytes) — preview indisponível.
            </Text>
          )}
        </Stack>
      )}
    </Stack>
  );
}

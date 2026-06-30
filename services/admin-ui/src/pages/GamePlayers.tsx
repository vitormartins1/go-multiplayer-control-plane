import { Alert, Button, Group, Loader, Stack, Table, Text } from "@mantine/core";
import { IconRefresh } from "@tabler/icons-react";
import { useNavigate } from "react-router-dom";
import { api } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { PlayerAvatar } from "../components/PlayerAvatar";

// GamePlayers é a player base de UM jogo (presença, ADR-0019): quem entrou no
// jogo ao menos uma vez. Clicar abre o detalhe do jogador. (Aba de GameDetail.)
export function GamePlayers({ gameId }: { gameId: string }) {
  const nav = useNavigate();
  const { data, loading, error, reload } = useAsync((s) => api.gamePlayers(gameId, s), [gameId]);

  return (
    <Stack>
      <Group justify="space-between">
        <Text fw={600}>
          Jogadores no jogo {data ? <Text span c="dimmed">· {data.length}</Text> : null}
        </Text>
        <Button variant="default" size="xs" leftSection={<IconRefresh size={14} />} onClick={reload}>
          Atualizar
        </Button>
      </Group>

      {loading && !data && <Loader />}
      {!!error && <Alert color="red">game-registry indisponível.</Alert>}
      {data && data.length === 0 && (
        <Text c="dimmed">Nenhum jogador entrou neste jogo ainda (a presença é registrada no login).</Text>
      )}
      {data && data.length > 0 && (
        <Table striped highlightOnHover withTableBorder>
          <Table.Thead>
            <Table.Tr>
              <Table.Th>Handle</Table.Th>
              <Table.Th>Id</Table.Th>
              <Table.Th>Entrou em</Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {data.map((p) => (
              <Table.Tr key={p.playerId} style={{ cursor: "pointer" }} onClick={() => nav(`/players/${p.playerId}`)}>
                <Table.Td>
                  <Group gap="sm" wrap="nowrap">
                    <PlayerAvatar seed={p.playerId} />
                    <span>{p.handle}</span>
                  </Group>
                </Table.Td>
                <Table.Td c="dimmed">{p.playerId.slice(0, 8)}…</Table.Td>
                <Table.Td>{new Date(p.firstSeen).toLocaleString("pt-BR")}</Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      )}
    </Stack>
  );
}

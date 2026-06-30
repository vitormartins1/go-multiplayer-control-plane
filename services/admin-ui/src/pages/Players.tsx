import { Alert, Button, Group, Loader, Stack, Table, Text, Title } from "@mantine/core";
import { useNavigate } from "react-router-dom";
import { IconRefresh } from "@tabler/icons-react";
import { api } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { useGames } from "../lib/games";

// Players é jogo-cêntrico (ADR-0019): lista a player base do jogo ativo
// (presença). Sem jogo ativo, orienta a escolher no header. Clicar num jogador
// abre o detalhe (que usa o mesmo jogo ativo).
export function Players() {
  const nav = useNavigate();
  const { activeGame } = useGames();
  const gameId = activeGame?.id ?? null;

  const { data, loading, error, reload } = useAsync(
    (s) => (gameId ? api.gamePlayers(gameId, s) : Promise.resolve([])),
    [gameId],
  );

  return (
    <Stack>
      <Group justify="space-between">
        <Title order={3}>
          Players
          {activeGame ? (
            <Text span c="dimmed">
              {" "}
              · {activeGame.name}
              {data ? ` · ${data.length}` : ""}
            </Text>
          ) : null}
        </Title>
        <Button variant="default" leftSection={<IconRefresh size={16} />} onClick={reload} disabled={!gameId}>
          Atualizar
        </Button>
      </Group>

      {!gameId && <Text c="dimmed">Selecione um jogo no topo para ver a base de jogadores dele.</Text>}
      {gameId && loading && !data && <Loader />}
      {gameId && !!error && <Alert color="red">game-registry indisponível.</Alert>}
      {gameId && data && data.length === 0 && (
        <Text c="dimmed">Nenhum jogador entrou neste jogo ainda (a presença é registrada no login).</Text>
      )}
      {gameId && data && data.length > 0 && (
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
              <Table.Tr
                key={p.playerId}
                style={{ cursor: "pointer" }}
                onClick={() => nav(`/players/${p.playerId}`)}
              >
                <Table.Td>{p.handle}</Table.Td>
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

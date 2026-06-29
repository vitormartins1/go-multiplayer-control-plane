import { Alert, Group, Loader, Select, Stack, Table, Text } from "@mantine/core";
import { useState } from "react";
import { api } from "../lib/api";
import { useAsync } from "../lib/useAsync";

// PlayerAchievements: progresso do jogador num jogo (seletor de jogo).
export function PlayerAchievements({ playerId }: { playerId: string }) {
  const { data: games } = useAsync((s) => api.games(s), []);
  const [gameId, setGameId] = useState<string | null>(null);

  const ach = useAsync(
    (s) => (gameId ? api.playerAchievements(gameId, playerId, s) : Promise.resolve([])),
    [gameId],
  );

  return (
    <Stack>
      <Group>
        <Select
          label="Jogo"
          placeholder="Selecione um jogo"
          data={(games ?? []).map((g) => ({ value: g.id, label: g.name }))}
          value={gameId}
          onChange={setGameId}
          w={280}
          searchable
        />
      </Group>

      {!gameId && <Text c="dimmed">Selecione um jogo para ver o progresso do jogador.</Text>}
      {gameId && ach.loading && <Loader />}
      {gameId && !!ach.error && <Alert color="red">Falha ao carregar (achievements indisponível?).</Alert>}
      {gameId && ach.data && ach.data.length === 0 && (
        <Text c="dimmed">Nenhum progresso para este jogador neste jogo.</Text>
      )}
      {gameId && ach.data && ach.data.length > 0 && (
        <Table striped highlightOnHover withTableBorder>
          <Table.Thead>
            <Table.Tr>
              <Table.Th>Conquista</Table.Th>
              <Table.Th>Progresso</Table.Th>
              <Table.Th>Estado</Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {ach.data.map((a) => (
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

import { ActionIcon, Alert, Button, Group, Loader, Modal, NumberInput, Select, Stack, Table, Text } from "@mantine/core";
import { IconPencil, IconRefresh } from "@tabler/icons-react";
import { useState } from "react";
import { notifications } from "@mantine/notifications";
import { api, type LeaderEntry } from "../lib/api";
import { useAsync } from "../lib/useAsync";

const SCORE_BOARD = "score";

export function GameLeaderboards({ gameId }: { gameId: string }) {
  const [board, setBoard] = useState(SCORE_BOARD);
  // Boards disponíveis = score + scoringStats do perfil de multiplayer (ADR-0023).
  // Jogo sem perfil -> só o board score.
  const { data: profile } = useAsync((s) => api.getMultiplayerProfile(gameId, s), [gameId]);
  const boards = [SCORE_BOARD, ...(profile?.params.scoringStats ?? [])];

  const { data, loading, error, reload } = useAsync(
    (s) => api.gameLeaderboard(gameId, board === SCORE_BOARD ? undefined : board, 20, s),
    [gameId, board],
  );
  const [editing, setEditing] = useState<LeaderEntry | null>(null);
  const [score, setScore] = useState<string | number>("");
  const [saving, setSaving] = useState(false);

  function openEdit(entry: LeaderEntry) {
    setEditing(entry);
    setScore(entry.score);
  }

  function closeEdit() {
    setEditing(null);
    setScore("");
  }

  async function saveScore() {
    if (!editing || score === "" || score === undefined) return;
    setSaving(true);
    try {
      await api.setLeaderboardScore(
        gameId,
        editing.playerId,
        Number(score),
        board === SCORE_BOARD ? undefined : board,
      );
      notifications.show({ color: "green", message: `Score de ${editing.playerId.slice(0, 8)}… atualizado.` });
      closeEdit();
      reload();
    } catch {
      notifications.show({ color: "red", message: "Falha ao atualizar o score." });
    } finally {
      setSaving(false);
    }
  }

  return (
    <Stack>
      <Group justify="space-between">
        <Text fw={600}>Ranking do jogo (top 20)</Text>
        <Group gap="xs">
          {boards.length > 1 && (
            <Select
              size="xs"
              w={160}
              aria-label="Board"
              value={board}
              onChange={(v) => v && setBoard(v)}
              data={boards.map((b) => ({
                value: b,
                label: b === SCORE_BOARD ? "Score" : b,
              }))}
              allowDeselect={false}
            />
          )}
          <Button variant="default" size="xs" leftSection={<IconRefresh size={14} />} onClick={reload}>
            Atualizar
          </Button>
        </Group>
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
              <Table.Th w={60}>#</Table.Th>
              <Table.Th>Jogador</Table.Th>
              <Table.Th>{board === SCORE_BOARD ? "Score" : board}</Table.Th>
              <Table.Th w={50}></Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {data.map((e) => (
              <Table.Tr key={e.playerId}>
                <Table.Td>{e.rank}</Table.Td>
                <Table.Td c="dimmed">{e.playerId}</Table.Td>
                <Table.Td>{e.score}</Table.Td>
                <Table.Td>
                  <ActionIcon variant="subtle" color="gray" onClick={() => openEdit(e)} aria-label="Ajustar score">
                    <IconPencil size={16} />
                  </ActionIcon>
                </Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      )}

      <Modal opened={!!editing} onClose={closeEdit} title="Ajustar score" size="xs">
        {editing && (
          <Stack>
            <Text size="sm" c="dimmed">
              Jogador: <code>{editing.playerId}</code>
            </Text>
            <NumberInput
              label="Novo score"
              value={score}
              onChange={setScore}
              min={0}
              autoFocus
            />
            <Text size="xs" c="dimmed">
              Este override substitui o score atual independente do valor (sem highscore).
            </Text>
            <Group justify="flex-end">
              <Button variant="default" onClick={closeEdit} disabled={saving}>
                Cancelar
              </Button>
              <Button onClick={saveScore} loading={saving} disabled={score === "" || score === undefined}>
                Salvar
              </Button>
            </Group>
          </Stack>
        )}
      </Modal>
    </Stack>
  );
}

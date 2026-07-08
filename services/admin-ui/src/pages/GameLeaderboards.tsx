import { ActionIcon, Alert, Button, Group, Loader, Modal, NumberInput, SegmentedControl, Select, Stack, Table, Text } from "@mantine/core";
import { IconPencil, IconRefresh } from "@tabler/icons-react";
import { useState } from "react";
import { notifications } from "@mantine/notifications";
import { api, type LeaderEntry } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { PlayerAvatar } from "../components/PlayerAvatar";
import { GuestBadge } from "../components/GuestBadge";

const SCORE_BOARD = "score";
const PAGE = 20;

// Janelas de tempo (leaderboard/007): derivadas do relógio no serviço.
const WINDOW_OPTIONS = [
  { value: "alltime", label: "Geral" },
  { value: "daily", label: "Hoje" },
  { value: "weekly", label: "Semana" },
  { value: "monthly", label: "Mês" },
];

export function GameLeaderboards({ gameId }: { gameId: string }) {
  const [board, setBoard] = useState(SCORE_BOARD);
  const [window, setWindow] = useState("alltime");
  // Paginação (spec 039): a 1ª página vem do useAsync; "Carregar mais" acumula
  // em extra (ranks contínuos vêm do serviço). Trocar board/janela reseta.
  const [extra, setExtra] = useState<LeaderEntry[]>([]);
  const [noMore, setNoMore] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);

  function resetPaging() {
    setExtra([]);
    setNoMore(false);
  }
  // Boards derivados do perfil de multiplayer (ADR-0023). Dois modos:
  //   highscore (melhor de uma partida): score + cada stat;
  //   cumulative (soma na temporada): vitórias + <stat>_total.
  const { data: profile } = useAsync((s) => api.getMultiplayerProfile(gameId, s), [gameId]);
  const stats = profile?.archetype === "queue_arena" ? (profile.params.scoringStats ?? []) : [];
  const boardOptions = [
    { value: SCORE_BOARD, label: "Score" },
    ...stats.map((s: string) => ({ value: s, label: `${s} (recorde)` })),
    { value: "wins", label: "Vitórias (total)" },
    ...stats.map((s: string) => ({ value: `${s}_total`, label: `${s} (total)` })),
  ];
  const boardLabel = boardOptions.find((o) => o.value === board)?.label ?? board;

  const boardParam = board === SCORE_BOARD ? undefined : board;
  const { data, loading, error, reload } = useAsync(
    (s) => api.gameLeaderboard(gameId, { board: boardParam, window, limit: PAGE }, s),
    [gameId, board, window],
  );

  const rows = data ? [...data, ...extra] : null;
  const hasMore = !noMore && !!data && data.length === PAGE;

  async function loadMore() {
    if (!rows) return;
    setLoadingMore(true);
    try {
      const batch = await api.gameLeaderboard(gameId, {
        board: boardParam,
        window,
        limit: PAGE,
        offset: rows.length,
      });
      if (batch.length < PAGE) setNoMore(true);
      setExtra((prev) => [...prev, ...batch]);
    } catch {
      notifications.show({ color: "red", message: "Falha ao carregar a próxima página." });
    } finally {
      setLoadingMore(false);
    }
  }
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
      resetPaging();
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
        <Text fw={600}>Ranking do jogo</Text>
        <Group gap="xs">
          <SegmentedControl
            size="xs"
            value={window}
            onChange={(v) => {
              resetPaging();
              setWindow(v);
            }}
            data={WINDOW_OPTIONS}
          />
          {boardOptions.length > 1 && (
            <Select
              size="xs"
              w={180}
              aria-label="Board"
              value={board}
              onChange={(v) => {
                if (!v) return;
                resetPaging();
                setBoard(v);
              }}
              data={boardOptions}
              allowDeselect={false}
            />
          )}
          <Button
            variant="default"
            size="xs"
            leftSection={<IconRefresh size={14} />}
            onClick={() => {
              resetPaging();
              reload();
            }}
          >
            Atualizar
          </Button>
        </Group>
      </Group>

      {loading && !rows && <Loader />}
      {!!error && <Alert color="red">leaderboard indisponível.</Alert>}
      {rows && rows.length === 0 && (
        <Text c="dimmed">
          {window === "alltime"
            ? "Nenhum score ainda — as partidas reportadas (ReportResult) alimentam o ranking."
            : "Sem partidas no período."}
        </Text>
      )}
      {rows && rows.length > 0 && (
        <Table striped highlightOnHover withTableBorder>
          <Table.Thead>
            <Table.Tr>
              <Table.Th w={60}>#</Table.Th>
              <Table.Th>Jogador</Table.Th>
              <Table.Th>{boardLabel}</Table.Th>
              <Table.Th w={50}></Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {rows.map((e) => (
              <Table.Tr key={e.playerId}>
                <Table.Td>{e.rank}</Table.Td>
                <Table.Td>
                  <Group gap={6} wrap="nowrap">
                    <PlayerAvatar seed={e.playerId} size={24} />
                    <Text size="sm" c="dimmed">
                      {e.playerId}
                    </Text>
                    <GuestBadge playerId={e.playerId} />
                  </Group>
                </Table.Td>
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
      {hasMore && (
        <Group justify="center">
          <Button variant="default" size="xs" onClick={loadMore} loading={loadingMore}>
            Carregar mais
          </Button>
        </Group>
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

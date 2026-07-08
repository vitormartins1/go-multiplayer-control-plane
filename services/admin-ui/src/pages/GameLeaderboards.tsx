import { ActionIcon, Alert, Button, Divider, Group, Loader, Modal, NumberInput, SegmentedControl, Select, SimpleGrid, Stack, Table, Text } from "@mantine/core";
import { IconPencil, IconRefresh } from "@tabler/icons-react";
import { useState } from "react";
import { notifications } from "@mantine/notifications";
import { api, type LeaderEntry } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { PlayerAvatar } from "../components/PlayerAvatar";
import { GuestBadge } from "../components/GuestBadge";

const SCORE_BOARD = "score";
const PAGE = 20;
const OVERVIEW_TOP = 5;

// Janelas de tempo (leaderboard/007): derivadas do relógio no serviço.
const WINDOW_OPTIONS = [
  { value: "alltime", label: "Geral" },
  { value: "daily", label: "Hoje" },
  { value: "weekly", label: "Semana" },
  { value: "monthly", label: "Mês" },
];

// WindowMiniTable: uma janela de um board na Visão geral (spec 041) — top 5,
// sem paginação e sem edição (ajuste de score é tarefa do modo Específica).
// tick força reload quando o Atualizar global é clicado.
function WindowMiniTable({
  gameId,
  board,
  label,
  win,
  tick,
}: {
  gameId: string;
  board: string;
  label: string;
  win: string;
  tick: number;
}) {
  const { data, loading, error } = useAsync(
    (s) =>
      api.gameLeaderboard(
        gameId,
        { board: board === SCORE_BOARD ? undefined : board, window: win, limit: OVERVIEW_TOP },
        s,
      ),
    [gameId, board, win, tick],
  );

  return (
    <Stack gap={6}>
      <Text size="sm" fw={600}>
        {WINDOW_OPTIONS.find((w) => w.value === win)?.label ?? win}
      </Text>
      {loading && !data && <Loader size="sm" />}
      {!!error && (
        <Text size="sm" c="red">
          leaderboard indisponível.
        </Text>
      )}
      {data && (
        <Table striped withTableBorder style={{ tableLayout: "fixed" }}>
          <Table.Thead>
            <Table.Tr>
              <Table.Th w={44}>#</Table.Th>
              <Table.Th>Jogador</Table.Th>
              <Table.Th w="28%">{label}</Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {data.map((e) => (
              <Table.Tr key={e.playerId}>
                <Table.Td>{e.rank}</Table.Td>
                <Table.Td>
                  <Group gap={6} wrap="nowrap">
                    <PlayerAvatar seed={e.playerId} size={20} />
                    <Text size="sm" c="dimmed" truncate>
                      {e.playerId}
                    </Text>
                    <GuestBadge playerId={e.playerId} />
                  </Group>
                </Table.Td>
                <Table.Td>{e.score}</Table.Td>
              </Table.Tr>
            ))}
            {/* completa até OVERVIEW_TOP com "–": todas as mini-tabelas ficam
                com a mesma altura e o grid permanece visualmente coeso. */}
            {Array.from({ length: Math.max(0, OVERVIEW_TOP - data.length) }, (_, i) => (
              <Table.Tr key={`empty-${i}`}>
                <Table.Td c="dimmed">–</Table.Td>
                <Table.Td c="dimmed">–</Table.Td>
                <Table.Td c="dimmed">–</Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      )}
    </Stack>
  );
}

export function GameLeaderboards({ gameId }: { gameId: string }) {
  const [board, setBoard] = useState(SCORE_BOARD);
  const [window, setWindow] = useState("alltime");
  // Modo de exibição (spec 041): "single" = uma tabela com janela/board
  // selecionáveis (e edição); "overview" = todas as janelas de todos os boards.
  const [mode, setMode] = useState<"single" | "overview">("single");
  const [overviewTick, setOverviewTick] = useState(0);
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
            value={mode}
            onChange={(v) => setMode(v as "single" | "overview")}
            data={[
              { value: "single", label: "Específica" },
              { value: "overview", label: "Visão geral" },
            ]}
          />
          {mode === "single" && (
            <SegmentedControl
              size="xs"
              value={window}
              onChange={(v) => {
                resetPaging();
                setWindow(v);
              }}
              data={WINDOW_OPTIONS}
            />
          )}
          {mode === "single" && boardOptions.length > 1 && (
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
              if (mode === "overview") {
                setOverviewTick((t) => t + 1);
                return;
              }
              resetPaging();
              reload();
            }}
          >
            Atualizar
          </Button>
        </Group>
      </Group>

      {mode === "overview" &&
        boardOptions.map((b) => (
          <Stack key={b.value} gap="sm">
            <Divider label={b.label} labelPosition="left" />
            <SimpleGrid cols={{ base: 1, md: 2 }} spacing="md">
              {WINDOW_OPTIONS.map((w) => (
                <WindowMiniTable
                  key={w.value}
                  gameId={gameId}
                  board={b.value}
                  label={b.label}
                  win={w.value}
                  tick={overviewTick}
                />
              ))}
            </SimpleGrid>
          </Stack>
        ))}

      {mode === "single" && loading && !rows && <Loader />}
      {mode === "single" && !!error && <Alert color="red">leaderboard indisponível.</Alert>}
      {mode === "single" && rows && rows.length === 0 && (
        <Text c="dimmed">
          {window === "alltime"
            ? "Nenhum score ainda — as partidas reportadas (ReportResult) alimentam o ranking."
            : "Sem partidas no período."}
        </Text>
      )}
      {mode === "single" && rows && rows.length > 0 && (
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
      {mode === "single" && hasMore && (
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

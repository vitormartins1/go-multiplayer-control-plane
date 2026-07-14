import { ActionIcon, Alert, Button, Group, Loader, Modal, NumberInput, SegmentedControl, SimpleGrid, Stack, Table, Text } from "@mantine/core";
import { IconChartBar, IconPencil, IconRefresh } from "@tabler/icons-react";
import { useState } from "react";
import { notifications } from "@mantine/notifications";
import { api, type LeaderEntry } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { useSettings } from "../lib/settings";
import { PlayerCell } from "../components/SessionMatchDetail";
import { ChannelStrip } from "../components/ChannelStrip";

const SCORE_BOARD = "score";
const ALL_BOARDS = "all"; // valor do switch de boards; nunca vai para a API
// OVERVIEW_TOP (fixo, spec 084): topo-5 das mini-tabelas da visão "Geral" é um
// preview de tamanho fixo, não uma página — não segue a preferência global de
// itens-por-página (essa só se aplica à tabela grande, paginada/editável).
const OVERVIEW_TOP = 5;

// Janelas de tempo (leaderboard/007): derivadas do relógio no serviço.
const WINDOW_OPTIONS = [
  { value: "alltime", label: "Geral" },
  { value: "daily", label: "Hoje" },
  { value: "weekly", label: "Semana" },
  { value: "monthly", label: "Mês" },
];

// MiniBoardTable: um card compacto (board × janela) — top 5, sem paginação e
// sem edição (ajuste de score é tarefa da tabela específica). title é a
// dimensão que varia no grid (janela na Visão geral; board no modo "Todas");
// header é a dimensão fixa complementar. tick força reload no Atualizar global.
function MiniBoardTable({
  gameId,
  board,
  title,
  header,
  win,
  tick,
}: {
  gameId: string;
  board: string;
  title: string;
  header: string;
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
        {title}
      </Text>
      {loading && !data && <Loader size="sm" />}
      {!!error && (
        <Text size="sm" c="critical">
          leaderboard indisponível.
        </Text>
      )}
      {data && (
        <Table striped withTableBorder style={{ tableLayout: "fixed" }}>
          <Table.Thead>
            <Table.Tr>
              <Table.Th w={44}>#</Table.Th>
              <Table.Th>Jogador</Table.Th>
              <Table.Th w="28%">{header}</Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {data.map((e) => (
              <Table.Tr key={e.playerId}>
                <Table.Td ff="monospace">{e.rank}</Table.Td>
                <Table.Td>
                  <PlayerCell id={e.playerId} handle={e.handle ?? ""} />
                </Table.Td>
                <Table.Td ff="monospace">{e.score}</Table.Td>
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

// BoardChannelStrip (spec 074): cabeçalho de cada grupo de board na "Visão
// geral" — mesmo elemento assinatura da aba Analytics (ChannelStrip,
// extraído como componente compartilhado). Ícone único (IconChartBar, o
// mesmo da própria aba Leaderboards) pra todo board — o board não tem uma
// taxonomia de ícones própria (diferente do arquétipo em Analytics); a
// diferenciação vem do rótulo, o ícone só sustenta a gramática visual do
// canal. Leitura = jogadores no ranking "Geral" (all-time) daquele board —
// única contagem que faz sentido pro grupo inteiro (as 4 janelas ao lado
// têm cada uma sua própria contagem, sem "o" total óbvio).
function BoardChannelStrip({ gameId, board, label }: { gameId: string; board: string; label: string }) {
  const { data } = useAsync(
    (s) =>
      api.gameLeaderboard(gameId, { board: board === SCORE_BOARD ? undefined : board, window: "alltime", limit: 50 }, s),
    [gameId, board],
  );
  const readout = data ? `${data.length}${data.length === 50 ? "+" : ""} jogador${data.length === 1 ? "" : "es"}` : undefined;
  return <ChannelStrip icon={IconChartBar} label={label} readout={readout} />;
}

export function GameLeaderboards({ gameId }: { gameId: string }) {
  const { pageSize: PAGE } = useSettings();
  const [board, setBoard] = useState(SCORE_BOARD);
  const [window, setWindow] = useState("alltime");
  // Dois eixos independentes (spec 041): mode controla JANELAS ("single" = a
  // janela escolhida; "overview" = as 4) e board controla RANKINGS (um board
  // ou ALL_BOARDS = todos). Só single+board específico tem paginação/edição.
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
  const windowLabel = WINDOW_OPTIONS.find((w) => w.value === window)?.label ?? window;
  // Boards a exibir nas visões de mini-tabelas ("Todas" ou um só).
  const shownBoards = board === ALL_BOARDS ? boardOptions : boardOptions.filter((o) => o.value === board);
  // A tabela grande (paginada/editável) só existe em single + board específico.
  const singleTable = mode === "single" && board !== ALL_BOARDS;

  const boardParam = board === SCORE_BOARD ? undefined : board;
  const { data, loading, error, reload } = useAsync(
    (s) =>
      board === ALL_BOARDS
        ? Promise.resolve<LeaderEntry[]>([])
        : api.gameLeaderboard(gameId, { board: boardParam, window, limit: PAGE }, s),
    [gameId, board, window, PAGE],
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
      notifications.show({ color: "critical", message: "Falha ao carregar a próxima página." });
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
      notifications.show({ color: "nominal", message: `Score de ${editing.playerId.slice(0, 8)}… atualizado.` });
      closeEdit();
      resetPaging();
      reload();
    } catch {
      notifications.show({ color: "critical", message: "Falha ao atualizar o score." });
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
          {boardOptions.length > 1 && (
            <SegmentedControl
              size="xs"
              value={board}
              onChange={(v) => {
                resetPaging();
                setBoard(v);
              }}
              data={[{ value: ALL_BOARDS, label: "Todas" }, ...boardOptions]}
            />
          )}
          <Button
            variant="default"
            size="xs"
            leftSection={<IconRefresh size={14} />}
            onClick={() => {
              if (!singleTable) {
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
        shownBoards.map((b) => (
          <Stack key={b.value} gap="sm">
            <BoardChannelStrip gameId={gameId} board={b.value} label={b.label} />
            <SimpleGrid cols={{ base: 1, md: 2 }} spacing="md">
              {WINDOW_OPTIONS.map((w) => (
                <MiniBoardTable
                  key={w.value}
                  gameId={gameId}
                  board={b.value}
                  title={w.label}
                  header={b.label}
                  win={w.value}
                  tick={overviewTick}
                />
              ))}
            </SimpleGrid>
          </Stack>
        ))}

      {mode === "single" && board === ALL_BOARDS && (
        <SimpleGrid cols={{ base: 1, md: 2 }} spacing="md">
          {shownBoards.map((b) => (
            <MiniBoardTable
              key={b.value}
              gameId={gameId}
              board={b.value}
              title={b.label}
              header={windowLabel}
              win={window}
              tick={overviewTick}
            />
          ))}
        </SimpleGrid>
      )}

      {singleTable && loading && !rows && <Loader />}
      {singleTable && !!error && <Alert color="critical">leaderboard indisponível.</Alert>}
      {singleTable && rows && rows.length === 0 && (
        <Text c="dimmed">
          {window === "alltime"
            ? "Nenhum score ainda — as partidas reportadas (ReportResult) alimentam o ranking."
            : "Sem partidas no período."}
        </Text>
      )}
      {singleTable && rows && rows.length > 0 && (
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
                <Table.Td ff="monospace">{e.rank}</Table.Td>
                <Table.Td>
                  {/* size=32 (padronização das tabelas, 2026-07-13): iguala a
                      densidade da tabela de referência (Achievements/Players)
                      — a mini-tabela da Visão geral continua compacta (20px),
                      contexto diferente (grid denso de vários boards).
                      PlayerCell (spec 088): handle resolvido pelo admin-bff
                      em vez do UUID cru — cai pro id truncado só quando o
                      identity não resolveu (guest antigo, best-effort). */}
                  <PlayerCell id={e.playerId} handle={e.handle ?? ""} avatarSize={32} textSize="sm" />
                </Table.Td>
                <Table.Td ff="monospace">{e.score}</Table.Td>
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
      {singleTable && hasMore && (
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

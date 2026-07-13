import { Alert, Button, Group, Loader, Stack, Table, Text } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { IconRefresh } from "@tabler/icons-react";
import { useEffect, useMemo, useState } from "react";
import { api, type GameSession } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { closeReasonLabel, formatDuration } from "../lib/format";
import { PlayersSummary } from "../components/PlayersSummary";
import { ParticipantOutcomeBadge } from "../components/SessionMatchDetail";

const PAGE = 20;

// PlayerHistory é a aba History do jogador (spec 025): sessões fechadas
// PERSISTIDAS do jogo selecionado em que ele participou — sobrevive a restart
// do session-service, ao contrário da aba Sessions (ring in-memory). gameId
// vem do seletor de jogo do PlayerDetail (contexto do jogador, ADR-0019).
export function PlayerHistory({ playerId, gameId }: { playerId: string; gameId: string | null }) {
  const { data, loading, error, reload } = useAsync(
    (s) => (gameId ? api.playerSessions(playerId, gameId, { limit: PAGE }, s) : Promise.resolve(null)),
    [gameId, playerId],
  );
  // Paginação por cursor (mesmo padrão do "Carregar mais" do leaderboard, spec
  // 039): a 1ª página vem do useAsync; cursor acumula o nextBefore mais recente.
  const [extra, setExtra] = useState<GameSession[]>([]);
  const [cursor, setCursor] = useState<string | undefined>(undefined);
  const [noMore, setNoMore] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);

  useEffect(() => {
    setExtra([]);
    setCursor(undefined);
    setNoMore(false);
  }, [gameId, playerId]);

  const rows = useMemo(() => [...(data?.closed ?? []), ...extra], [data, extra]);
  const hasMore = !noMore && !!(cursor ?? data?.nextBefore);

  async function loadMore() {
    if (!gameId) return;
    const before = cursor ?? data?.nextBefore;
    if (!before) return;
    setLoadingMore(true);
    try {
      const page = await api.playerSessions(playerId, gameId, { limit: PAGE, before });
      setExtra((prev) => [...prev, ...page.closed]);
      setCursor(page.nextBefore);
      if (!page.nextBefore) setNoMore(true);
    } catch {
      notifications.show({ color: "critical", message: "Falha ao carregar mais sessões." });
    } finally {
      setLoadingMore(false);
    }
  }

  if (!gameId) return <Text c="dimmed">Selecione um jogo no topo para ver o histórico do jogador.</Text>;

  return (
    <Stack>
      <Group justify="space-between">
        <Text fw={600}>
          Histórico{" "}
          {data && (
            <Text span c="dimmed">
              · {rows.length}
              {hasMore ? "+" : ""} sessão(ões)
            </Text>
          )}
        </Text>
        <Button variant="default" size="xs" leftSection={<IconRefresh size={14} />} onClick={reload}>
          Atualizar
        </Button>
      </Group>

      {loading && !data && <Loader />}
      {!!error && <Alert color="critical">Histórico indisponível (session-service ou Postgres do histórico fora?).</Alert>}
      {data && rows.length === 0 && (
        <Text c="dimmed">Nenhuma sessão fechada deste jogador neste jogo ainda.</Text>
      )}
      {rows.length > 0 && (
        <Table.ScrollContainer minWidth={640}>
          <Table striped withTableBorder>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Quando</Table.Th>
                <Table.Th>Modo</Table.Th>
                <Table.Th>Papel</Table.Th>
                <Table.Th>Motivo</Table.Th>
                <Table.Th>Duração</Table.Th>
                <Table.Th>Participantes</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {rows.map((s) => (
                <PlayerHistoryRow key={s.sessionId} session={s} playerId={playerId} />
              ))}
            </Table.Tbody>
          </Table>
        </Table.ScrollContainer>
      )}
      {hasMore && (
        <Group justify="center">
          <Button variant="default" size="xs" onClick={loadMore} loading={loadingMore}>
            Carregar mais
          </Button>
        </Group>
      )}
    </Stack>
  );
}

function PlayerHistoryRow({ session: s, playerId }: { session: GameSession; playerId: string }) {
  const participant = s.participants?.find((p) => p.playerId === playerId);
  const duration = s.closedAt ? formatDuration(Date.parse(s.closedAt) - Date.parse(s.openedAt)) : "?";
  return (
    <Table.Tr>
      <Table.Td>
        <Text size="sm">{s.closedAt ? new Date(s.closedAt).toLocaleString("pt-BR") : "—"}</Text>
      </Table.Td>
      <Table.Td>
        <Text size="sm">{s.modeId || "default"}</Text>
      </Table.Td>
      <Table.Td>
        <Group gap={6} wrap="nowrap">
          <Text size="sm">{participant?.role || "—"}</Text>
          <ParticipantOutcomeBadge participant={participant} />
        </Group>
      </Table.Td>
      <Table.Td>
        <Text size="sm">{closeReasonLabel(s.closeReason)}</Text>
      </Table.Td>
      <Table.Td>
        <Text size="sm">{duration}</Text>
      </Table.Td>
      <Table.Td>
        <PlayersSummary players={s.players} />
      </Table.Td>
    </Table.Tr>
  );
}

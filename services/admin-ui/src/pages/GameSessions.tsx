import { useEffect, useMemo, useState } from "react";
import {
  Alert,
  Badge,
  Box,
  Button,
  Collapse,
  Group,
  Loader,
  SegmentedControl,
  Select,
  Stack,
  Table,
  Text,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { IconActivity } from "@tabler/icons-react";
import { api, type GameSession } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { usePolling } from "../lib/usePolling";
import { useSettings } from "../lib/settings";
import { closeReasonLabel, formatDuration, serverLabel, shortId, truncate } from "../lib/format";
import { sessionStateLabel } from "../lib/vocab";
import { SessionAvatar } from "../components/SessionAvatar";
import { PlayersSummary } from "../components/PlayersSummary";
import { SessionMatchDetail } from "../components/SessionMatchDetail";
import { EmptyState } from "../components/EmptyState";
import { archetypeMetaOf } from "../lib/archetypeMeta";

// GameSessions é a aba de operação/debug das partidas do jogo (spec 007):
// abertas ao vivo (mapa in-memory, polling da preferência global) + histórico
// de fechadas PERSISTIDO (spec 025), paginado com "carregar mais" (tamanho
// da preferência global, spec 084). Polling só das abertas — o histórico não
// muda sozinho, recarrega só ao trocar de jogo ou via "carregar mais".
// GameDetail usa keepMounted=false, então sair da aba desmonta e para o
// polling.
export function GameSessions({ gameId }: { gameId: string }) {
  const { pageSize, pollingMs } = useSettings();
  const { data, loading, error, reload } = useAsync((s) => api.gameSessions(gameId, s), [gameId]);
  // Histórico persistido (spec 025): 1ª página vem do useAsync; "carregar
  // mais" acumula em closedExtra e avança o cursor (mesmo padrão do
  // leaderboard, spec 039). Postgres do histórico fora (503) -> cai pro ring
  // in-memory de `data.closed` (plan D6), com aviso.
  const { data: closedPage, error: closedError } = useAsync(
    (s) => api.closedGameSessions(gameId, { limit: pageSize }, s),
    [gameId, pageSize],
  );
  const [closedExtra, setClosedExtra] = useState<GameSession[]>([]);
  const [closedCursor, setClosedCursor] = useState<string | undefined>(undefined);
  const [closedNoMore, setClosedNoMore] = useState(false);
  const [loadingMoreClosed, setLoadingMoreClosed] = useState(false);
  const [filter, setFilter] = useState<"all" | "open" | "closed">("all");
  const [modeFilter, setModeFilter] = useState<string | null>(null);
  const [lastUpdate, setLastUpdate] = useState<Date | null>(null);
  const [expandedId, setExpandedId] = useState<string | null>(null);

  useEffect(() => {
    setClosedExtra([]);
    setClosedCursor(undefined);
    setClosedNoMore(false);
  }, [gameId]);

  usePolling(reload, pollingMs);

  useEffect(() => {
    if (data) setLastUpdate(new Date());
  }, [data]);

  // Postgres do histórico fora -> fallback pro ring in-memory do gameSessions
  // (plan D6): mostra o que der (bounded, ~200 fechadas globais), sem "carregar
  // mais" (o ring não pagina).
  const historyUnavailable = !!closedError;
  const closedRows = useMemo(() => {
    if (historyUnavailable) return data?.closed ?? [];
    return [...(closedPage?.closed ?? []), ...closedExtra];
  }, [historyUnavailable, data, closedPage, closedExtra]);
  const hasMoreClosed = !historyUnavailable && !closedNoMore && !!(closedCursor ?? closedPage?.nextBefore);

  async function loadMoreClosed() {
    const before = closedCursor ?? closedPage?.nextBefore;
    if (!before) return;
    setLoadingMoreClosed(true);
    try {
      const page = await api.closedGameSessions(gameId, { limit: pageSize, before });
      setClosedExtra((prev) => [...prev, ...page.closed]);
      setClosedCursor(page.nextBefore);
      if (!page.nextBefore) setClosedNoMore(true);
    } catch {
      notifications.show({ color: "critical", message: "Falha ao carregar mais sessões fechadas." });
    } finally {
      setLoadingMoreClosed(false);
    }
  }

  // modo de cada sessão (ADR-0024, 028): ausente em sessões legadas = default.
  const modeOf = (s: GameSession) => s.modeId || "default";

  const allRows = useMemo(() => {
    const active = filter === "closed" ? [] : (data?.active ?? []);
    const closed = filter === "open" ? [] : closedRows;
    return [...active, ...closed];
  }, [data, closedRows, filter]);

  // filtro por modo só aparece com ≥2 modos distintos na lista (sem ruído).
  const distinctModes = useMemo(() => [...new Set(allRows.map(modeOf))].sort(), [allRows]);
  const rows = useMemo(
    () => (modeFilter ? allRows.filter((s) => modeOf(s) === modeFilter) : allRows),
    [allRows, modeFilter],
  );

  return (
    <Stack>
      <Group justify="space-between">
        <Text fw={600}>
          Partidas{" "}
          {data ? (
            <Text span c="dimmed">
              · {data.active.length} aberta(s), {closedRows.length}
              {hasMoreClosed ? "+" : ""} fechada(s)
            </Text>
          ) : null}
        </Text>
        <Group gap="sm">
          {lastUpdate && (
            <Text size="xs" c="dimmed">
              atualizado {lastUpdate.toLocaleTimeString("pt-BR")}
            </Text>
          )}
          {distinctModes.length > 1 && (
            <Select
              size="xs"
              w={170}
              placeholder="Todos os modos"
              clearable
              value={modeFilter}
              onChange={setModeFilter}
              data={distinctModes}
            />
          )}
          <SegmentedControl
            size="xs"
            value={filter}
            onChange={(v) => setFilter(v as typeof filter)}
            data={[
              { label: "Todas", value: "all" },
              { label: "Abertas", value: "open" },
              { label: "Fechadas", value: "closed" },
            ]}
          />
        </Group>
      </Group>

      {loading && !data && <Loader />}
      {!!error && <Alert color="critical">sessões indisponíveis (session-service fora?).</Alert>}
      {historyUnavailable && (
        <Alert color="yellow">
          Histórico persistido indisponível (Postgres do session-service fora?) — mostrando só as fechadas
          recentes em memória, sem paginação.
        </Alert>
      )}
      {data && rows.length === 0 && (
        <EmptyState
          icon={IconActivity}
          message="Nenhuma partida por aqui — abertas aparecem ao vivo quando um match é formado; fechadas ficam persistidas no histórico do jogo."
        />
      )}
      {rows.length > 0 && (
        <Table.ScrollContainer minWidth={640}>
          <Table striped withTableBorder>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Sessão</Table.Th>
                <Table.Th>Modo</Table.Th>
                <Table.Th>Estado</Table.Th>
                <Table.Th>Players</Table.Th>
                <Table.Th>Server</Table.Th>
                <Table.Th>Tempos</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {rows.map((s) => (
                <SessionRow
                  key={s.sessionId}
                  session={s}
                  expanded={expandedId === s.sessionId}
                  onToggle={() => setExpandedId((prev) => (prev === s.sessionId ? null : s.sessionId))}
                />
              ))}
            </Table.Tbody>
          </Table>
        </Table.ScrollContainer>
      )}
      {hasMoreClosed && (filter === "all" || filter === "closed") && (
        <Group justify="center">
          <Button variant="default" size="xs" onClick={loadMoreClosed} loading={loadingMoreClosed}>
            Carregar mais fechadas
          </Button>
        </Group>
      )}
    </Stack>
  );
}

function SessionRow({
  session: s,
  expanded,
  onToggle,
}: {
  session: GameSession;
  expanded: boolean;
  onToggle: () => void;
}) {
  const modeMeta = archetypeMetaOf(s.archetype);
  return (
    <>
      <Table.Tr
        style={{ cursor: "pointer" }}
        bg={expanded ? "var(--mantine-color-default-hover)" : undefined}
        onClick={onToggle}
      >
        <Table.Td>
          {/* size=32 (padronização das tabelas, 2026-07-13): iguala a
              densidade da tabela de referência (Achievements/Players). */}
          <Group gap={6} wrap="nowrap">
            <SessionAvatar seed={s.profileHash || s.sessionId} size={32} />
            <Text ff="monospace" size="sm">
              {shortId(s.matchId || s.sessionId)}
            </Text>
          </Group>
        </Table.Td>
        <Table.Td>
          <Badge
            variant="light"
            color="gray"
            leftSection={modeMeta && <modeMeta.icon size={12} />}
          >
            {s.modeId || "default"}
          </Badge>
        </Table.Td>
        <Table.Td>
          <Badge color={s.state === "open" ? "nominal" : "gray"} variant="light">
            {sessionStateLabel(s.state, s.closeReason)}
          </Badge>
        </Table.Td>
        <Table.Td>
          {/* resumo compacto (034): a lista completa vive no painel expandido.
              size=32 (padronização das tabelas): mesmo motivo do avatar acima. */}
          <PlayersSummary players={s.players} size={32} />
        </Table.Td>
        <Table.Td>
          {/* fallback "—" (achado: coluna ficava em branco em sessão
              fechada — servidor já desalocado — e parecia bug de
              carregamento; o painel expandido já tinha o fallback). */}
          <Text ff="monospace" size="sm" c={serverLabel(s) === "—" ? "dimmed" : undefined}>
            {s.serverAddress ? truncate(s.serverAddress) : serverLabel(s)}
          </Text>
        </Table.Td>
        <Table.Td>
          {s.state === "open" ? (
            <Text size="sm">há {formatDuration(Date.now() - Date.parse(s.openedAt))}</Text>
          ) : (
            <Text size="sm">
              durou {s.closedAt ? formatDuration(Date.parse(s.closedAt) - Date.parse(s.openedAt)) : "?"} · fechou{" "}
              {s.closedAt ? new Date(s.closedAt).toLocaleTimeString("pt-BR") : "?"}
            </Text>
          )}
        </Table.Td>
      </Table.Tr>
      {expanded && (
        <Table.Tr>
          <Table.Td colSpan={6} p={0}>
            <Collapse in={expanded}>
              <Box p="md" style={{ borderTop: "1px solid var(--mantine-color-default-border)" }}>
                <Stack gap="xs">
                  <Group gap="xl" wrap="wrap">
                    <SessionAvatar seed={s.profileHash || s.sessionId} size={40} />
                    <Stack gap={2}>
                      <Text size="xs" c="dimmed" fw={600}>SESSION ID</Text>
                      <Text ff="monospace" size="xs">{s.sessionId}</Text>
                    </Stack>
                    <Stack gap={2}>
                      <Text size="xs" c="dimmed" fw={600}>MATCH ID</Text>
                      <Text ff="monospace" size="xs">{s.matchId}</Text>
                    </Stack>
                    <Stack gap={2}>
                      <Text size="xs" c="dimmed" fw={600}>MODO</Text>
                      <Text ff="monospace" size="xs">{s.modeId || "default"}</Text>
                    </Stack>
                    <Stack gap={2}>
                      <Text size="xs" c="dimmed" fw={600}>SERVIDOR</Text>
                      <Text ff="monospace" size="xs">{serverLabel(s)}</Text>
                    </Stack>
                  </Group>

                  {/* roster/formação/resultado vivem no painel por arquétipo
                      (spec 019) — sem bloco PLAYERS duplicado. */}
                  <SessionMatchDetail session={s} />

                  <Group gap="xl" wrap="wrap">
                    <Stack gap={2}>
                      <Text size="xs" c="dimmed" fw={600}>ABERTA EM</Text>
                      <Text size="xs">{new Date(s.openedAt).toLocaleString("pt-BR")}</Text>
                    </Stack>
                    {s.closedAt && (
                      <Stack gap={2}>
                        <Text size="xs" c="dimmed" fw={600}>FECHADA EM</Text>
                        <Text size="xs">{new Date(s.closedAt).toLocaleString("pt-BR")}</Text>
                      </Stack>
                    )}
                    {s.closeReason && (
                      <Stack gap={2}>
                        <Text size="xs" c="dimmed" fw={600}>MOTIVO</Text>
                        <Text size="xs">{closeReasonLabel(s.closeReason)}</Text>
                      </Stack>
                    )}
                  </Group>
                </Stack>
              </Box>
            </Collapse>
          </Table.Td>
        </Table.Tr>
      )}
    </>
  );
}


import { useEffect, useMemo, useState } from "react";
import {
  Alert,
  Button,
  Group,
  Loader,
  SegmentedControl,
  Select,
  Stack,
  Text,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { IconActivity } from "@tabler/icons-react";
import { api, type GameSession } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { usePolling } from "../lib/usePolling";
import { useSettings } from "../lib/settings";
import { EmptyState } from "../components/EmptyState";
import { SessionTable } from "../components/SessionTable";

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
  // filtro por um modo que sumiu da lista (o Select some com <2 modos) não
  // pode deixar a tabela vazia sem jeito de limpar.
  const effectiveModeFilter = modeFilter && distinctModes.includes(modeFilter) ? modeFilter : null;
  const rows = useMemo(
    () => (effectiveModeFilter ? allRows.filter((s) => modeOf(s) === effectiveModeFilter) : allRows),
    [allRows, effectiveModeFilter],
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
              value={effectiveModeFilter}
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
        <SessionTable
          rows={rows}
          expandedId={expandedId}
          onToggle={(id) => setExpandedId((prev) => (prev === id ? null : id))}
        />
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

import { useMemo, useState } from "react";
import { Alert, Loader, Select, SimpleGrid, Stack, Table, Text } from "@mantine/core";
import { BarChart, DonutChart, LineChart } from "@mantine/charts";
import { api, type MultiplayerMode, type MultiplayerProfile, type ParticipationStat } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { StatCard } from "../components/StatCard";
import { DashboardGrid, type WidgetDef } from "../components/DashboardGrid";
import { formatDuration } from "../lib/format";
import { ARCHETYPE_META } from "../lib/archetypeMeta";

// Widget -> arquétipo (spec 064): tabela fixa, cada widget já é condicionado
// a um campo do GameAnalyticsDetail que só existe pra um arquétipo. raw-stats
// fica de fora — cross-arquétipo por natureza (qualquer modo reporta stats).
const WIDGET_ARCHETYPE: Record<string, MultiplayerProfile["archetype"]> = {
  summary: "queue_arena",
  "queue-daily": "queue_arena",
  "outcomes-donut": "queue_arena",
  "participations-role": "host_world_dropin",
  "participation-daily": "host_world_dropin",
  "placement-distribution": "battle_royale",
  "placement-daily": "battle_royale",
  "mission-distribution": "coop_mission",
  "mission-daily": "coop_mission",
};

// seletor de modo (spec 009/064): sentinela porque o Select do Mantine não
// lida bem com value="" (cai no branch de "sem seleção").
const ALL_MODES = "__all__";

function modeLabel(modeId: string, modes: MultiplayerMode[]): string {
  if (!modeId) return "Sem modo";
  return modes.find((m) => m.modeId === modeId)?.displayName || modeId;
}

// rótulos amigáveis dos motivos de fechamento (spec 065, session health) —
// dado cru, sem pré-classificar o que "conta como abandono" (decisão do
// analytics-service, spec 013): o usuário lê os motivos e interpreta.
const CLOSE_REASON_LABELS: Record<string, string> = {
  result: "Concluída",
  ttl: "Expirou (TTL)",
  admin: "Encerrada pelo operador",
  abandoned: "Abandonada (crash)",
  host_left: "Host saiu",
  host_killed: "Host morto",
};

const CLOSE_REASON_COLORS: Record<string, string> = {
  result: "green.6",
  ttl: "gray.5",
  admin: "blue.5",
  abandoned: "red.6",
  host_left: "teal.5",
  host_killed: "grape.5",
};

// rótulos amigáveis dos eventos de participação (host_world_dropin).
const EVENT_LABELS: Record<string, string> = {
  JOINED: "Entrou",
  BOSS_DEFEATED: "Boss derrotado",
  HOST_KILLED: "Host morto",
  DIED: "Morreu",
  LEFT: "Saiu",
};

// paleta fixa por evento (categórica/domínio — não é status do sistema, é
// vocabulário do jogo, mesmo espírito do TEAM_COLORS em SessionMatchDetail).
const EVENT_COLORS: Record<string, string> = {
  JOINED: "blue.4",
  BOSS_DEFEATED: "teal.4",
  HOST_KILLED: "grape.4",
  DIED: "orange.4",
  LEFT: "gray.4",
};

const ROLE_LABELS: Record<string, string> = {
  host: "Host",
  coop: "Co-op",
  invader: "Invasor",
};
const ROLE_ORDER = ["host", "coop", "invader"];

const OUTCOME_LABELS: Record<string, string> = {
  SUCCESS: "Sucesso",
  FAILURE: "Falha",
  NEUTRAL: "Neutro",
};

// participationByRole agrupa por papel (Host/Co-op/Invasor) com um campo por
// tipo de evento — vira um bar chart empilhado de 3 categorias em vez de uma
// combinação "papel · evento" (achado do Vitor: com dado real, 10 combinações
// de rótulo longo colidiam e ficavam ilegíveis no eixo).
function participationByRole(stats: ParticipationStat[]) {
  const events = [...new Set(stats.map((s) => s.event))];
  const byRole = new Map<string, Record<string, number>>();
  for (const s of stats) {
    const row = byRole.get(s.role) ?? {};
    row[s.event] = (row[s.event] ?? 0) + s.count;
    byRole.set(s.role, row);
  }
  const roles = [...byRole.keys()].sort(
    (a, b) => (ROLE_ORDER.indexOf(a) === -1 ? 99 : ROLE_ORDER.indexOf(a)) -
      (ROLE_ORDER.indexOf(b) === -1 ? 99 : ROLE_ORDER.indexOf(b)),
  );
  const data = roles.map((role) => ({ role: ROLE_LABELS[role] ?? role, ...byRole.get(role) }));
  const series = events.map((e) => ({
    name: e,
    label: EVENT_LABELS[e] ?? e,
    color: EVENT_COLORS[e] ?? "gray.4",
  }));
  return { data, series };
}

// dailyLineChart é o LineChart genérico reaproveitado pelas 4 séries diárias
// (queue_arena + as 3 novas da spec 010) — mesmo visual.
function DailyLineChart({ data, label }: { data: { day: string; matches: number }[]; label: string }) {
  return (
    <LineChart
      h={220}
      data={data}
      dataKey="day"
      series={[{ name: "matches", label, color: "signal.5" }]}
      curveType="monotone"
      withDots={data.length < 40}
      tickLine="x"
      gridAxis="xy"
    />
  );
}

export function GameAnalytics({ gameId, modes = [] }: { gameId: string; modes?: MultiplayerMode[] }) {
  // "" (sentinela ALL_MODES) = sem filtro, mostra todos os modos somados
  // (spec 064: o seletor de página só oferece modos REAIS + "todos" — o
  // sentinela NO_MODE do widget antigo não sobrevive, ver plan.md: o
  // backend não distingue "filtrar por modo vazio" de "sem filtro", então
  // oferecer essa opção no seletor de página seria enganoso).
  const [selectedMode, setSelectedMode] = useState(ALL_MODES);
  const modeIdParam = selectedMode === ALL_MODES ? undefined : selectedMode;
  const { data, loading, error } = useAsync(
    (s) => api.gameAnalytics(gameId, { modeId: modeIdParam }, s),
    [gameId, modeIdParam],
  );

  const hasData = data && data.matches > 0;
  const participations = data?.roleParticipations ?? [];
  // modos distintos NOS DADOS RECEBIDOS (spec 009) — só tem >1 quando
  // "Todos os modos" está selecionado (um modo específico já vem filtrado
  // pelo backend, spec 064); decide se a coluna "Modo" da tabela aparece.
  const modeIdsInData = useMemo(() => [...new Set(participations.map((p) => p.modeId))], [participations]);
  // total por partida (independente de arquétipo) — só some se não houver nenhum resultado.
  const hasOutcomes = !!data && data.wins + data.losses + data.draws > 0;
  const roleChart = participations.length > 0 ? participationByRole(participations) : null;

  // arquétipo do modo selecionado (spec 064) — filtra o catálogo pra só
  // mostrar widgets desse arquétipo (+ raw-stats/session-health, sempre
  // elegíveis).
  const selectedArchetype =
    selectedMode !== ALL_MODES ? modes.find((m) => m.modeId === selectedMode)?.archetype : undefined;

  // Catálogo de widgets (spec 062): cada seção vira um widget com id estável,
  // disponível só quando a condição de dado (mesma de antes, agora
  // centralizada) é satisfeita. A ordem aqui é só o default inicial — o
  // operador reordena/redimensiona/esconde livremente (persistido por jogo).
  const widgets = useMemo<WidgetDef[]>(() => {
    if (!data) return [];
    const out: WidgetDef[] = [];

    if (hasData) {
      out.push({
        id: "summary",
        title: "Resumo",
        defaultH: 5,
        icon: ARCHETYPE_META.queue_arena.icon,
        render: () => (
          <SimpleGrid cols={{ base: 2, sm: 3 }}>
            <StatCard label="Partidas" value={String(data.matches)} />
            <StatCard label="Jogadores únicos" value={String(data.uniquePlayers)} />
            <StatCard label="Score médio" value={data.avgScore.toFixed(1)} />
            <StatCard label="Vitórias" value={String(data.wins)} />
            <StatCard label="Derrotas" value={String(data.losses)} />
            <StatCard label="Empates" value={String(data.draws)} />
          </SimpleGrid>
        ),
      });
    }

    if (data.daily.length > 0) {
      out.push({
        id: "queue-daily",
        title: "Partidas por dia",
        defaultH: 9,
        icon: ARCHETYPE_META.queue_arena.icon,
        render: () => <DailyLineChart data={data.daily} label="Partidas" />,
      });
    }

    if (hasOutcomes) {
      out.push({
        id: "outcomes-donut",
        title: "Vitórias × derrotas × empates",
        defaultH: 8,
        icon: ARCHETYPE_META.queue_arena.icon,
        render: () => (
          <DonutChart
            size={160}
            thickness={22}
            withLabelsLine
            withLabels
            data={[
              { name: "Vitórias", value: data.wins, color: "green.6" },
              { name: "Derrotas", value: data.losses, color: "red.6" },
              { name: "Empates", value: data.draws, color: "yellow.6" },
            ]}
          />
        ),
      });
    }

    // host_world_dropin (spec 004/020/009): participações por papel/evento/outcome.
    // Seletor de modo interno REMOVIDO (spec 064) — superado pelo seletor de
    // página, que já filtra no backend; modeIdsInData só tem >1 elemento
    // quando "Todos os modos" está selecionado (um modo específico já vem
    // filtrado, então a coluna "Modo" não teria variação pra mostrar).
    if (roleChart) {
      out.push({
        id: "participations-role",
        title: "Participações por papel",
        defaultH: Math.max(11, Math.ceil(roleChart.data.length * 70 / 30) + 6),
        icon: ARCHETYPE_META.host_world_dropin.icon,
        render: () => (
          <Stack gap="md">
            <BarChart
              h={Math.max(160, roleChart.data.length * 70)}
              data={roleChart.data}
              dataKey="role"
              series={roleChart.series}
              type="stacked"
              orientation="vertical"
              gridAxis="x"
              xAxisProps={{ allowDecimals: false }}
              withLegend
              legendProps={{ verticalAlign: "bottom" }}
            />
            <Table striped withTableBorder>
              <Table.Thead>
                <Table.Tr>
                  {modeIdsInData.length > 1 && <Table.Th>Modo</Table.Th>}
                  <Table.Th>Papel</Table.Th>
                  <Table.Th>Evento</Table.Th>
                  <Table.Th>Desfecho</Table.Th>
                  <Table.Th ta="right">Total</Table.Th>
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {participations.map((p) => (
                  <Table.Tr key={`${p.modeId}-${p.role}-${p.event}-${p.outcome}`}>
                    {modeIdsInData.length > 1 && <Table.Td c="dimmed">{modeLabel(p.modeId, modes)}</Table.Td>}
                    <Table.Td>{ROLE_LABELS[p.role] ?? p.role}</Table.Td>
                    <Table.Td>{EVENT_LABELS[p.event] ?? p.event}</Table.Td>
                    <Table.Td c="dimmed">{OUTCOME_LABELS[p.outcome] ?? (p.outcome || "—")}</Table.Td>
                    <Table.Td ta="right" ff="monospace">{p.count}</Table.Td>
                  </Table.Tr>
                ))}
              </Table.Tbody>
            </Table>
          </Stack>
        ),
      });
    }

    // battle_royale (BR4): distribuição de colocação.
    if (data.placement) {
      const placement = data.placement;
      out.push({
        id: "placement-distribution",
        title: "Distribuição de colocação",
        defaultH: 12,
        icon: ARCHETYPE_META.battle_royale.icon,
        render: () => (
          <Stack gap="md">
            <SimpleGrid cols={{ base: 2, sm: 3 }}>
              <StatCard label="1º lugar" value={String(placement.winners)} />
              <StatCard label="Top 10" value={String(placement.topN)} />
              <StatCard label="Resto" value={String(placement.rest)} />
              <StatCard label="Participações" value={String(placement.entries)} />
              <StatCard label="Sobrevivência média" value={formatDuration(placement.avgSurvivedMs)} />
            </SimpleGrid>
            <DonutChart
              size={160}
              thickness={22}
              withLabelsLine
              withLabels
              data={[
                { name: "1º lugar", value: placement.winners, color: "yellow.6" },
                { name: "Top 10", value: placement.topN, color: "blue.5" },
                { name: "Resto", value: placement.rest, color: "gray.5" },
              ]}
            />
          </Stack>
        ),
      });
    }

    // battle_royale (spec 010): série "partidas por dia" — não existia antes.
    if (data.placementDaily.length > 0) {
      out.push({
        id: "placement-daily",
        title: "Partidas de battle royale por dia",
        defaultH: 9,
        icon: ARCHETYPE_META.battle_royale.icon,
        render: () => <DailyLineChart data={data.placementDaily} label="Partidas" />,
      });
    }

    // coop_mission (C4): desfecho coletivo da missão.
    if (data.mission) {
      const mission = data.mission;
      out.push({
        id: "mission-distribution",
        title: "Desfecho das missões",
        defaultH: 10,
        icon: ARCHETYPE_META.coop_mission.icon,
        render: () => (
          <Stack gap="md">
            <SimpleGrid cols={{ base: 2, sm: 3 }}>
              <StatCard label="Missões cumpridas" value={String(mission.successes)} />
              <StatCard label="Missões falhas" value={String(mission.failures)} />
              <StatCard label="Taxa de sucesso" value={`${(mission.successRate * 100).toFixed(1)}%`} />
            </SimpleGrid>
            <DonutChart
              size={160}
              thickness={22}
              withLabelsLine
              withLabels
              data={[
                { name: "Cumpridas", value: mission.successes, color: "green.6" },
                { name: "Falhas", value: mission.failures, color: "red.6" },
              ]}
            />
          </Stack>
        ),
      });
    }

    // coop_mission (spec 010): série "missões por dia" — não existia antes.
    if (data.missionDaily.length > 0) {
      out.push({
        id: "mission-daily",
        title: "Missões por dia",
        defaultH: 9,
        icon: ARCHETYPE_META.coop_mission.icon,
        render: () => <DailyLineChart data={data.missionDaily} label="Missões" />,
      });
    }

    // host_world_dropin (spec 010): série "eventos de participação por dia" —
    // não existia antes (o arquétipo não tem partida terminal).
    if (data.participationDaily.length > 0) {
      out.push({
        id: "participation-daily",
        title: "Eventos de participação por dia",
        defaultH: 9,
        icon: ARCHETYPE_META.host_world_dropin.icon,
        render: () => <DailyLineChart data={data.participationDaily} label="Eventos" />,
      });
    }

    // stats brutos (spec 011): resumo agregado (soma/contagem/média) por métrica.
    // Sem ícone (spec 064): cross-arquétipo por natureza, já vem corretamente
    // filtrado pelo backend quando um modo está selecionado (spec 012).
    if (data.stats.length > 0) {
      const stats = data.stats;
      out.push({
        id: "raw-stats",
        title: "Stats agregados",
        defaultH: 4 + stats.length * 2,
        render: () => (
          <Table striped withTableBorder>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Stat</Table.Th>
                <Table.Th ta="right">Soma</Table.Th>
                <Table.Th ta="right">Ocorrências</Table.Th>
                <Table.Th ta="right">Média</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {stats.map((s) => (
                <Table.Tr key={s.key}>
                  <Table.Td>{s.key}</Table.Td>
                  <Table.Td ta="right" ff="monospace">{s.sum}</Table.Td>
                  <Table.Td ta="right" ff="monospace">{s.count}</Table.Td>
                  <Table.Td ta="right" ff="monospace">{s.avg.toFixed(1)}</Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        ),
      });
    }

    // saúde das sessões (spec 065/013): distribuição por motivo de
    // fechamento. Sem ícone fixo (mesmo tratamento de raw-stats): a tabela
    // de origem cobre TODO arquétipo (todo closeSession publica o evento).
    if (data.sessionHealth.length > 0) {
      const health = data.sessionHealth;
      const healthModeIds = [...new Set(health.map((h) => h.modeId ?? ""))];
      out.push({
        id: "session-health",
        title: "Saúde das sessões",
        defaultH: 10 + health.length * 2,
        render: () => (
          <Stack gap="md">
            <DonutChart
              size={160}
              thickness={22}
              withLabelsLine
              withLabels
              data={health.map((h) => ({
                name: CLOSE_REASON_LABELS[h.closeReason] ?? h.closeReason,
                value: h.count,
                color: CLOSE_REASON_COLORS[h.closeReason] ?? "gray.4",
              }))}
            />
            <Table striped withTableBorder>
              <Table.Thead>
                <Table.Tr>
                  {healthModeIds.length > 1 && <Table.Th>Modo</Table.Th>}
                  <Table.Th>Motivo</Table.Th>
                  <Table.Th ta="right">Contagem</Table.Th>
                  <Table.Th ta="right">Duração média</Table.Th>
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {health.map((h) => (
                  <Table.Tr key={`${h.modeId}-${h.closeReason}`}>
                    {healthModeIds.length > 1 && (
                      <Table.Td c="dimmed">{modeLabel(h.modeId ?? "", modes)}</Table.Td>
                    )}
                    <Table.Td>{CLOSE_REASON_LABELS[h.closeReason] ?? h.closeReason}</Table.Td>
                    <Table.Td ta="right" ff="monospace">{h.count}</Table.Td>
                    <Table.Td ta="right" ff="monospace">{formatDuration(h.avgDurationMs)}</Table.Td>
                  </Table.Tr>
                ))}
              </Table.Tbody>
            </Table>
          </Stack>
        ),
      });
    }

    // filtra o catálogo pelo arquétipo do modo selecionado (spec 064) — só
    // widgets desse arquétipo + raw-stats/session-health (sempre elegíveis,
    // cross-arquétipo, spec 064/065).
    if (!selectedArchetype) return out;
    return out.filter(
      (w) => w.id === "raw-stats" || w.id === "session-health" || WIDGET_ARCHETYPE[w.id] === selectedArchetype,
    );
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [data, hasData, hasOutcomes, roleChart, participations, modeIdsInData, selectedArchetype, modes]);

  return (
    <Stack>
      {loading && !data && <Loader />}
      {!!error && !data && <Alert color="critical">Analytics indisponível (analytics-service fora?).</Alert>}

      {modes.length > 1 && (
        <Select
          label="Modo"
          w={280}
          allowDeselect={false}
          value={selectedMode}
          onChange={(v) => v && setSelectedMode(v)}
          data={[
            { value: ALL_MODES, label: "Todos os modos" },
            ...modes.map((m) => ({ value: m.modeId, label: modeLabel(m.modeId, modes) })),
          ]}
        />
      )}

      {data && widgets.length === 0 && (
        <Text c="dimmed">
          Sem dados ainda — os agregados aparecem conforme as partidas terminam e reportam resultado.
        </Text>
      )}

      {widgets.length > 0 && <DashboardGrid gameId={gameId} widgets={widgets} />}
    </Stack>
  );
}

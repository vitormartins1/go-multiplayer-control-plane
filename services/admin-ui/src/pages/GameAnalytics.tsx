import { Alert, Loader, SimpleGrid, Stack, Table, Text, Title } from "@mantine/core";
import { BarChart, DonutChart, LineChart } from "@mantine/charts";
import { api } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { StatCard } from "../components/StatCard";
import { formatDuration } from "../lib/format";

// rótulos amigáveis dos eventos de participação (host_world_dropin).
const EVENT_LABELS: Record<string, string> = {
  JOINED: "Entrou",
  BOSS_DEFEATED: "Boss derrotado",
  HOST_KILLED: "Host morto",
  DIED: "Morreu",
  LEFT: "Saiu",
};

const ROLE_LABELS: Record<string, string> = {
  host: "Host",
  coop: "Co-op",
  invader: "Invasor",
};

const OUTCOME_LABELS: Record<string, string> = {
  SUCCESS: "Sucesso",
  FAILURE: "Falha",
  NEUTRAL: "Neutro",
};

export function GameAnalytics({ gameId }: { gameId: string }) {
  const { data, loading, error } = useAsync((s) => api.gameAnalytics(gameId, s), [gameId]);

  const hasData = data && data.matches > 0;
  const participations = data?.roleParticipations ?? [];
  // total por partida (independente de arquétipo) — só some se não houver nenhum resultado.
  const hasOutcomes = !!data && data.wins + data.losses + data.draws > 0;

  return (
    <Stack>
      {loading && !data && <Loader />}
      {!!error && !data && <Alert color="critical">Analytics indisponível (analytics-service fora?).</Alert>}

      {data && !hasData && participations.length === 0 && (
        <Text c="dimmed">
          Sem dados ainda — os agregados aparecem conforme as partidas terminam e reportam resultado.
        </Text>
      )}

      {hasData && (
        <>
          <SimpleGrid cols={{ base: 2, sm: 3 }}>
            <StatCard label="Partidas" value={String(data.matches)} />
            <StatCard label="Jogadores únicos" value={String(data.uniquePlayers)} />
            <StatCard label="Score médio" value={data.avgScore.toFixed(1)} />
            <StatCard label="Vitórias" value={String(data.wins)} />
            <StatCard label="Derrotas" value={String(data.losses)} />
            <StatCard label="Empates" value={String(data.draws)} />
          </SimpleGrid>

          {/* Partidas por dia (queue_arena, migration game_daily) — o dado já
              vinha da API e não era renderizado em lugar nenhum. */}
          {data.daily.length > 0 && (
            <Stack gap="xs">
              <Title order={5}>Partidas por dia</Title>
              <LineChart
                h={220}
                data={data.daily}
                dataKey="day"
                series={[{ name: "matches", label: "Partidas", color: "blue.5" }]}
                curveType="monotone"
                withDots={data.daily.length < 40}
                tickLine="x"
                gridAxis="xy"
              />
            </Stack>
          )}

          {hasOutcomes && (
            <Stack gap="xs">
              <Title order={5}>Vitórias × derrotas × empates</Title>
              <DonutChart
                size={160}
                thickness={22}
                withLabelsLine
                withLabels
                data={[
                  { name: "Vitórias", value: data.wins, color: "teal.6" },
                  { name: "Derrotas", value: data.losses, color: "red.6" },
                  { name: "Empates", value: data.draws, color: "gray.5" },
                ]}
              />
            </Stack>
          )}
        </>
      )}

      {/* host_world_dropin (spec 020): participações por papel/evento/outcome.
          Só aparece quando o jogo reportou eventos de participação. */}
      {participations.length > 0 && (
        <Stack gap="xs">
          <Title order={5}>Participações por papel</Title>
          <BarChart
            h={Math.max(160, participations.length * 34)}
            data={participations.map((p) => ({
              label: `${ROLE_LABELS[p.role] ?? p.role} · ${EVENT_LABELS[p.event] ?? p.event}`,
              count: p.count,
            }))}
            dataKey="label"
            series={[{ name: "count", label: "Total", color: "violet.5" }]}
            orientation="vertical"
            gridAxis="x"
            xAxisProps={{ allowDecimals: false }}
          />
          <Table striped withTableBorder>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Papel</Table.Th>
                <Table.Th>Evento</Table.Th>
                <Table.Th>Desfecho</Table.Th>
                <Table.Th ta="right">Total</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {participations.map((p) => (
                <Table.Tr key={`${p.role}-${p.event}-${p.outcome}`}>
                  <Table.Td>{ROLE_LABELS[p.role] ?? p.role}</Table.Td>
                  <Table.Td>{EVENT_LABELS[p.event] ?? p.event}</Table.Td>
                  <Table.Td c="dimmed">{OUTCOME_LABELS[p.outcome] ?? (p.outcome || "—")}</Table.Td>
                  <Table.Td ta="right" ff="monospace">{p.count}</Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </Stack>
      )}

      {/* battle_royale (BR4): distribuição de colocação. Só aparece quando o jogo
          reportou resultados por colocação (placement). */}
      {data?.placement && (
        <Stack gap="xs">
          <Title order={5}>Distribuição de colocação</Title>
          <SimpleGrid cols={{ base: 2, sm: 3 }}>
            <StatCard label="1º lugar" value={String(data.placement.winners)} />
            <StatCard label="Top 10" value={String(data.placement.topN)} />
            <StatCard label="Resto" value={String(data.placement.rest)} />
            <StatCard label="Participações" value={String(data.placement.entries)} />
            <StatCard label="Sobrevivência média" value={formatDuration(data.placement.avgSurvivedMs)} />
          </SimpleGrid>
          <DonutChart
            size={160}
            thickness={22}
            withLabelsLine
            withLabels
            data={[
              { name: "1º lugar", value: data.placement.winners, color: "yellow.6" },
              { name: "Top 10", value: data.placement.topN, color: "blue.5" },
              { name: "Resto", value: data.placement.rest, color: "gray.5" },
            ]}
          />
        </Stack>
      )}

      {/* coop_mission (C4): desfecho coletivo da missão. Só aparece quando o jogo
          reportou resultados coletivos (mission). */}
      {data?.mission && (
        <Stack gap="xs">
          <Title order={5}>Desfecho das missões</Title>
          <SimpleGrid cols={{ base: 2, sm: 3 }}>
            <StatCard label="Missões cumpridas" value={String(data.mission.successes)} />
            <StatCard label="Missões falhas" value={String(data.mission.failures)} />
            <StatCard label="Taxa de sucesso" value={`${(data.mission.successRate * 100).toFixed(1)}%`} />
          </SimpleGrid>
          <DonutChart
            size={160}
            thickness={22}
            withLabelsLine
            withLabels
            data={[
              { name: "Cumpridas", value: data.mission.successes, color: "teal.6" },
              { name: "Falhas", value: data.mission.failures, color: "red.6" },
            ]}
          />
        </Stack>
      )}
    </Stack>
  );
}

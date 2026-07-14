import { useMemo } from "react";
import { Alert, Grid, Group, Loader, Paper, SimpleGrid, Stack, Table, Text } from "@mantine/core";
import { DonutChart, LineChart } from "@mantine/charts";
import { IconActivity } from "@tabler/icons-react";
import { api, type Game } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { StatCard } from "../components/StatCard";
import { ChannelStrip } from "../components/ChannelStrip";
import { GameCover } from "../components/GameCover";
import { EmptyState } from "../components/EmptyState";

// Paleta cíclica pro donut de "partidas por jogo" — quantidade de jogos é
// dinâmica, diferente dos donuts do Analytics por-jogo (que sempre têm um
// conjunto fixo de categorias com cor semântica, tipo vitória/derrota).
const GAME_COLORS = ["signal.5", "nominal.5", "yellow.5", "grape.5", "orange.5", "teal.5", "pink.5", "blue.5"];

// Analytics: overview cross-game alimentado pelo analytics-service (ADR-0021).
// Agregados do resultado das partidas (fan-out) — totais + ranking de jogos.
// Redesign (spec 089): antes era o layout padrão de dashboard (3 KPI cards →
// line chart → tabela), sem nenhum dos padrões que o Analytics por-jogo já
// tinha ganhado (ChannelStrip, par stats+visual/gráfico lado a lado, spec
// 067/081/086) — esta página tinha ficado pra trás dessa evolução.
export function Analytics() {
  const { data, loading, error } = useAsync((s) => api.analyticsOverview(s), []);
  // Lista de jogos só pra montar a capa (GameCover) na tabela — mesmo padrão
  // de Rooms.tsx (um "listar todos" já existe, não vale buscar jogo a jogo).
  const { data: games } = useAsync((s) => api.games(s), []);
  const gamesById = useMemo(() => new Map((games ?? []).map((g) => [g.id, g])), [games]);

  const gameShareData = useMemo(
    () =>
      (data?.games ?? [])
        .filter((g) => g.matches > 0)
        .map((g, i) => ({
          name: g.gameName || g.gameId,
          value: g.matches,
          color: GAME_COLORS[i % GAME_COLORS.length],
        })),
    [data],
  );

  return (
    <Stack>
      <ChannelStrip
        icon={IconActivity}
        label="Geral"
        readout={data ? `${data.totals.matches} partidas` : undefined}
      />
      <Text size="sm" c="dimmed" mt={-8}>
        Agregados das partidas por jogo (worker pool, ADR-0021).
      </Text>

      {loading && !data && <Loader />}
      {!!error && !data && (
        <Alert color="critical">Analytics indisponível (analytics-service fora?).</Alert>
      )}

      {data && (
        <>
          <Grid>
            <Grid.Col span={{ base: 12, md: 8 }}>
              <Paper withBorder p="md" h="100%">
                <Text fw={600} size="sm" mb="xs">
                  Resumo
                </Text>
                <Group align="center" wrap="wrap" gap="lg">
                  <SimpleGrid cols={{ base: 1, sm: 3 }} style={{ flex: "1 1 260px" }}>
                    <StatCard size="lg" label="Partidas" value={String(data.totals.matches)} />
                    <StatCard size="lg" label="Jogadores únicos" value={String(data.totals.uniquePlayers)} />
                    <StatCard size="lg" label="Jogos com atividade" value={String(data.totals.games)} />
                  </SimpleGrid>
                  {gameShareData.length > 0 && (
                    <DonutChart
                      size={120}
                      thickness={18}
                      withLabelsLine
                      withLabels
                      data={gameShareData}
                    />
                  )}
                </Group>
              </Paper>
            </Grid.Col>
            <Grid.Col span={{ base: 12, md: 4 }}>
              <Paper withBorder p="md" h="100%">
                <Text fw={600} size="sm" mb="xs">
                  Partidas por dia
                </Text>
                {data.daily.length > 0 ? (
                  <LineChart
                    h={170}
                    data={data.daily}
                    dataKey="day"
                    series={[{ name: "matches", label: "Partidas", color: "signal.5" }]}
                    curveType="monotone"
                    withDots={data.daily.length < 40}
                    tickLine="x"
                    gridAxis="xy"
                  />
                ) : (
                  <Text size="sm" c="dimmed">
                    Sem série diária ainda.
                  </Text>
                )}
              </Paper>
            </Grid.Col>
          </Grid>

          {data.games.length === 0 ? (
            <EmptyState
              icon={IconActivity}
              message="Sem dados ainda — os agregados aparecem conforme as partidas terminam e reportam resultado."
            />
          ) : (
            <Stack gap="xs">
              <Text fw={600} size="sm">
                Jogos por partidas
              </Text>
              <Table striped highlightOnHover withTableBorder>
                <Table.Thead>
                  <Table.Tr>
                    <Table.Th>Jogo</Table.Th>
                    <Table.Th ta="right">Partidas</Table.Th>
                    <Table.Th ta="right">Jogadores únicos</Table.Th>
                    <Table.Th ta="right">Vitórias</Table.Th>
                    <Table.Th ta="right">Derrotas</Table.Th>
                    <Table.Th ta="right">Empates</Table.Th>
                    <Table.Th ta="right">Score médio</Table.Th>
                  </Table.Tr>
                </Table.Thead>
                <Table.Tbody>
                  {data.games.map((g) => {
                    const game: Game | undefined = gamesById.get(g.gameId);
                    return (
                      <Table.Tr key={g.gameId}>
                        <Table.Td>
                          <Group gap="xs" wrap="nowrap">
                            {game && <GameCover game={game} variant="mini" />}
                            <Text fw={500}>{g.gameName || g.gameId}</Text>
                          </Group>
                        </Table.Td>
                        <Table.Td ta="right" ff="monospace">{g.matches}</Table.Td>
                        <Table.Td ta="right" ff="monospace">{g.uniquePlayers}</Table.Td>
                        <Table.Td ta="right" ff="monospace">{g.wins}</Table.Td>
                        <Table.Td ta="right" ff="monospace">{g.losses}</Table.Td>
                        <Table.Td ta="right" ff="monospace">{g.draws}</Table.Td>
                        <Table.Td ta="right" ff="monospace">{g.avgScore.toFixed(1)}</Table.Td>
                      </Table.Tr>
                    );
                  })}
                </Table.Tbody>
              </Table>
            </Stack>
          )}
        </>
      )}
    </Stack>
  );
}

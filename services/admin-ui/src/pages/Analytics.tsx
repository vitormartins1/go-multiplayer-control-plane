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
import { formatDuration } from "../lib/format";

// Paleta cíclica pro donut de "sessões por jogo" — quantidade de jogos é
// dinâmica, diferente dos donuts do Analytics por-jogo (que sempre têm um
// conjunto fixo de categorias com cor semântica, tipo vitória/derrota).
const GAME_COLORS = ["signal.5", "nominal.5", "yellow.5", "grape.5", "orange.5", "teal.5", "pink.5", "blue.5"];

// Analytics: overview cross-game alimentado pelo analytics-service (ADR-0021).
// Desde a analytics/016 a medida comum é a SESSÃO fechada (mundo ou partida),
// que existe para todo arquétipo — antes era "partidas", só do queue_arena, e
// o Folclore Souls (host_world_dropin) aparecia zerado aqui. Vitória/derrota/
// score ficam no detalhe por jogo, onde fazem sentido.
export function Analytics() {
  const { data, loading, error } = useAsync((s) => api.analyticsOverview(s), []);
  // Lista de jogos só pra montar a capa (GameCover) na tabela — mesmo padrão
  // de Rooms.tsx (um "listar todos" já existe, não vale buscar jogo a jogo).
  const { data: games } = useAsync((s) => api.games(s), []);
  const gamesById = useMemo(() => new Map((games ?? []).map((g) => [g.id, g])), [games]);

  const gameShareData = useMemo(
    () =>
      (data?.games ?? [])
        .filter((g) => g.sessions > 0)
        .map((g, i) => ({
          name: g.gameName || g.gameId,
          value: g.sessions,
          color: GAME_COLORS[i % GAME_COLORS.length],
        })),
    [data],
  );
  const abandoned = (data?.games ?? []).reduce((acc, g) => acc + g.abandoned, 0);
  const showMatches = (data?.games ?? []).some((g) => g.matches > 0);

  return (
    <Stack>
      <ChannelStrip
        icon={IconActivity}
        label="Geral"
        readout={data ? `${data.totals.sessions} sessões` : undefined}
      />
      <Text size="sm" c="dimmed" mt={-8}>
        Sessões fechadas (mundos e partidas) de todos os jogos — worker pool do analytics-service (ADR-0021).
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
                  <SimpleGrid cols={{ base: 2, sm: 4 }} style={{ flex: "1 1 320px" }}>
                    <StatCard size="lg" label="Sessões" value={String(data.totals.sessions)} />
                    <StatCard size="lg" label="Jogadores únicos" value={String(data.totals.uniquePlayers)} />
                    <StatCard size="lg" label="Jogos com atividade" value={String(data.totals.games)} />
                    <StatCard
                      size="lg"
                      label="Abandono"
                      value={pct(abandoned, data.totals.sessions)}
                      bad={abandoned > 0}
                    />
                  </SimpleGrid>
                  {gameShareData.length > 1 && (
                    <DonutChart size={120} thickness={18} withLabelsLine withLabels data={gameShareData} />
                  )}
                </Group>
              </Paper>
            </Grid.Col>
            <Grid.Col span={{ base: 12, md: 4 }}>
              <Paper withBorder p="md" h="100%">
                <Text fw={600} size="sm" mb="xs">
                  Sessões por dia
                </Text>
                {data.daily.length > 0 ? (
                  <LineChart
                    h={170}
                    data={data.daily}
                    dataKey="day"
                    series={[{ name: "sessions", label: "Sessões", color: "signal.5" }]}
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
              message="Sem dados ainda — os agregados aparecem conforme as sessões fecham."
            />
          ) : (
            <Stack gap="xs">
              <Text fw={600} size="sm">
                Jogos por sessões
              </Text>
              <Table striped highlightOnHover withTableBorder>
                <Table.Thead>
                  <Table.Tr>
                    <Table.Th>Jogo</Table.Th>
                    <Table.Th ta="right">Sessões</Table.Th>
                    <Table.Th ta="right">Jogadores únicos</Table.Th>
                    <Table.Th ta="right">Duração média</Table.Th>
                    <Table.Th ta="right">Abandono</Table.Th>
                    {showMatches && <Table.Th ta="right">Partidas (arena)</Table.Th>}
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
                        <Table.Td ta="right" ff="monospace">{g.sessions}</Table.Td>
                        <Table.Td ta="right" ff="monospace">{g.uniquePlayers}</Table.Td>
                        <Table.Td ta="right" ff="monospace">
                          {g.sessions > 0 ? formatDuration(g.avgSessionMs) : "—"}
                        </Table.Td>
                        <Table.Td ta="right" ff="monospace" c={g.abandoned > 0 ? "critical" : undefined}>
                          {pct(g.abandoned, g.sessions)}
                        </Table.Td>
                        {showMatches && <Table.Td ta="right" ff="monospace">{g.matches}</Table.Td>}
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

// pct formata part/total como porcentagem inteira ("—" sem base).
function pct(part: number, total: number): string {
  return total > 0 ? `${Math.round((part / total) * 100)}%` : "—";
}

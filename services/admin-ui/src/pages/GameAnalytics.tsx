import { Alert, Loader, SimpleGrid, Stack, Text } from "@mantine/core";
import { api } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { StatCard } from "../components/StatCard";

export function GameAnalytics({ gameId }: { gameId: string }) {
  const { data, loading, error } = useAsync((s) => api.analyticsOverview(s), []);

  const stat = data?.games.find((g) => g.gameId === gameId);

  return (
    <Stack>
      {loading && !data && <Loader />}
      {!!error && !data && <Alert color="red">Analytics indisponível (analytics-service fora?).</Alert>}

      {data && !stat && (
        <Text c="dimmed">
          Sem dados ainda — os agregados aparecem conforme as partidas terminam e reportam resultado.
        </Text>
      )}

      {stat && (
        <SimpleGrid cols={{ base: 2, sm: 3 }}>
          <StatCard label="Partidas" value={String(stat.matches)} />
          <StatCard label="Jogadores únicos" value={String(stat.uniquePlayers)} />
          <StatCard label="Score médio" value={stat.avgScore.toFixed(1)} />
          <StatCard label="Vitórias" value={String(stat.wins)} />
          <StatCard label="Derrotas" value={String(stat.losses)} />
          <StatCard label="Empates" value={String(stat.draws)} />
        </SimpleGrid>
      )}
    </Stack>
  );
}

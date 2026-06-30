import { Anchor, Breadcrumbs, Stack, Tabs, Text, Title } from "@mantine/core";
import { IconChartBar, IconSettings, IconTrophy, IconUsers } from "@tabler/icons-react";
import { Link, useParams } from "react-router-dom";
import { api } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { GameAchievements } from "./GameAchievements";
import { GameLeaderboards } from "./GameLeaderboards";
import { GamePlayers } from "./GamePlayers";
import { ComingSoon } from "../components/ComingSoon";

export function GameDetail() {
  const { gameId = "" } = useParams();
  const { data: games } = useAsync((s) => api.games(s), []);
  const game = games?.find((g) => g.id === gameId);

  return (
    <Stack>
      <Breadcrumbs separator="›">
        <Anchor component={Link} to="/games">
          Games
        </Anchor>
        <Text>{game?.name ?? gameId}</Text>
      </Breadcrumbs>
      <Title order={3}>{game?.name ?? "Jogo"}</Title>

      <Tabs defaultValue="achievements">
        <Tabs.List>
          <Tabs.Tab value="achievements" leftSection={<IconTrophy size={16} />}>
            Achievements
          </Tabs.Tab>
          <Tabs.Tab value="leaderboards" leftSection={<IconChartBar size={16} />}>
            Leaderboards
          </Tabs.Tab>
          <Tabs.Tab value="players" leftSection={<IconUsers size={16} />}>
            Players
          </Tabs.Tab>
          <Tabs.Tab value="settings" leftSection={<IconSettings size={16} />}>
            Configurações
          </Tabs.Tab>
        </Tabs.List>

        <Tabs.Panel value="achievements" pt="md">
          <GameAchievements gameId={gameId} />
        </Tabs.Panel>
        <Tabs.Panel value="leaderboards" pt="md">
          <GameLeaderboards gameId={gameId} />
        </Tabs.Panel>
        <Tabs.Panel value="players" pt="md">
          <GamePlayers gameId={gameId} />
        </Tabs.Panel>
        <Tabs.Panel value="settings" pt="md">
          <ComingSoon what="Configurações do jogo" />
        </Tabs.Panel>
      </Tabs>
    </Stack>
  );
}

import { Anchor, Breadcrumbs, Stack, Tabs, Text, Title } from "@mantine/core";
import { IconCloud, IconDatabase, IconTrophy } from "@tabler/icons-react";
import { Link, useParams } from "react-router-dom";
import { PlayerAchievements } from "./PlayerAchievements";
import { ComingSoon } from "../components/ComingSoon";

export function PlayerDetail() {
  const { playerId = "" } = useParams();

  return (
    <Stack>
      <Breadcrumbs separator="›">
        <Anchor component={Link} to="/players">
          Players
        </Anchor>
        <Text>{playerId.slice(0, 8)}…</Text>
      </Breadcrumbs>
      <Title order={3}>Jogador</Title>
      <Text c="dimmed" size="sm" mt={-8}>
        {playerId}
      </Text>

      <Tabs defaultValue="achievements">
        <Tabs.List>
          <Tabs.Tab value="achievements" leftSection={<IconTrophy size={16} />}>
            Achievements
          </Tabs.Tab>
          <Tabs.Tab value="player-data" leftSection={<IconDatabase size={16} />}>
            Player data
          </Tabs.Tab>
          <Tabs.Tab value="cloud-saves" leftSection={<IconCloud size={16} />}>
            Cloud saves
          </Tabs.Tab>
        </Tabs.List>

        <Tabs.Panel value="achievements" pt="md">
          <PlayerAchievements playerId={playerId} />
        </Tabs.Panel>
        <Tabs.Panel value="player-data" pt="md">
          <ComingSoon what="Dados do jogador (player-data)" />
        </Tabs.Panel>
        <Tabs.Panel value="cloud-saves" pt="md">
          <ComingSoon what="Cloud saves do jogador" />
        </Tabs.Panel>
      </Tabs>
    </Stack>
  );
}

import { Anchor, Badge, Breadcrumbs, Group, Stack, Tabs, Text, Title } from "@mantine/core";
import { IconCloud, IconDatabase, IconTrophy } from "@tabler/icons-react";
import { Link, useParams } from "react-router-dom";
import { PlayerAchievements } from "./PlayerAchievements";
import { PlayerDataView } from "./PlayerDataView";
import { CloudSaveView } from "./CloudSaveView";
import { useGames } from "../lib/games";
import { PlayerAvatar } from "../components/PlayerAvatar";

export function PlayerDetail() {
  const { playerId = "" } = useParams();
  // O jogo é o contexto global (header). As abas (dados por-jogo) o consomem.
  const { activeGame } = useGames();
  const gameId = activeGame?.id ?? null;

  return (
    <Stack>
      <Breadcrumbs separator="›">
        <Anchor component={Link} to="/players">
          Players
        </Anchor>
        <Text>{playerId.slice(0, 8)}…</Text>
      </Breadcrumbs>
      <Group justify="space-between" align="flex-end">
        <Group>
          <PlayerAvatar seed={playerId} size={48} />
          <div>
            <Title order={3}>Jogador</Title>
            <Text c="dimmed" size="sm" mt={-4}>
              {playerId}
            </Text>
          </div>
        </Group>
        {activeGame ? (
          <Badge variant="light" size="lg">
            Jogo: {activeGame.name}
          </Badge>
        ) : (
          <Badge variant="light" color="gray" size="lg">
            Nenhum jogo selecionado
          </Badge>
        )}
      </Group>

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
          <PlayerAchievements playerId={playerId} gameId={gameId} />
        </Tabs.Panel>
        <Tabs.Panel value="player-data" pt="md">
          <PlayerDataView playerId={playerId} gameId={gameId} />
        </Tabs.Panel>
        <Tabs.Panel value="cloud-saves" pt="md">
          <CloudSaveView playerId={playerId} gameId={gameId} />
        </Tabs.Panel>
      </Tabs>
    </Stack>
  );
}

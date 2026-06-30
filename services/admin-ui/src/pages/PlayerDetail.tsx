import { Alert, Anchor, Breadcrumbs, Group, Loader, Select, Stack, Tabs, Text, Title } from "@mantine/core";
import { IconCloud, IconDatabase, IconTrophy } from "@tabler/icons-react";
import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { api } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { PlayerAchievements } from "./PlayerAchievements";
import { PlayerDataView } from "./PlayerDataView";
import { CloudSaveView } from "./CloudSaveView";
import { PlayerAvatar } from "../components/PlayerAvatar";

export function PlayerDetail() {
  const { playerId = "" } = useParams();
  // Os jogos são escopados a ESTE jogador (presença reversa), não a um contexto
  // global. O primeiro vem pré-selecionado (ADR-0019 + spec 004).
  const { data: games, loading, error } = useAsync((s) => api.gamesOfPlayer(playerId, s), [playerId]);
  const [gameId, setGameId] = useState<string | null>(null);

  // ao carregar os jogos do player, seleciona o primeiro (se ainda não há escolha).
  useEffect(() => {
    if (games && games.length > 0 && !gameId) setGameId(games[0].id);
  }, [games, gameId]);

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
        {games && games.length > 0 && (
          <Select
            label="Jogo"
            data={games.map((g) => ({ value: g.id, label: g.name }))}
            value={gameId}
            onChange={setGameId}
            w={240}
            allowDeselect={false}
            searchable
          />
        )}
      </Group>

      {loading && <Loader />}
      {!!error && <Alert color="red">Falha ao carregar os jogos do jogador (game-registry indisponível?).</Alert>}
      {games && games.length === 0 && (
        <Text c="dimmed">Este jogador ainda não entrou em nenhum jogo (a presença é registrada no login).</Text>
      )}

      {games && games.length > 0 && (
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
      )}
    </Stack>
  );
}

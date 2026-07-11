import { Alert, Anchor, Breadcrumbs, Group, Loader, Stack, Tabs, Text, Title } from "@mantine/core";
import { IconCloud, IconDatabase, IconTrophy } from "@tabler/icons-react";
import { useEffect, useState } from "react";
import { Link, useLocation, useParams } from "react-router-dom";
import { api } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { PlayerAchievements } from "./PlayerAchievements";
import { PlayerDataView } from "./PlayerDataView";
import { CloudSaveView } from "./CloudSaveView";
import { PlayerAvatar, isGuest } from "../components/PlayerAvatar";
import { GuestBadge } from "../components/GuestBadge";
import { GameChipsSelector } from "../components/GameChipsSelector";

export function PlayerDetail() {
  const { playerId = "" } = useParams();
  // fromGameId: jogo de origem quando a navegação partiu da aba Players de um
  // jogo (spec 040) — pré-seleciona esse jogo em vez do primeiro da lista.
  const { state } = useLocation() as { state?: { fromGameId?: string } };
  const fromGameId = state?.fromGameId;
  // Os jogos são escopados a ESTE jogador (presença reversa), não a um contexto
  // global. O primeiro vem pré-selecionado (ADR-0019 + spec 004).
  const { data: games, loading, error } = useAsync((s) => api.gamesOfPlayer(playerId, s), [playerId]);
  const [gameId, setGameId] = useState<string | null>(null);

  // Handle do jogador (spec 040): best-effort — guest não tem conta (curto-
  // circuito sem fetch) e falha/404 mantém o fallback "Jogador" do header.
  const { data: player } = useAsync(
    (s) => (isGuest(playerId) ? Promise.resolve(null) : api.player(playerId, s).catch(() => null)),
    [playerId],
  );
  const displayName = isGuest(playerId) ? "Convidado" : (player?.handle ?? "Jogador");

  // Seleciona o jogo de origem (quando presente na lista) ou o primeiro sempre
  // que a lista muda (inclui trocar de jogador: o router reusa o componente,
  // então precisamos re-selecionar — senão o jogo do jogador anterior ficaria
  // preso). Seleção manual persiste dentro do mesmo jogador (a lista só refaz
  // quando playerId muda).
  useEffect(() => {
    if (!games || games.length === 0) {
      setGameId(null);
      return;
    }
    const origin = games.find((g) => g.id === fromGameId);
    setGameId(origin ? origin.id : games[0].id);
  }, [games, fromGameId]);

  return (
    <Stack>
      <Breadcrumbs separator="›">
        <Anchor component={Link} to="/players">
          Players
        </Anchor>
        <Text>{player?.handle ?? `${playerId.slice(0, 8)}…`}</Text>
      </Breadcrumbs>
      <Group justify="space-between" align="flex-end">
        <Group>
          <PlayerAvatar seed={playerId} size={48} />
          <div>
            <Group gap="xs">
              <Title order={3}>{displayName}</Title>
              <GuestBadge playerId={playerId} />
            </Group>
            <Text c="dimmed" size="sm" mt={-4}>
              {playerId}
            </Text>
          </div>
        </Group>
        {games && games.length > 0 && (
          <GameChipsSelector games={games} value={gameId} onChange={setGameId} />
        )}
      </Group>

      {loading && <Loader />}
      {!!error && <Alert color="critical">Falha ao carregar os jogos do jogador (game-registry indisponível?).</Alert>}
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

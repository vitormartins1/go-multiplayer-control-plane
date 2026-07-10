import { Anchor, Badge, Breadcrumbs, Group, Stack, Tabs, Text, Title } from "@mantine/core";
import {
  IconChartBar,
  IconChartPie,
  IconGitBranch,
  IconServer2,
  IconSettings,
  IconTrophy,
  IconUsers,
} from "@tabler/icons-react";
import { useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { api } from "../lib/api";
import { ARCHETYPE_META } from "../lib/archetypeMeta";
import { useAsync } from "../lib/useAsync";
import { GameCover } from "../components/GameCover";
import { GameAchievements } from "./GameAchievements";
import { GameAnalytics } from "./GameAnalytics";
import { GameEventRules } from "./GameEventRules";
import { GameLeaderboards } from "./GameLeaderboards";
import { GamePlayers } from "./GamePlayers";
import { GameSessions } from "./GameSessions";
import { GameSettings } from "./GameSettings";

function CountBadge({ count, color = "gray" }: { count?: number; color?: string }) {
  if (count == null) return null;
  return (
    <Badge size="xs" variant="filled" color={color}>
      {count > 999 ? "999" : count}
    </Badge>
  );
}

export function GameDetail() {
  const { gameId = "" } = useParams();
  const { data: games, reload: reloadGames } = useAsync((s) => api.games(s), []);
  const { data: catalog } = useAsync((s) => api.gameCatalog(gameId, s), [gameId]);
  const { data: gamePlayers } = useAsync((s) => api.gamePlayers(gameId, s), [gameId]);
  const { data: sessions } = useAsync((s) => api.gameSessions(gameId, s), [gameId]);
  // modos do jogo p/ o header (030) — único fetch novo da página, sem polling.
  const { data: gameModes } = useAsync((s) => api.listMultiplayerModes(gameId, s), [gameId]);

  const game = games?.find((g) => g.id === gameId);

  // Navegação Achievements -> Event Rules com uma regra já focada (spec 057):
  // Tabs vira controlado só por causa disso; pendingRuleFocus viaja pra
  // GameEventRules, que foca e (se edit) dispara o form de edição, depois
  // se auto-limpa (onConsumePendingRuleFocus) pra não re-disparar.
  const [activeTab, setActiveTab] = useState<string | null>("achievements");
  const [pendingRuleFocus, setPendingRuleFocus] = useState<{ ruleId: string; edit: boolean } | null>(null);

  const onlinePlayerIds = useMemo(() => {
    if (!sessions) return null;
    return new Set(sessions.active.flatMap((s) => s.players.map((p) => p.id)));
  }, [sessions]);

  // Players registrados que estão em sessão ativa (intersection session ∩ registry).
  // Guests anônimos têm IDs "guest-<uuid>" que não aparecem no game-registry,
  // por isso o badge só conta quem tem conta registrada.
  const matchedOnlineCount = useMemo(() => {
    if (!onlinePlayerIds || !gamePlayers) return null;
    return gamePlayers.filter((p) => onlinePlayerIds.has(p.playerId)).length;
  }, [onlinePlayerIds, gamePlayers]);

  return (
    <Stack>
      <Breadcrumbs separator="›">
        <Anchor component={Link} to="/games">
          Games
        </Anchor>
        <Text>{game?.name ?? gameId}</Text>
      </Breadcrumbs>
      {/* Fotografia do jogo no primeiro olhar (030, ajustada pós-validação):
          capa mini + nome + badges dos modos numa linha só. Slug fica na aba
          Configurações; totais ficam nos badges das tabs (sem StatCards). */}
      <Group gap="sm" align="center">
        {game && <GameCover game={game} variant="mini" />}
        <Title order={3}>{game?.name ?? "Jogo"}</Title>
        {gameModes &&
          (gameModes.length === 0 ? (
            <Badge variant="light" color="gray">
              sem multiplayer
            </Badge>
          ) : (
            gameModes.map((m) => (
              <Badge key={m.modeId} variant="light" color={ARCHETYPE_META[m.archetype].color}>
                {m.modeId}
              </Badge>
            ))
          ))}
      </Group>

      {/* keepMounted=false: painel desmontado ao trocar de aba — o polling da
          aba Sessions só roda com ela visível. */}
      <Tabs value={activeTab} onChange={setActiveTab} keepMounted={false}>
        <Tabs.List>
          <Tabs.Tab
            value="achievements"
            leftSection={<IconTrophy size={16} />}
            rightSection={<CountBadge count={catalog?.length} />}
          >
            Achievements
          </Tabs.Tab>
          <Tabs.Tab value="event-rules" leftSection={<IconGitBranch size={16} />}>
            Event Rules
          </Tabs.Tab>
          <Tabs.Tab value="leaderboards" leftSection={<IconChartBar size={16} />}>
            Leaderboards
          </Tabs.Tab>
          <Tabs.Tab
            value="players"
            leftSection={<IconUsers size={16} />}
            rightSection={
              gamePlayers != null ? (
                <Group gap={4} wrap="nowrap">
                  {matchedOnlineCount != null && matchedOnlineCount > 0 && (
                    <CountBadge count={matchedOnlineCount} color="green" />
                  )}
                  <CountBadge count={gamePlayers.length} />
                </Group>
              ) : null
            }
          >
            Players
          </Tabs.Tab>
          <Tabs.Tab
            value="sessions"
            leftSection={<IconServer2 size={16} />}
            rightSection={
              sessions != null && sessions.active.length > 0 ? (
                <CountBadge count={sessions.active.length} color="green" />
              ) : null
            }
          >
            Sessions
          </Tabs.Tab>
          <Tabs.Tab value="analytics" leftSection={<IconChartPie size={16} />}>
            Analytics
          </Tabs.Tab>
          <Tabs.Tab value="settings" leftSection={<IconSettings size={16} />}>
            Configurações
          </Tabs.Tab>
        </Tabs.List>

        <Tabs.Panel value="achievements" pt="md">
          <GameAchievements
            gameId={gameId}
            onNavigateToRule={(ruleId, opts) => {
              setPendingRuleFocus({ ruleId, ...opts });
              setActiveTab("event-rules");
            }}
          />
        </Tabs.Panel>
        <Tabs.Panel value="event-rules" pt="md">
          <GameEventRules
            gameId={gameId}
            pendingRuleFocus={pendingRuleFocus}
            onConsumePendingRuleFocus={() => setPendingRuleFocus(null)}
          />
        </Tabs.Panel>
        <Tabs.Panel value="leaderboards" pt="md">
          <GameLeaderboards gameId={gameId} />
        </Tabs.Panel>
        <Tabs.Panel value="players" pt="md">
          <GamePlayers gameId={gameId} onlinePlayerIds={onlinePlayerIds ?? undefined} />
        </Tabs.Panel>
        <Tabs.Panel value="sessions" pt="md">
          <GameSessions gameId={gameId} />
        </Tabs.Panel>
        <Tabs.Panel value="analytics" pt="md">
          <GameAnalytics gameId={gameId} />
        </Tabs.Panel>
        <Tabs.Panel value="settings" pt="md">
          {game ? (
            <GameSettings game={game} onRenamed={reloadGames} />
          ) : (
            <Text c="dimmed">Carregando…</Text>
          )}
        </Tabs.Panel>
      </Tabs>
    </Stack>
  );
}

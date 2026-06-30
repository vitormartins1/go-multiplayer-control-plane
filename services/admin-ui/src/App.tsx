import { AppShell, Button, Group, Menu, NavLink, Text, Title } from "@mantine/core";
import { Link, Navigate, Route, Routes, useLocation } from "react-router-dom";
import {
  IconChartBar,
  IconChevronDown,
  IconDeviceGamepad2,
  IconLayoutDashboard,
  IconUsers,
} from "@tabler/icons-react";

import { Overview } from "./pages/Overview";
import { Games } from "./pages/Games";
import { GameDetail } from "./pages/GameDetail";
import { Players } from "./pages/Players";
import { PlayerDetail } from "./pages/PlayerDetail";
import { Analytics } from "./pages/Analytics";
import { GameCover } from "./components/GameCover";
import { useGames } from "./lib/games";

const NAV = [
  { to: "/", label: "Visão geral", icon: IconLayoutDashboard },
  { to: "/games", label: "Games", icon: IconDeviceGamepad2 },
  { to: "/players", label: "Players", icon: IconUsers },
  { to: "/analytics", label: "Analytics", icon: IconChartBar },
];

// GameSwitcher é o seletor do jogo ativo no header — contexto compartilhado por
// todo o painel (abas do jogador, biblioteca). Persistido via GameProvider.
function GameSwitcher() {
  const { games, activeGame, setActiveGameId } = useGames();
  if (games.length === 0) return null;

  return (
    <Menu position="bottom-end" width={240} withinPortal>
      <Menu.Target>
        <Button variant="default" px="xs" rightSection={<IconChevronDown size={14} />}>
          <Group gap="xs" wrap="nowrap">
            {activeGame ? (
              <>
                <GameCover game={activeGame} variant="mini" />
                <Text size="sm" fw={600} lineClamp={1} maw={140}>
                  {activeGame.name}
                </Text>
              </>
            ) : (
              <Text size="sm" c="dimmed">
                Selecione um jogo
              </Text>
            )}
          </Group>
        </Button>
      </Menu.Target>
      <Menu.Dropdown>
        <Menu.Label>Jogo ativo</Menu.Label>
        {games.map((g) => (
          <Menu.Item
            key={g.id}
            leftSection={<GameCover game={g} variant="mini" />}
            onClick={() => setActiveGameId(g.id)}
            bg={g.id === activeGame?.id ? "var(--mantine-color-blue-light)" : undefined}
          >
            {g.name}
          </Menu.Item>
        ))}
      </Menu.Dropdown>
    </Menu>
  );
}

export function App() {
  const { pathname } = useLocation();
  const active = (to: string) => (to === "/" ? pathname === "/" : pathname.startsWith(to));

  return (
    <AppShell header={{ height: 56 }} navbar={{ width: 230, breakpoint: "sm" }} padding="md">
      <AppShell.Header>
        <Group h="100%" px="md" justify="space-between">
          <Title order={4}>🎮 Admin Console</Title>
          <GameSwitcher />
        </Group>
      </AppShell.Header>

      <AppShell.Navbar p="sm">
        {NAV.map((n) => (
          <NavLink
            key={n.to}
            component={Link}
            to={n.to}
            label={n.label}
            leftSection={<n.icon size={18} />}
            active={active(n.to)}
          />
        ))}
      </AppShell.Navbar>

      <AppShell.Main>
        <Routes>
          <Route path="/" element={<Overview />} />
          <Route path="/games" element={<Games />} />
          <Route path="/games/:gameId" element={<GameDetail />} />
          <Route path="/players" element={<Players />} />
          <Route path="/players/:playerId" element={<PlayerDetail />} />
          <Route path="/analytics" element={<Analytics />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </AppShell.Main>
    </AppShell>
  );
}

import { AppShell, Group, NavLink, Title } from "@mantine/core";
import { Link, Navigate, Route, Routes, useLocation } from "react-router-dom";
import {
  IconChartBar,
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

const NAV = [
  { to: "/", label: "Visão geral", icon: IconLayoutDashboard },
  { to: "/games", label: "Games", icon: IconDeviceGamepad2 },
  { to: "/players", label: "Players", icon: IconUsers },
  { to: "/analytics", label: "Analytics", icon: IconChartBar },
];

export function App() {
  const { pathname } = useLocation();
  const active = (to: string) => (to === "/" ? pathname === "/" : pathname.startsWith(to));

  return (
    <AppShell header={{ height: 56 }} navbar={{ width: 230, breakpoint: "sm" }} padding="md">
      <AppShell.Header>
        <Group h="100%" px="md">
          <Title order={4}>🎮 Admin Console</Title>
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

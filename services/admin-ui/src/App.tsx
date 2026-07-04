import { AppShell, Center, Group, Loader, NavLink, Title } from "@mantine/core";
import { lazy, Suspense } from "react";
import { Link, Navigate, Route, Routes, useLocation } from "react-router-dom";
import {
  IconChartBar,
  IconDeviceGamepad2,
  IconDoorEnter,
  IconServer,
  IconUsers,
} from "@tabler/icons-react";

// Rotas carregadas sob demanda (code-splitting): cada página vira um chunk
// próprio, então os deps pesados (DiceBear nas capas/avatares, @xyflow no mapa
// de topologia) só baixam ao navegar. Named exports -> mapeados p/ default.
const Overview = lazy(() => import("./pages/Overview").then((m) => ({ default: m.Overview })));
const Games = lazy(() => import("./pages/Games").then((m) => ({ default: m.Games })));
const GameDetail = lazy(() => import("./pages/GameDetail").then((m) => ({ default: m.GameDetail })));
const Players = lazy(() => import("./pages/Players").then((m) => ({ default: m.Players })));
const PlayerDetail = lazy(() => import("./pages/PlayerDetail").then((m) => ({ default: m.PlayerDetail })));
const Analytics = lazy(() => import("./pages/Analytics").then((m) => ({ default: m.Analytics })));
const Rooms = lazy(() => import("./pages/Rooms").then((m) => ({ default: m.Rooms })));

const NAV = [
  { to: "/games", label: "Games", icon: IconDeviceGamepad2 },
  { to: "/players", label: "Players", icon: IconUsers },
  { to: "/rooms", label: "Rooms", icon: IconDoorEnter },
  { to: "/analytics", label: "Analytics", icon: IconChartBar },
  { to: "/infrastructure", label: "Infrastructure", icon: IconServer },
];

export function App() {
  const { pathname } = useLocation();
  const active = (to: string) => pathname.startsWith(to);

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
        <Suspense
          fallback={
            <Center h={200}>
              <Loader />
            </Center>
          }
        >
          <Routes>
            <Route path="/" element={<Navigate to="/games" replace />} />
            <Route path="/games" element={<Games />} />
            <Route path="/games/:gameId" element={<GameDetail />} />
            <Route path="/players" element={<Players />} />
            <Route path="/players/:playerId" element={<PlayerDetail />} />
            <Route path="/rooms" element={<Rooms />} />
            <Route path="/analytics" element={<Analytics />} />
            <Route path="/infrastructure" element={<Overview />} />
            <Route path="*" element={<Navigate to="/games" replace />} />
          </Routes>
        </Suspense>
      </AppShell.Main>
    </AppShell>
  );
}

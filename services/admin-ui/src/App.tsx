import { AppShell, Badge, Box, Burger, Center, Group, Loader, NavLink, Text, Title } from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import { lazy, Suspense, useEffect } from "react";
import { Link, Navigate, Route, Routes, useLocation } from "react-router-dom";
import {
  IconChartBar,
  IconDeviceGamepad2,
  IconDoorEnter,
  IconServer,
  IconServer2,
  IconUsers,
} from "@tabler/icons-react";
import { api } from "./lib/api";
import { useAsync } from "./lib/useAsync";

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
const Fleet = lazy(() => import("./pages/Fleet").then((m) => ({ default: m.Fleet })));

const NAV = [
  { to: "/games", label: "Games", icon: IconDeviceGamepad2 },
  { to: "/players", label: "Players", icon: IconUsers },
  { to: "/rooms", label: "Rooms", icon: IconDoorEnter },
  { to: "/analytics", label: "Analytics", icon: IconChartBar },
  // Fleet (spec 059): conceito de produto/jogo (game servers), diferente da
  // Infrastructure abaixo (saúde da plataforma) — ícone distinto de propósito.
  { to: "/fleet", label: "Fleet", icon: IconServer2 },
  { to: "/infrastructure", label: "Infrastructure", icon: IconServer },
];

const HEALTH_POLL_MS = 10000;

// HealthStrip (assinatura do redesign — POC, ver conversa): faixa no topo do
// header cuja cor reflete a saúde agregada da topologia real (mesmo endpoint
// que o Overview usa) — não é decoração, é o console "respirando" com o que
// ele está operando. nominal = todo serviço up; critical = algum fora;
// cinza = Prometheus indisponível (sem dado pra avaliar).
function HealthStrip() {
  const { data, reload } = useAsync((s) => api.topology(s), []);

  useEffect(() => {
    const id = setInterval(reload, HEALTH_POLL_MS);
    return () => clearInterval(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const color =
    !data || !data.sourceAvailable
      ? "var(--mantine-color-dark-4)"
      : data.nodes.some((n) => n.up === false)
        ? "var(--mantine-color-critical-5)"
        : "var(--mantine-color-nominal-5)";

  return <Box h={3} style={{ background: color, transition: "background-color 600ms ease" }} />;
}

export function App() {
  const { pathname } = useLocation();
  const active = (to: string) => pathname.startsWith(to);
  // Nav mobile (breakpoint "sm"): o AppShell só entende o breakpoint quando
  // este estado + o Burger existem — sem isso o navbar fica sempre fixo por
  // cima do conteúdo abaixo de "sm" (achado da revisão de layout, 2026-07-10).
  const [navOpened, { toggle: toggleNav, close: closeNav }] = useDisclosure(false);

  return (
    <AppShell
      header={{ height: 59 }}
      navbar={{ width: 230, breakpoint: "sm", collapsed: { mobile: !navOpened } }}
      padding="md"
    >
      <AppShell.Header>
        <HealthStrip />
        {/* Marca do console (025): ecoa o nome do repo (go-multiplayer-control-plane). */}
        <Group h="calc(100% - 3px)" px="md" justify="space-between">
          <Group gap="sm">
            <Burger opened={navOpened} onClick={toggleNav} hiddenFrom="sm" size="sm" />
            <Group gap={6} align="baseline">
              <Title order={4}>Multiplayer Control Plane</Title>
              <Text size="sm" c="dimmed">
                Console
              </Text>
            </Group>
          </Group>
          <Badge variant="light" color="gray">
            local
          </Badge>
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
            onClick={closeNav}
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
            <Route path="/fleet" element={<Fleet />} />
            <Route path="/infrastructure" element={<Overview />} />
            <Route path="*" element={<Navigate to="/games" replace />} />
          </Routes>
        </Suspense>
      </AppShell.Main>
    </AppShell>
  );
}

import {
  Alert,
  Anchor,
  Button,
  Code,
  Group,
  Loader,
  Paper,
  SimpleGrid,
  Stack,
  Text,
  Title,
  Tooltip,
} from "@mantine/core";
import {
  IconChartBar,
  IconCloudComputing,
  IconDatabase,
  IconDeviceGamepad2,
  IconExternalLink,
  IconLayoutDashboard,
  IconMedal,
  IconRefresh,
  IconServer,
  IconServer2,
  IconShieldLock,
  IconTrophy,
  IconUsersGroup,
  type Icon,
} from "@tabler/icons-react";
import { api, type FanoutHealth, type LiveMetrics, type ServiceNode, type Topology } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { usePolling } from "../lib/usePolling";
import { useSettings } from "../lib/settings";
import { StatusBadge } from "../components/StatusBadge";
import { StatCard } from "../components/StatCard";
import { ChannelStrip } from "../components/ChannelStrip";

// SERVICE_META: ícone + equivalente AWS de cada serviço, citado do mapa
// canônico (docs/ARCHITECTURE.md §3, ADR-0008) — decisão documentada, não dado
// vivo. O "compute alvo" e a aba de custos que existiam aqui saíram na revisão
// 2026-09 (decisão D4): eram ilustrativos, sem deploy nem medição por trás; a
// comparação de custo volta quando houver números medidos (ADR-0008).
const SERVICE_META: Record<string, { icon: Icon; aws: string }> = {
  "identity-service": { icon: IconShieldLock, aws: "GameKit Identity & Auth" },
  "achievements-service": { icon: IconTrophy, aws: "GameKit Achievements" },
  "player-data-service": { icon: IconDatabase, aws: "GameKit User Gameplay Data" },
  "cloud-save-service": { icon: IconCloudComputing, aws: "GameKit Game State Saving" },
  "leaderboard-service": { icon: IconMedal, aws: "ref. arch. DynamoDB/Redis" },
  "analytics-service": { icon: IconChartBar, aws: "Game Analytics Pipeline" },
  "matchmaking-service": { icon: IconUsersGroup, aws: "GameLift FlexMatch" },
  "session-service": { icon: IconServer2, aws: "GameLift (orquestração)" },
  "game-registry": { icon: IconDeviceGamepad2, aws: "— (interno ao control plane)" },
  "admin-bff": { icon: IconLayoutDashboard, aws: "painel de operação (ADR-0016)" },
};

// Dashboard "Service Overview (RED)" do Grafana provisionado
// (infra/docker/grafana/.../service-overview.json, uid gb-service-overview),
// filtrado pelo job do serviço. VITE_GRAFANA_URL sobrescreve o host.
const GRAFANA_URL = (import.meta.env.VITE_GRAFANA_URL as string | undefined) ?? "http://localhost:3000";
const grafanaServiceURL = (job: string) =>
  `${GRAFANA_URL}/d/gb-service-overview?var-job=${encodeURIComponent(job)}`;

// Infrastructure (spec 081, simplificada na revisão 2026-09): saúde da
// PLATAFORMA (serviços Go), só dado real — status e dependências vindos do
// Prometheus via admin-bff, sinais do matchmaking e do fan-out, e um link para
// o dashboard RED de cada serviço no Grafana.
export function Overview() {
  const { data, loading, error, reload } = useAsync((s) => api.topology(s), []);
  const live = useAsync((s) => api.liveMetrics(s), []);
  const fanout = useAsync((s) => api.fanoutHealth(s), []);
  const { pollingMs } = useSettings();
  usePolling(() => {
    live.reload();
    fanout.reload();
  }, pollingMs);

  const upCount = data ? data.nodes.filter((n) => n.up).length : 0;

  return (
    <Stack>
      <Group justify="space-between" align="flex-end">
        <Title order={3}>
          Infrastructure{" "}
          {data && (
            <Text span c="dimmed">
              · {upCount}/{data.nodes.length} serviços saudáveis
            </Text>
          )}
        </Title>
        <Button
          variant="default"
          leftSection={<IconRefresh size={16} />}
          onClick={() => {
            reload();
            live.reload();
            fanout.reload();
          }}
        >
          Atualizar
        </Button>
      </Group>

      {loading && !data && <Loader />}
      {!!error && !data && <Alert color="critical">Painel indisponível (admin-bff fora?).</Alert>}

      {data && (
        <Stack gap="xl">
          {!data.sourceAvailable && (
            <Alert color="yellow">Prometheus indisponível — status dos serviços desconhecido.</Alert>
          )}

          <Stack gap="sm">
            <ChannelStrip icon={IconServer} label="Serviços" readout={`${upCount}/${data.nodes.length} up`} />
            <SimpleGrid cols={{ base: 1, sm: 2, lg: 3 }} spacing="md">
              {data.nodes.map((n) => (
                <ServiceCard key={n.id} node={n} topology={data} />
              ))}
            </SimpleGrid>
          </Stack>

          <Stack gap="sm">
            <ChannelStrip
              icon={IconUsersGroup}
              label="Matchmaking"
              readout={live.data ? `fila: ${live.data.queueSize ?? "—"}` : undefined}
            />
            <Text size="sm" c="dimmed" mt={-6}>
              Começo do gameplay loop: fila → match → sessão. Sinais do Prometheus.
            </Text>
            {live.error && !live.data ? (
              <Alert color="yellow">Métricas do matchmaking indisponíveis (Prometheus fora?).</Alert>
            ) : (
              <MatchmakingCards m={live.data} />
            )}
          </Stack>

          <Stack gap="sm">
            <ChannelStrip
              icon={IconLayoutDashboard}
              label="Fan-out / entrega"
              readout={fanout.data ? `${fmtMetric(fanout.data.publishOk)} publicadas` : undefined}
            />
            <Text size="sm" c="dimmed" mt={-6}>
              Entrega do resultado da partida (SNS→SQS, ADR-0020). Sinais do Prometheus.
            </Text>
            {fanout.error && !fanout.data ? (
              <Alert color="yellow">Métricas do fan-out indisponíveis (Prometheus fora?).</Alert>
            ) : (
              <FanoutCards health={fanout.data} />
            )}
          </Stack>
        </Stack>
      )}
    </Stack>
  );
}

const fmtMetric = (v: number | null | undefined) => (v == null ? "—" : String(v));
const isBad = (v: number | null | undefined) => v != null && v > 0;

function MatchmakingCards({ m }: { m: LiveMetrics | undefined }) {
  return (
    <SimpleGrid cols={{ base: 2, sm: 4 }}>
      <StatCard label="Fila (tickets)" value={fmtMetric(m?.queueSize)} />
      <StatCard label="Sessões ativas" value={fmtMetric(m?.activeSessions)} />
      <StatCard label="Matches formados" value={fmtMetric(m?.matchesTotal)} />
      <StatCard label="Matches com falha" value={fmtMetric(m?.matchesFailed)} bad={isBad(m?.matchesFailed)} />
    </SimpleGrid>
  );
}

function FanoutCards({ health }: { health: FanoutHealth | undefined }) {
  return (
    <SimpleGrid cols={{ base: 2, sm: 3, md: 5 }}>
      <StatCard label="Publicadas (ok)" value={fmtMetric(health?.publishOk)} />
      <StatCard label="Publicações c/ erro" value={fmtMetric(health?.publishError)} bad={isBad(health?.publishError)} />
      <StatCard label="Consumidas (ok)" value={fmtMetric(health?.consumeOk)} />
      <StatCard label="Duplicadas (no-op)" value={fmtMetric(health?.consumeDup)} />
      <StatCard label="Consumos c/ erro" value={fmtMetric(health?.consumeError)} bad={isBad(health?.consumeError)} />
    </SimpleGrid>
  );
}

// ServiceCard: saúde real (Prometheus) + dependências reais (edges do
// topology) + atalho para o dashboard RED do serviço no Grafana.
function ServiceCard({ node, topology }: { node: ServiceNode; topology: Topology }) {
  const meta = SERVICE_META[node.id];
  const Icon = meta?.icon ?? IconServer;
  const dependsOn = topology.edges.filter((e) => e.from === node.id);

  return (
    <Paper withBorder p="md">
      <Group justify="space-between" mb={6} wrap="nowrap">
        <Group gap="sm" wrap="nowrap">
          <Icon size={20} />
          <Text fw={600} size="sm">
            {node.label}
          </Text>
        </Group>
        <StatusBadge up={node.up} />
      </Group>
      <Text size="xs" c="dimmed" ff="monospace" mb={dependsOn.length > 0 ? 6 : 0}>
        {node.id}
      </Text>
      {dependsOn.length > 0 && (
        <Group gap={4} mb="sm">
          <Text size="xs" c="dimmed">
            depende de:
          </Text>
          {dependsOn.map((e) => (
            <Tooltip key={`${e.from}-${e.to}`} label={e.label}>
              <Code>{e.to}</Code>
            </Tooltip>
          ))}
        </Group>
      )}

      <Group justify="space-between" gap="xs" wrap="nowrap">
        <Text size="xs" c="dimmed" truncate>
          {meta ? `AWS: ${meta.aws}` : ""}
        </Text>
        <Anchor href={grafanaServiceURL(node.id)} target="_blank" rel="noreferrer" size="xs">
          <Group gap={2} wrap="nowrap">
            Grafana <IconExternalLink size={12} />
          </Group>
        </Anchor>
      </Group>
    </Paper>
  );
}

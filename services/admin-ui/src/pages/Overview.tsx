import {
  Alert,
  Box,
  Button,
  Code,
  Divider,
  Group,
  Loader,
  Paper,
  SimpleGrid,
  Stack,
  Table,
  Tabs,
  Text,
  Title,
  Tooltip,
} from "@mantine/core";
import {
  IconChartBar,
  IconCloudComputing,
  IconCurrencyDollar,
  IconDatabase,
  IconDeviceGamepad2,
  IconLayoutDashboard,
  IconLayoutGrid,
  IconMedal,
  IconRefresh,
  IconScale,
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
import { MockZone } from "../components/MockZone";

// SERVICE_META (spec 081): mapeamento serviço -> equivalente AWS + papel,
// citado direto do mapa canônico de serviços (docs/ARCHITECTURE.md §3) — dado
// REAL (decisão arquitetural documentada, não invenção), só que sem nenhum
// endpoint que o exponha hoje (é um mapa hardcoded no frontend, por isso
// ainda leva o tratamento visual de MockZone — mesma régua da Fleet: "mock"
// significa "sem API viva por trás hoje", não "informação falsa").
// `compute` é ILUSTRATIVO — guiado pelo framework real do ADR-0005 (menu
// Lambda/Fargate/EC2/EKS por perfil de carga), mas a escolha final por
// serviço ainda está "diferida" (ninguém commitou isso em produção; Fase 11
// do ROADMAP — comparação Go vs AWS — segue pendente do primeiro deploy).
const SERVICE_META: Record<string, { icon: Icon; aws: string; compute: string }> = {
  "identity-service": { icon: IconShieldLock, aws: "GameKit Identity & Auth", compute: "Lambda" },
  "achievements-service": { icon: IconTrophy, aws: "GameKit Achievements", compute: "ECS Fargate" },
  "player-data-service": { icon: IconDatabase, aws: "GameKit User Gameplay Data", compute: "Lambda" },
  "cloud-save-service": { icon: IconCloudComputing, aws: "GameKit Game State Saving", compute: "Lambda" },
  "leaderboard-service": { icon: IconMedal, aws: "ref. arch. DynamoDB/Redis", compute: "ECS Fargate" },
  "analytics-service": { icon: IconChartBar, aws: "Game Analytics Pipeline", compute: "ECS Fargate" },
  "matchmaking-service": { icon: IconUsersGroup, aws: "GameLift FlexMatch", compute: "ECS Fargate" },
  "session-service": { icon: IconServer2, aws: "GameLift (orquestração)", compute: "ECS Fargate" },
  "game-registry": { icon: IconDeviceGamepad2, aws: "— (interno ao control plane)", compute: "Lambda" },
  "admin-bff": { icon: IconLayoutDashboard, aws: "painel de operação (ADR-0016)", compute: "Lambda" },
};

// Infrastructure (redesign spec 081): saúde da PLATAFORMA (serviços Go) — o
// canvas de topologia (TopologyGraph) e a tabela "Saúde dos serviços" viraram
// um grid de cards por serviço (a mesma informação, sem a duplicação
// canvas+tabela mostrando a mesma coisa duas vezes). "Visão geral" é dado
// REAL (Prometheus, via admin-bff); "Comparação AWS" é a tese do projeto
// (ADR-0008) — mockada, mas com números reais do ROADMAP.md onde existem.
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
        <Tabs defaultValue="overview">
          <Tabs.List>
            <Tabs.Tab value="overview" leftSection={<IconLayoutGrid size={16} />}>
              Visão geral
            </Tabs.Tab>
            <Tabs.Tab value="aws" leftSection={<IconScale size={16} />}>
              Comparação AWS
            </Tabs.Tab>
          </Tabs.List>

          <Tabs.Panel value="overview" pt="md">
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
          </Tabs.Panel>

          <Tabs.Panel value="aws" pt="md">
            <AwsComparisonTab topology={data} />
          </Tabs.Panel>
        </Tabs>
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
// topology) em destaque; equivalente AWS + compute alvo numa MockZone —
// substitui o canvas (TopologyGraph) + a tabela "Saúde dos serviços", que
// mostravam a mesma informação de saúde de dois jeitos redundantes.
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

      {meta && (
        <MockZone label="Equivalente AWS">
          <Text size="sm" mb="xs">
            {meta.aws}
          </Text>
          <Group gap={6}>
            <Text size="xs" c="dimmed">
              Compute alvo:
            </Text>
            <Code>{meta.compute}</Code>
          </Group>
        </MockZone>
      )}
    </Paper>
  );
}

// AwsComparisonTab (spec 081): a tese do projeto (ADR-0008) materializada —
// "Fase 11" do ROADMAP.md ("Comparação Go vs. AWS") ainda está pendente (só
// acontece depois do primeiro deploy prod), então esta aba adianta o que
// essa comparação vai mostrar. Números com fonte citada (ROADMAP.md) NÃO
// levam o badge "mock" — são estimativas reais já documentadas, diferente do
// resto da aba, que é ilustrativo.
function AwsComparisonTab({ topology }: { topology: Topology }) {
  return (
    <Stack gap="xl">
      <ChannelStrip icon={IconScale} label="Comparação Go × AWS" readout="Fase 11 · pendente (ROADMAP.md)" />
      <Text size="sm" c="dimmed" mt={-6} maw={760}>
        Tese do projeto (ADR-0008): reimplementar em Go, sobre compute genérico barato, os serviços que a AWS
        vende prontos (GameKit/GameLift) — e comparar custo/performance com a oferta gerenciada. A comparação de
        verdade (latência, throughput, custo medido) só acontece depois do primeiro deploy em produção; por ora,
        esta aba é a prévia de como ela vai se parecer.
      </Text>

      <SimpleGrid cols={{ base: 1, md: 2 }} spacing="md">
        <Paper withBorder p="md">
          <Text size="xs" c="dimmed" fw={600} tt="uppercase" style={{ letterSpacing: "0.04em" }} mb="xs">
            Ambiente atual
          </Text>
          <Group gap="xs" mb="sm">
            <StatusBadge up={true} />
            <Text fw={600}>Local — Docker Compose + LocalStack</Text>
          </Group>
          <Text size="sm" c="dimmed">
            9 serviços Go + Postgres/Redis/MinIO em containers, custo de infra <b>$0</b> (hardware próprio) —
            mesmo ambiente que gera as capturas de tela deste console.
          </Text>
        </Paper>

        <MockZone label="Ambiente de produção (alvo)">
          <Group gap="xs" mb="sm">
            <Text fw={600}>AWS — Terraform (ADR-0005)</Text>
          </Group>
          <Text size="sm" c="dimmed">
            Compute por serviço via o menu do ADR-0005 (Lambda/ECS Fargate/EC2/EKS, ver cada card na Visão
            geral) — nenhum serviço tem a escolha final commitada em produção ainda; Fase 3 do ROADMAP marca
            "prod AWS = backlog".
          </Text>
        </MockZone>
      </SimpleGrid>

      <Divider label="Estimativa de custo" labelPosition="left" />

      <SimpleGrid cols={{ base: 1, md: 2 }} spacing="md">
        <Paper withBorder p="md">
          <Group gap="sm" mb="xs">
            <IconCurrencyDollar size={20} />
            <Text fw={600}>Go (control plane próprio)</Text>
          </Group>
          <Text size="xl" fw={700} ff="monospace" c="nominal">
            $20–37/mês
          </Text>
          <Text size="xs" c="dimmed" mt={4}>
            Estimativa do ROADMAP.md (FinOps) — budget via Terraform, alerta em 80% de $40/mês. Serviços
            stateless em Lambda tendem a puxar o custo pra baixo (free tier + escala a zero).
          </Text>
        </Paper>

        <MockZone label="AWS gerenciado (equivalente)">
          <Text size="xl" fw={700} ff="monospace">
            $150–400+/mês
          </Text>
          <Text size="xs" c="dimmed" mt={4}>
            Ordem de grandeza ilustrativa: GameLift cobra por hora de instância mesmo ocioso (~365h/mês com
            autoscaling) + FlexMatch por ticket de matchmaking — bem diferente de um serviço Go que escala a
            zero (Lambda) ou roda numa instância pequena compartilhada. Sem cotação real feita ainda.
          </Text>
        </MockZone>
      </SimpleGrid>

      <Divider label="Mapa de serviços (ADR-0008/ARCHITECTURE.md, canônico)" labelPosition="left" />

      <Table striped withTableBorder verticalSpacing="sm">
        <Table.Thead>
          <Table.Tr>
            <Table.Th>Serviço</Table.Th>
            <Table.Th>Status</Table.Th>
            <Table.Th>Equivalente AWS</Table.Th>
            <Table.Th>Compute alvo</Table.Th>
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {topology.nodes.map((n) => {
            const meta = SERVICE_META[n.id];
            return (
              <Table.Tr key={n.id}>
                <Table.Td>
                  <Text size="sm" fw={500}>
                    {n.label}
                  </Text>
                  <Text size="xs" c="dimmed" ff="monospace">
                    {n.id}
                  </Text>
                </Table.Td>
                <Table.Td>
                  <StatusBadge up={n.up} />
                </Table.Td>
                <Table.Td>
                  <Text size="sm" c="dimmed">
                    {meta?.aws ?? "—"}
                  </Text>
                </Table.Td>
                <Table.Td>{meta && <Code>{meta.compute}</Code>}</Table.Td>
              </Table.Tr>
            );
          })}
        </Table.Tbody>
      </Table>

      <Divider label="Evoluções futuras (documentadas, não implementadas)" labelPosition="left" />
      <Box component="ul" pl="lg" style={{ margin: 0 }}>
        <Text component="li" size="sm" c="dimmed">
          Badge de ambiente no header (hoje fixo em "local", `App.tsx`) passa a refletir o ambiente real assim
          que houver um deploy prod — provavelmente lido do mesmo lugar que já resolve a localização da Fleet
          (spec 023), generalizado pra plataforma inteira.
        </Text>
        <Text component="li" size="sm" c="dimmed">
          Custo real via AWS Cost Explorer API (por serviço, tag-based) substituindo a estimativa do
          ROADMAP.md — só faz sentido depois do primeiro deploy prod (Fase 11).
        </Text>
        <Text component="li" size="sm" c="dimmed">
          Link direto pro dashboard Grafana de cada serviço (Service Overview RED, já provisionado —
          OBSERVABILITY.md) a partir do card — hoje o link é só "olha no Grafana", sem deep-link parametrizado
          por `$job`.
        </Text>
      </Box>
    </Stack>
  );
}

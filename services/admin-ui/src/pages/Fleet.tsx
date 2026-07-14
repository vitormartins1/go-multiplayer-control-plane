import {
  Alert,
  Badge,
  Box,
  Button,
  Code,
  Grid,
  Group,
  Loader,
  Paper,
  Select,
  SimpleGrid,
  Stack,
  Table,
  Tabs,
  Text,
  TextInput,
  Title,
  Tooltip,
} from "@mantine/core";
import {
  IconAlertTriangle,
  IconBrandApple,
  IconCloud,
  IconCpu,
  IconLayoutGrid,
  IconServer2,
  IconSettings,
  IconTag,
} from "@tabler/icons-react";
import { useState } from "react";
import { api, type Game } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { GameCover } from "../components/GameCover";
import { FleetLocationBadge } from "../components/FleetLocationBadge";
import { ChannelStrip } from "../components/ChannelStrip";
import { fleetLocationOf } from "../lib/fleetLocation";

// mockFleetStatus (Fase 0, spec 059; enriquecido na spec 077 — mesma natureza
// ilustrativa, mais fiel ao que a Fatia 2 do ADR-0029 vai expor de verdade:
// imageRepository no GHCR + tags publicadas). Determinístico por nome de
// fleet, sem chamada de rede — nenhum dos campos abaixo existe hoje em
// lugar nenhum do backend.
interface MockVersion {
  tag: string;
  current: boolean;
  daysAgo: number;
}

interface MockFleetStatus {
  imageRef: string;
  currentTag: string;
  healthy: number;
  total: number;
  versions: MockVersion[];
}

function mockFleetStatus(fleetName: string): MockFleetStatus {
  const seed = [...fleetName].reduce((h, c) => (h * 31 + c.charCodeAt(0)) >>> 0, 7);
  const minor = 3 + (seed % 3);
  const patch = seed % 5;
  const currentTag = `v0.${minor}.${patch}`;
  const total = 2 + (seed % 3);
  return {
    imageRef: `ghcr.io/vitormartins1/${fleetName}-server`,
    currentTag,
    healthy: total,
    total,
    versions: [
      { tag: `v0.${minor + 1}.0-rc1`, current: false, daysAgo: 0 },
      { tag: `v0.${minor}.${patch + 1}`, current: false, daysAgo: 2 },
      { tag: currentTag, current: true, daysAgo: 6 + (seed % 5) },
    ],
  };
}

function agoLabel(daysAgo: number): string {
  return daysAgo === 0 ? "hoje" : `há ${daysAgo}d`;
}

// mockFleetHealth (spec 079): breakdown por estado do GameServer Agones —
// nomenclatura conferida na documentação real do Agones (agones.dev/site/
// docs/reference/fleet, .../guides/health-checking):
//   FleetStatus real tem Replicas (atual), ReadyReplicas, ReservedReplicas,
//   AllocatedReplicas — "Desired" vem do Fleet.spec.replicas, não do status.
//   Um GameServer individual passa por Scheduled → RequestReady → Ready, e
//   dali pode ir a Allocated (jogadores conectados) ou Reserved (não é
//   removido em scale-down, mas também não conta pro autoscaler) — ou cair
//   em Unhealthy se o container morrer enquanto Ready/Allocated/Reserved
//   (falha de health check). Nenhum desses campos existe em endpoint
//   nenhum hoje (FleetStatus real, spec 023, só tem o agregado da Fleet
//   inteira: cluster/fleetExists/desired/current/ready/allocated — sem
//   reserved nem unhealthy). Determinístico por fleet, mesmo espírito do
//   resto do mock desta página.
interface MockFleetHealth {
  desired: number;
  current: number;
  ready: number;
  allocated: number;
  reserved: number;
  unhealthy: number;
}

function mockFleetHealth(fleetName: string): MockFleetHealth {
  const seed = [...fleetName].reduce((h, c) => (h * 17 + c.charCodeAt(0)) >>> 0, 11);
  const desired = 2 + (seed % 3); // 2..4
  const allocated = seed % (desired + 1);
  // >>> (sem sinal), não >> — achado: com >>, um seed acima de 2^31 vira
  // negativo (ToInt32 reinterpreta o bit alto como sinal) e "% 2" preserva
  // o sinal do dividendo em JS, gerando "Reserved: -1" (visto ao vivo com
  // a fleet "packet-arena").
  const reserved = (seed >>> 2) % 2;
  const ready = Math.max(0, desired - allocated - reserved);
  return {
    desired,
    current: ready + allocated + reserved,
    ready,
    allocated,
    reserved,
    // Unhealthy é transiente (o Fleet controller recicla o GameServer) —
    // raro no estado estável, por isso 0 na maioria das seeds.
    unhealthy: seed % 5 === 0 ? 1 : 0,
  };
}

// mockAgonesClusters (spec 078): perfis dos 2 clusters reais da infra
// (ADR-0029 — k3d local no Mac + k3s no Raspberry Pi 5 home lab), mas os
// CAMPOS abaixo (nós, versões) são ilustrativos — não existe hoje nenhum
// endpoint que liste clusters conhecidos ou sua versão do Kubernetes/Agones,
// só o status de UMA fleet específica num cluster específico (fleet-status,
// spec 023). Por isso esta seção não tem badge de "status ao vivo" nenhum
// (ready/unreachable etc.) — só cadastro, pra não repetir a confusão real-
// vs-mock que a Visão geral já teve que corrigir.
const MOCK_AGONES_CLUSTERS = [
  { cluster: "local", label: "Local (Mac)", icon: IconBrandApple, runtime: "k3d", nodes: 1, k8s: "v1.29.4", agones: "v1.41.0" },
  { cluster: "pi5", label: "Pi 5 (Home Lab)", icon: IconCpu, runtime: "k3s", nodes: 1, k8s: "v1.29.1", agones: "v1.41.0" },
] as const;

// Fleet (spec 059 + redesign spec 077): cadastro real da Fleet Agones por jogo
// (ADR-0026) + status real de localização/réplicas (spec 023) + esqueleto
// mockado da evolução futura (ADR-0029 fatia 2 — imageRepository no GHCR,
// tags publicadas, promoção manual). "Visão geral" destaca o que está rodando
// agora; "Configuração" é onde se cadastra o nome real da fleet (e, no
// futuro, o repositório de imagem).
export function Fleet() {
  const { data: games, loading, error, reload } = useAsync((s) => api.games(s), []);
  const fleetGames = (games ?? []).filter((g) => g.fleet);
  const otherGames = (games ?? []).filter((g) => !g.fleet);

  return (
    <Stack>
      {/* Title order={3} (correção, 2026-07-13): ChannelStrip é o divisor de
          SEÇÃO dentro de uma página (Analytics, Leaderboards, e agora a
          própria "Configuração" desta página) — usá-lo como título da
          página confundia os dois papéis. Título de página segue a mesma
          convenção de Games/Players/Analytics. */}
      <Title order={3}>
        Fleet{" "}
        {games && (
          <Text span c="dimmed">
            · {fleetGames.length} fleet{fleetGames.length === 1 ? "" : "s"} cadastrada
            {fleetGames.length === 1 ? "" : "s"}
          </Text>
        )}
      </Title>

      {loading && !games && <Loader />}
      {!!error && !games && <Alert color="critical">admin-bff indisponível.</Alert>}

      {games && (
        <Tabs defaultValue="overview">
          <Tabs.List>
            <Tabs.Tab value="overview" leftSection={<IconLayoutGrid size={16} />}>
              Visão geral
            </Tabs.Tab>
            <Tabs.Tab value="config" leftSection={<IconSettings size={16} />}>
              Configuração
            </Tabs.Tab>
          </Tabs.List>

          <Tabs.Panel value="overview" pt="md">
            <Stack>
              <Alert color="yellow" icon={<IconAlertTriangle size={16} />}>
                Imagem e versões (zona tracejada de cada card) são <b>ilustrativas</b> — a
                integração real com um registry de imagens ainda não existe (ADR-0029, fatia 2).
                Localização e réplicas já são dados reais do cluster Agones.
              </Alert>

              {fleetGames.length === 0 && (
                <Text c="dimmed">
                  Nenhum jogo tem fleet cadastrada ainda — use a aba Configuração.
                </Text>
              )}

              {fleetGames.length > 0 && (
                <SimpleGrid cols={{ base: 1, md: 2 }} spacing="md">
                  {fleetGames.map((g) => (
                    <FleetOverviewCard key={g.id} game={g} />
                  ))}
                </SimpleGrid>
              )}

              {otherGames.length > 0 && (
                <Text size="sm" c="dimmed">
                  + {otherGames.length} jogo{otherGames.length === 1 ? "" : "s"} sem fleet
                  configurada — ver aba Configuração.
                </Text>
              )}
            </Stack>
          </Tabs.Panel>

          <Tabs.Panel value="config" pt="md">
            <Stack gap="xl">
              <Stack gap="sm">
                <ChannelStrip
                  icon={IconCloud}
                  label="Clusters Agones"
                  readout={`${MOCK_AGONES_CLUSTERS.length} clusters (mock)`}
                />
                <Text size="sm" c="dimmed" mt={-6}>
                  Perfis de cluster ilustrativos — os dois clusters (local e Pi 5) são reais
                  (ADR-0029), mas não existe hoje um endpoint que liste clusters conhecidos ou
                  sua versão do Kubernetes/Agones.
                </Text>
                <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="md">
                  {MOCK_AGONES_CLUSTERS.map((c) => (
                    <ClusterCard key={c.cluster} cluster={c} />
                  ))}
                </SimpleGrid>
              </Stack>

              <Stack gap="sm">
                <ChannelStrip
                  icon={IconServer2}
                  label="Jogos"
                  readout={`${games.filter((g) => g.fleet).length}/${games.length} com fleet`}
                />
                <FleetConfigTable games={games} onSaved={reload} />
              </Stack>
            </Stack>
          </Tabs.Panel>
        </Tabs>
      )}
    </Stack>
  );
}

// FleetOverviewCard: um card por jogo com fleet cadastrada — telemetria real
// (localização + réplicas) em destaque, zona mockada (imagem/versões/promoção)
// visualmente separada por borda tracejada, pra nunca ser confundida com dado
// real (achado revisando a v1: o badge "OFFLINE" real e o "saudáveis (mock)"
// ficavam lado a lado sem nenhuma distinção visual).
function FleetOverviewCard({ game }: { game: Game }) {
  const mock = mockFleetStatus(game.fleet!);
  const health = mockFleetHealth(game.fleet!);
  const { data: status, loading, error } = useAsync((s) => api.fleetStatus(game.id, s), [game.id]);

  return (
    <Paper withBorder p="md">
      <Group justify="space-between" mb="sm" wrap="nowrap">
        <Group gap="sm" wrap="nowrap">
          <GameCover game={game} variant="mini" />
          <Text fw={600}>{game.name}</Text>
        </Group>
        <Badge variant="light" ff="monospace">
          {game.fleet}
        </Badge>
      </Group>

      <Group gap="xl" wrap="wrap">
        <Stack gap={2}>
          <Text size="xs" c="dimmed" fw={600} tt="uppercase" style={{ letterSpacing: "0.04em" }}>
            Onde está rodando
          </Text>
          {loading && <Loader size="xs" />}
          {!loading && !!error && <FleetLocationBadge location="unknown" />}
          {!loading && !error && status && <FleetLocationBadge location={fleetLocationOf(status)} />}
        </Stack>
        {!loading && !error && status?.fleetExists && (
          <Stack gap={2}>
            <Text size="xs" c="dimmed" fw={600} tt="uppercase" style={{ letterSpacing: "0.04em" }}>
              Réplicas
            </Text>
            <Text size="sm" ff="monospace">
              {status.ready}/{status.desired} ready
              {status.allocated > 0 && ` · ${status.allocated} allocated`}
            </Text>
          </Stack>
        )}
      </Group>

      {/* zona mockada — borda tracejada + fundo levemente diferente marcam
          visualmente "isto não é dado real", reforçando o texto "(mock)".
          Grid 5/7 (~40/60, pedido do usuário): Saúde à esquerda, mais
          compacta (poucos números); Imagem & versões à direita, precisa de
          mais largura pra a referência de imagem + tags não truncarem tanto.
          Empilha em telas estreitas (base=12). */}
      <Grid mt="md" gutter="sm">
        <Grid.Col span={{ base: 12, md: 5 }}>
          <Box
            p="sm"
            h="100%"
            style={{
              border: "1px dashed var(--mantine-color-dark-4)",
              borderRadius: 8,
              background: "var(--mantine-color-dark-8)",
            }}
          >
            <Group justify="space-between" mb={6}>
              <Text size="xs" c="dimmed" fw={600} tt="uppercase" style={{ letterSpacing: "0.04em" }}>
                Saúde (game servers)
              </Text>
              <Badge size="xs" variant="light" color="yellow">
                mock
              </Badge>
            </Group>

            {/* réplicas atual/desejado — mesma convenção do "READY" do
                kubectl get deployment (2/4): current/desired, não um número
                solto. Desired vem de Fleet.spec.replicas (não do status). */}
            <Group gap={6} mb="xs" wrap="nowrap">
              <Text size="xs" c="dimmed">Réplicas</Text>
              <Text size="sm" fw={600} ff="monospace">
                {health.current}/{health.desired}
              </Text>
            </Group>

            <SimpleGrid cols={2} spacing="xs">
              <Stack gap={0}>
                <Text size="xs" c="dimmed">Ready</Text>
                <Text size="sm" fw={600} c="nominal" ff="monospace">{health.ready}</Text>
              </Stack>
              <Stack gap={0}>
                <Text size="xs" c="dimmed">Allocated</Text>
                <Text size="sm" fw={600} c="signal" ff="monospace">{health.allocated}</Text>
              </Stack>
              <Stack gap={0}>
                <Text size="xs" c="dimmed">Reserved</Text>
                <Text size="sm" fw={600} ff="monospace">{health.reserved}</Text>
              </Stack>
              <Stack gap={0}>
                <Text size="xs" c="dimmed">Unhealthy</Text>
                <Text size="sm" fw={600} c={health.unhealthy > 0 ? "critical" : undefined} ff="monospace">
                  {health.unhealthy}
                </Text>
              </Stack>
            </SimpleGrid>
          </Box>
        </Grid.Col>

        <Grid.Col span={{ base: 12, md: 7 }}>
          <Box
            p="sm"
            h="100%"
            style={{
              border: "1px dashed var(--mantine-color-dark-4)",
              borderRadius: 8,
              background: "var(--mantine-color-dark-8)",
            }}
          >
            <Group justify="space-between" mb={6}>
              <Text size="xs" c="dimmed" fw={600} tt="uppercase" style={{ letterSpacing: "0.04em" }}>
                Imagem &amp; versões
              </Text>
              <Badge size="xs" variant="light" color="yellow">
                mock
              </Badge>
            </Group>

            <Group gap={6} wrap="nowrap" mb="xs">
              <IconTag size={14} color="var(--mantine-color-dimmed)" style={{ flexShrink: 0 }} />
              <Code style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                {mock.imageRef}:{mock.currentTag}
              </Code>
            </Group>

            <Group gap={6} mb="sm">
              {mock.versions.map((v) => (
                <Tooltip key={v.tag} label={`publicada ${agoLabel(v.daysAgo)} (mock)`}>
                  <Badge
                    size="xs"
                    variant={v.current ? "filled" : "outline"}
                    color={v.current ? "nominal" : "gray"}
                    ff="monospace"
                    style={{ cursor: "default" }}
                  >
                    {v.tag}
                  </Badge>
                </Tooltip>
              ))}
            </Group>

            <Group gap={6}>
              <Select
                data={mock.versions.map((v) => v.tag)}
                defaultValue={mock.currentTag}
                w={170}
                size="xs"
              />
              <Tooltip label="Integração com o registry de imagens ainda não implementada (ADR-0029, fatia 2)">
                <Button size="xs" variant="light" disabled>
                  Promover
                </Button>
              </Tooltip>
            </Group>
          </Box>
        </Grid.Col>
      </Grid>
    </Paper>
  );
}

// ClusterCard: perfil ilustrativo de um cluster Agones conhecido (mock, ver
// MOCK_AGONES_CLUSTERS). Mesma borda tracejada + badge "mock" das zonas
// mockadas da Visão geral — vocabulário visual único pra "isto é ilustrativo"
// em toda a página.
//
// EVOLUÇÃO FUTURA (documentada a pedido do usuário, 2026-07-13 — não
// implementar agora): hover no card revela um ícone de lápis ao lado do
// badge "mock"; clicar troca os campos (Runtime/Nós/Kubernetes/Agones) pra
// inputs editáveis inline, no mesmo padrão de TextInput+Salvar já usado em
// FleetConfigRow. Só faz sentido quando existir de fato um endpoint de
// perfis de cluster — hoje `MOCK_AGONES_CLUSTERS` é uma constante hardcoded
// no frontend, sem persistência nenhuma.
function ClusterCard({ cluster: c }: { cluster: (typeof MOCK_AGONES_CLUSTERS)[number] }) {
  const Icon = c.icon;
  return (
    <Box
      p="md"
      style={{
        border: "1px dashed var(--mantine-color-dark-4)",
        borderRadius: 8,
        background: "var(--mantine-color-dark-8)",
      }}
    >
      <Group justify="space-between" mb="sm">
        <Group gap="sm">
          <Icon size={20} />
          <Text fw={600}>{c.label}</Text>
        </Group>
        <Badge size="xs" variant="light" color="yellow">
          mock
        </Badge>
      </Group>
      <SimpleGrid cols={2} spacing="xs">
        <Stack gap={0}>
          <Text size="xs" c="dimmed">Runtime</Text>
          <Code>{c.runtime}</Code>
        </Stack>
        <Stack gap={0}>
          <Text size="xs" c="dimmed">Nós</Text>
          <Text size="sm" ff="monospace">{c.nodes}</Text>
        </Stack>
        <Stack gap={0}>
          <Text size="xs" c="dimmed">Kubernetes</Text>
          <Text size="sm" ff="monospace">{c.k8s}</Text>
        </Stack>
        <Stack gap={0}>
          <Text size="xs" c="dimmed">Agones</Text>
          <Text size="sm" ff="monospace">{c.agones}</Text>
        </Stack>
      </SimpleGrid>
    </Box>
  );
}

// FleetConfigTable: cadastro real do nome da fleet (todos os jogos, com ou
// sem fleet ainda) + coluna preparada (desabilitada) pro imageRepository —
// schema que só existe na Fatia 2 do ADR-0029, gated por evidência; a coluna
// já deixa a evolução visível sem implementar nada.
//
// EVOLUÇÃO FUTURA (documentada a pedido do usuário, 2026-07-13 — não
// implementar agora): hoje "cadastrar fleet" é só preencher o TextInput e
// salvar — funciona, mas é uma ação pouco guiada. Uma versão melhor seria
// um fluxo de setup dedicado pra jogo sem fleet: sugerir um nome de fleet a
// partir do slug do jogo, deixar escolher um dos clusters da seção acima
// (hoje só cadastro descritivo, sem capacidade real de "aplicar fleet num
// cluster"), salvar e já destacar/rolar até o card novo na Visão geral. A
// versão atual (badge "sem fleet" + botão "Ativar") é o passo intermediário
// — deixa a ação mais clara sem construir o fluxo guiado completo ainda.
function FleetConfigTable({ games, onSaved }: { games: Game[]; onSaved: () => void }) {
  return (
    <Table verticalSpacing="sm" highlightOnHover>
      <Table.Thead>
        <Table.Tr>
          <Table.Th>Jogo</Table.Th>
          <Table.Th>Fleet Agones</Table.Th>
          <Table.Th>Repositório de imagem</Table.Th>
        </Table.Tr>
      </Table.Thead>
      <Table.Tbody>
        {games.map((g) => (
          <FleetConfigRow key={g.id} game={g} onSaved={onSaved} />
        ))}
      </Table.Tbody>
    </Table>
  );
}

function FleetConfigRow({ game, onSaved }: { game: Game; onSaved: () => void }) {
  const [fleetInput, setFleetInput] = useState(game.fleet ?? "");
  const [saving, setSaving] = useState(false);
  const dirty = fleetInput.trim() !== (game.fleet ?? "");
  const hasFleet = !!game.fleet;

  async function save() {
    setSaving(true);
    try {
      await api.setFleet(game.id, fleetInput.trim());
      onSaved();
    } finally {
      setSaving(false);
    }
  }

  return (
    <Table.Tr>
      <Table.Td>
        <Group gap="sm" wrap="nowrap">
          <GameCover game={game} variant="mini" />
          <Text size="sm" fw={500}>
            {game.name}
          </Text>
          {/* badge + verbo do botão (spec 079): sinaliza que preencher +
              salvar aqui é o que faz o jogo aparecer na Visão geral — sem
              isso a linha não se distinguia de "edição comum". */}
          {!hasFleet && (
            <Badge size="xs" variant="outline" color="gray">
              sem fleet
            </Badge>
          )}
        </Group>
      </Table.Td>
      <Table.Td>
        <Group gap={6} wrap="nowrap">
          <TextInput
            placeholder="packet-arena"
            value={fleetInput}
            onChange={(e) => setFleetInput(e.currentTarget.value)}
            size="xs"
            w={200}
          />
          <Button size="xs" onClick={save} loading={saving} disabled={!dirty}>
            {hasFleet ? "Salvar" : "Ativar"}
          </Button>
        </Group>
      </Table.Td>
      <Table.Td>
        <Tooltip label="Repositório de imagem no GHCR — Fatia 2 do ADR-0029, gated por evidência. Ainda não implementado.">
          <TextInput placeholder="ghcr.io/… (em breve)" size="xs" w={260} disabled />
        </Tooltip>
      </Table.Td>
    </Table.Tr>
  );
}

import {
  Alert,
  Badge,
  Box,
  Button,
  Code,
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
  Tooltip,
} from "@mantine/core";
import {
  IconAlertTriangle,
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
      <ChannelStrip
        icon={IconServer2}
        label="Fleet"
        readout={games ? `${fleetGames.length} fleet${fleetGames.length === 1 ? "" : "s"} cadastrada${fleetGames.length === 1 ? "" : "s"}` : undefined}
      />

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
            <FleetConfigTable games={games} onSaved={reload} />
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
          visualmente "isto não é dado real", reforçando o texto "(mock)". */}
      <Box
        mt="md"
        p="sm"
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
    </Paper>
  );
}

// FleetConfigTable: cadastro real do nome da fleet (todos os jogos, com ou
// sem fleet ainda) + coluna preparada (desabilitada) pro imageRepository —
// schema que só existe na Fatia 2 do ADR-0029, gated por evidência; a coluna
// já deixa a evolução visível sem implementar nada.
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
            Salvar
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

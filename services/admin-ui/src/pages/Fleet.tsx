import {
  Alert,
  Badge,
  Button,
  Group,
  Loader,
  Paper,
  SimpleGrid,
  Stack,
  Table,
  Text,
  TextInput,
  Title,
} from "@mantine/core";
import { IconServer2 } from "@tabler/icons-react";
import { useState, type ReactNode } from "react";
import { api, type Game } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { GameCover } from "../components/GameCover";
import { FleetLocationBadge } from "../components/FleetLocationBadge";
import { ChannelStrip } from "../components/ChannelStrip";
import { fleetLocationOf } from "../lib/fleetLocation";

// Fleet (spec 059, simplificada na revisão 2026-09 — decisão D4): só dado
// real. Um card por jogo com fleet cadastrada, com o status que o
// session-service lê do cluster Agones (spec 023: localização + réplicas), e o
// cadastro do nome da fleet (ADR-0026). As zonas ilustrativas (imagem/versões,
// saúde por GameServer, perfis de cluster) saíram: nenhum desses dados existe
// no backend, e número inventado ao lado de número real desvaloriza os reais.
// Voltam quando a Fatia 2 da ADR-0029 (registry de imagem) existir de fato.
export function Fleet() {
  const { data: games, loading, error, reload } = useAsync((s) => api.games(s), []);
  const fleetGames = (games ?? []).filter((g) => g.fleet);

  return (
    <Stack>
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
        <>
          {fleetGames.length === 0 ? (
            <Text c="dimmed">Nenhum jogo tem fleet cadastrada ainda — cadastre abaixo.</Text>
          ) : (
            <SimpleGrid cols={{ base: 1, md: 2, xl: 3 }} spacing="md">
              {fleetGames.map((g) => (
                <FleetStatusCard key={g.id} game={g} />
              ))}
            </SimpleGrid>
          )}

          <ChannelStrip
            icon={IconServer2}
            label="Cadastro"
            readout={`${fleetGames.length}/${games.length} jogos com fleet`}
          />
          <FleetConfigTable games={games} onSaved={reload} />
        </>
      )}
    </Stack>
  );
}

// FleetStatusCard: status real da fleet do jogo no cluster (spec 023) —
// onde está rodando e réplicas ready/desired/allocated.
function FleetStatusCard({ game }: { game: Game }) {
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
        <Stat label="Onde está rodando">
          {loading && <Loader size="xs" />}
          {!loading && !!error && <FleetLocationBadge location="unknown" />}
          {!loading && !error && status && <FleetLocationBadge location={fleetLocationOf(status)} />}
        </Stat>
        {!loading && !error && status?.fleetExists && (
          <>
            <Stat label="Réplicas">
              <Text size="sm" ff="monospace">
                {status.current}/{status.desired}
              </Text>
            </Stat>
            <Stat label="Ready">
              <Text size="sm" fw={600} c="nominal" ff="monospace">
                {status.ready}
              </Text>
            </Stat>
            <Stat label="Allocated">
              <Text size="sm" fw={600} c="signal" ff="monospace">
                {status.allocated}
              </Text>
            </Stat>
          </>
        )}
        {!loading && !error && status && status.cluster && !status.fleetExists && (
          <Text size="sm" c="dimmed">
            Fleet <b>{game.fleet}</b> não existe no cluster — aplique o manifesto
            (<code>infra/agones/apply-fleet.sh</code>).
          </Text>
        )}
      </Group>
    </Paper>
  );
}

function Stat({ label, children }: { label: string; children: ReactNode }) {
  return (
    <Stack gap={2}>
      <Text size="xs" c="dimmed" fw={600} tt="uppercase" style={{ letterSpacing: "0.04em" }}>
        {label}
      </Text>
      {children}
    </Stack>
  );
}

// FleetConfigTable: cadastro real do nome da fleet (ADR-0026), todos os jogos.
function FleetConfigTable({ games, onSaved }: { games: Game[]; onSaved: () => void }) {
  return (
    <Table verticalSpacing="sm" highlightOnHover>
      <Table.Thead>
        <Table.Tr>
          <Table.Th>Jogo</Table.Th>
          <Table.Th>Fleet Agones</Table.Th>
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
    </Table.Tr>
  );
}

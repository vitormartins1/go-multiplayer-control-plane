import {
  Alert,
  Badge,
  Box,
  Button,
  Collapse,
  Group,
  Loader,
  Select,
  Stack,
  Table,
  Text,
  TextInput,
  Title,
  Tooltip,
} from "@mantine/core";
import { IconAlertTriangle } from "@tabler/icons-react";
import { useState } from "react";
import { api, type Game } from "../lib/api";
import { useAsync } from "../lib/useAsync";

// mockFleetStatus (Fase 0, spec 059): dado ILUSTRATIVO determinístico por
// nome de fleet — sem chamada de rede. A integração real (build publicado
// num registry + leitura de estado do Agones) não existe ainda; ver "Visão
// de evolução futura" na spec. Determinístico (não aleatório a cada render)
// pra parecer estável entre reloads, mesmo espírito da capa gerada por seed.
function mockFleetStatus(fleetName: string) {
  const seed = [...fleetName].reduce((h, c) => (h * 31 + c.charCodeAt(0)) >>> 0, 7);
  const minor = 3 + (seed % 3);
  const patch = seed % 5;
  const currentTag = `v0.${minor}.${patch}`;
  const total = 2 + (seed % 3);
  return {
    currentTag,
    healthy: total,
    total,
    availableTags: [currentTag, `v0.${minor}.${patch + 1}`, `v0.${minor + 1}.0-rc1`],
  };
}

// Fleet (spec 059): cadastro real da Fleet Agones por jogo (ADR-0026 — o
// campo já existia no game-registry desde a spec 012, mas nunca tinha sido
// exposto pelo admin-bff/admin-ui) + esqueleto mockado da evolução futura
// (build/versão/health/promoção — nada disso existe hoje, ver spec).
export function Fleet() {
  const { data: games, loading, error, reload } = useAsync((s) => api.games(s), []);
  const [expandedId, setExpandedId] = useState<string | null>(null);

  return (
    <Stack>
      <Title order={3}>Fleet</Title>
      <Alert color="yellow" icon={<IconAlertTriangle size={16} />}>
        Imagem, versão e health desta tela são <b>ilustrativos</b> — a integração real (build
        publicado num registry de imagens + leitura de estado do Agones) ainda não existe. Ver
        spec 059.
      </Alert>

      {loading && !games && <Loader />}
      {!!error && !games && <Alert color="red">admin-bff indisponível.</Alert>}

      {games && (
        <Table striped highlightOnHover withTableBorder>
          <Table.Thead>
            <Table.Tr>
              <Table.Th>Jogo</Table.Th>
              <Table.Th>Fleet</Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {games.map((g) => (
              <FleetRow
                key={g.id}
                game={g}
                expanded={expandedId === g.id}
                onToggle={() => setExpandedId(expandedId === g.id ? null : g.id)}
                onSaved={reload}
              />
            ))}
          </Table.Tbody>
        </Table>
      )}
    </Stack>
  );
}

function FleetRow({
  game,
  expanded,
  onToggle,
  onSaved,
}: {
  game: Game;
  expanded: boolean;
  onToggle: () => void;
  onSaved: () => void;
}) {
  const [fleetInput, setFleetInput] = useState(game.fleet ?? "");
  const [saving, setSaving] = useState(false);
  const mock = game.fleet ? mockFleetStatus(game.fleet) : null;

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
    <>
      <Table.Tr style={{ cursor: "pointer" }} onClick={onToggle}>
        <Table.Td>{game.name}</Table.Td>
        <Table.Td>
          {game.fleet ? (
            <Badge variant="light">{game.fleet}</Badge>
          ) : (
            <Text c="dimmed" size="sm">
              sem fleet configurada
            </Text>
          )}
        </Table.Td>
      </Table.Tr>
      {expanded && (
        <Table.Tr>
          <Table.Td colSpan={2} p={0}>
            <Collapse in={expanded}>
              <Box p="md">
                <Stack gap="sm">
                  <Group align="flex-end">
                    <TextInput
                      label="Fleet Agones"
                      placeholder="packet-arena"
                      value={fleetInput}
                      onChange={(e) => setFleetInput(e.currentTarget.value)}
                      w={260}
                    />
                    <Button onClick={save} loading={saving}>
                      Salvar
                    </Button>
                  </Group>

                  {mock && (
                    <Group gap="xl" wrap="wrap" mt="xs">
                      <Stack gap={2}>
                        <Text size="xs" c="dimmed" fw={600}>
                          IMAGEM ATUAL
                        </Text>
                        <Text size="sm" ff="monospace">
                          {mock.currentTag} (mock)
                        </Text>
                      </Stack>
                      <Stack gap={2}>
                        <Text size="xs" c="dimmed" fw={600}>
                          HEALTH
                        </Text>
                        <Badge color="green" variant="light">
                          {mock.healthy}/{mock.total} saudáveis (mock)
                        </Badge>
                      </Stack>
                      <Stack gap={2}>
                        <Text size="xs" c="dimmed" fw={600}>
                          VERSÕES DISPONÍVEIS (mock)
                        </Text>
                        <Group gap={6}>
                          <Select data={mock.availableTags} defaultValue={mock.currentTag} w={160} size="xs" />
                          <Tooltip label="Integração com o registry de imagens ainda não implementada">
                            <Button size="xs" variant="light" disabled>
                              Promover
                            </Button>
                          </Tooltip>
                        </Group>
                      </Stack>
                    </Group>
                  )}
                </Stack>
              </Box>
            </Collapse>
          </Table.Td>
        </Table.Tr>
      )}
    </>
  );
}

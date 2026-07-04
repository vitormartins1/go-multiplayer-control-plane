import { Alert, Button, Card, Group, Loader, SimpleGrid, Stack, Table, Text, Title } from "@mantine/core";
import { IconRefresh } from "@tabler/icons-react";
import { api, type FanoutHealth } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { TopologyGraph } from "../components/TopologyGraph";
import { StatusBadge } from "../components/StatusBadge";

// Visão geral: mapa arquitetural + saúde leve dos serviços. Detalhe operacional
// (RED, latência, memória) fica no Grafana (ADR-0016).
export function Overview() {
  const { data, loading, error, reload } = useAsync((s) => api.topology(s), []);
  const fanout = useAsync((s) => api.fanoutHealth(s), []);

  return (
    <Stack>
      <Group justify="space-between">
        <Title order={3}>Infrastructure</Title>
        <Button
          variant="default"
          leftSection={<IconRefresh size={16} />}
          onClick={() => {
            reload();
            fanout.reload();
          }}
        >
          Atualizar
        </Button>
      </Group>

      {loading && !data && <Loader />}
      {!!error && !data && <Alert color="red">Painel indisponível (admin-bff fora?).</Alert>}

      {data && (
        <>
          {!data.sourceAvailable && (
            <Alert color="yellow">Prometheus indisponível — status dos serviços desconhecido.</Alert>
          )}
          <TopologyGraph topo={data} onSelectDomain={() => {}} />

          <Title order={5}>Saúde dos serviços</Title>
          <Table striped highlightOnHover withTableBorder>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Serviço</Table.Th>
                <Table.Th>Status</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {data.nodes.map((n) => (
                <Table.Tr key={n.id}>
                  <Table.Td>{n.label}</Table.Td>
                  <Table.Td>
                    <StatusBadge up={n.up} />
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </>
      )}

      <Title order={5}>Fan-out / entrega</Title>
      <Text size="sm" c="dimmed" mt={-8}>
        Entrega do resultado da partida (SNS→SQS, ADR-0020). Sinais do Prometheus.
      </Text>
      {fanout.error && !fanout.data ? (
        <Alert color="yellow">Métricas do fan-out indisponíveis (Prometheus fora?).</Alert>
      ) : (
        <FanoutCards health={fanout.data} />
      )}
    </Stack>
  );
}

function FanoutCards({ health }: { health: FanoutHealth | undefined }) {
  const fmt = (v: number | null | undefined) => (v == null ? "—" : String(v));
  const items = [
    { label: "Publicadas (ok)", value: fmt(health?.publishOk), color: undefined },
    { label: "Publicações c/ erro", value: fmt(health?.publishError), color: "red" as const },
    { label: "Consumidas (ok)", value: fmt(health?.consumeOk), color: undefined },
    { label: "Duplicadas (no-op)", value: fmt(health?.consumeDup), color: undefined },
    { label: "Consumos c/ erro", value: fmt(health?.consumeError), color: "red" as const },
  ];
  return (
    <SimpleGrid cols={{ base: 2, sm: 3, md: 5 }}>
      {items.map((it) => {
        const bad = it.color === "red" && it.value !== "—" && it.value !== "0";
        return (
          <Card key={it.label} withBorder padding="sm">
            <Text size="xs" c="dimmed">
              {it.label}
            </Text>
            <Text size="xl" fw={700} c={bad ? "red" : undefined}>
              {it.value}
            </Text>
          </Card>
        );
      })}
    </SimpleGrid>
  );
}

import { Alert, Button, Group, Loader, Stack, Table, Title } from "@mantine/core";
import { IconRefresh } from "@tabler/icons-react";
import { api } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { TopologyGraph } from "../components/TopologyGraph";
import { StatusBadge } from "../components/StatusBadge";

// Visão geral: mapa arquitetural + saúde leve dos serviços. Detalhe operacional
// (RED, latência, memória) fica no Grafana (ADR-0016).
export function Overview() {
  const { data, loading, error, reload } = useAsync((s) => api.topology(s), []);

  return (
    <Stack>
      <Group justify="space-between">
        <Title order={3}>Visão geral</Title>
        <Button variant="default" leftSection={<IconRefresh size={16} />} onClick={reload}>
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
    </Stack>
  );
}

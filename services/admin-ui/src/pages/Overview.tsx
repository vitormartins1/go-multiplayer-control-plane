import { useEffect } from "react";
import { Alert, Button, Group, Loader, SimpleGrid, Stack, Table, Text, Title } from "@mantine/core";
import { IconRefresh } from "@tabler/icons-react";
import { api, type FanoutHealth, type LiveMetrics } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { TopologyGraph } from "../components/TopologyGraph";
import { StatusBadge } from "../components/StatusBadge";
import { StatCard } from "../components/StatCard";

const LIVE_POLL_MS = 10000;

// Visão geral: mapa arquitetural + saúde leve dos serviços + sinais operacionais
// ao vivo (matchmaking, fan-out). Detalhe (RED, latência, memória) fica no
// Grafana (ADR-0016).
export function Overview() {
  const { data, loading, error, reload } = useAsync((s) => api.topology(s), []);
  const live = useAsync((s) => api.liveMetrics(s), []);
  const fanout = useAsync((s) => api.fanoutHealth(s), []);

  // Contadores operacionais são "ao vivo": refresca em intervalo curto.
  useEffect(() => {
    const id = setInterval(() => {
      live.reload();
      fanout.reload();
    }, LIVE_POLL_MS);
    return () => clearInterval(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <Stack>
      <Group justify="space-between">
        <Title order={3}>Infrastructure</Title>
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

      <Title order={5}>Matchmaking</Title>
      <Text size="sm" c="dimmed" mt={-8}>
        Começo do gameplay loop: fila → match → sessão. Sinais do Prometheus.
      </Text>
      {live.error && !live.data ? (
        <Alert color="yellow">Métricas do matchmaking indisponíveis (Prometheus fora?).</Alert>
      ) : (
        <MatchmakingCards m={live.data} />
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

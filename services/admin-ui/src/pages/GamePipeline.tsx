import { Alert, Badge, Group, Loader, Stack, Table, Text } from "@mantine/core";
import { api } from "../lib/api";
import { ARCHETYPE_META } from "../lib/archetypeMeta";
import { gameArchetypes, filterRulesForGame } from "../lib/eventRules";
import { modeScoringStats } from "../lib/modes";
import { useAsync } from "../lib/useAsync";

// Envelope que cada arquétipo publica no fan-out ao fim da partida/evento
// (ADR-0020/0023) e quem o consome — espelha o dispatch real dos consumers Go:
// o leaderboard descarta participation_event (nada a ranquear, adendo ADR-0028).
const KIND_BY_ARCHETYPE: Record<string, string> = {
  queue_arena: "win_loss_draw",
  host_world_dropin: "participation_event",
  battle_royale: "placement",
  coop_mission: "collective",
};
const CONSUMERS_BY_KIND: Record<string, string[]> = {
  win_loss_draw: ["achievements", "leaderboard", "analytics"],
  placement: ["achievements", "leaderboard", "analytics"],
  collective: ["achievements", "leaderboard", "analytics"],
  participation_event: ["achievements", "analytics"],
};

// GamePipeline (spec 046, virou tabela na revisão 2026-09): o mapa de dados do
// jogo — modo → envelope → consumidores, e o que cada consumidor faz com ele.
// Era um grafo React Flow com alturas de card calibradas à mão; a mesma
// informação cabe numa tabela, sem layout frágil, e agora inclui o
// session_closed (que todo fechamento publica e o grafo omitia).
export function GamePipeline({ gameId }: { gameId: string }) {
  const { data: modes, loading, error } = useAsync((s) => api.listMultiplayerModes(gameId, s), [gameId]);
  const { data: rules } = useAsync((s) => api.gameEventRules(gameId, s), [gameId]);
  const { data: catalog } = useAsync((s) => api.gameCatalog(gameId, s), [gameId]);

  if (loading && !modes) return <Loader />;
  if (error && !modes) return <Alert color="critical">game-registry indisponível.</Alert>;
  if (!modes || modes.length === 0) {
    return (
      <Text c="dimmed">
        Sem modos multiplayer cadastrados — o pipeline aparece quando o jogo tiver ao menos um modo
        (Configurações → Multiplayer).
      </Text>
    );
  }

  const visibleRules = filterRulesForGame(rules ?? [], gameArchetypes(modes));
  const eventTypes = new Set(visibleRules.flatMap((r) => r.eventTypes)).size;
  const byEvent = (catalog ?? []).filter((a) => !a.statKey && a.eventType).length;
  const byStat = (catalog ?? []).filter((a) => a.statKey).length;
  const allStats = [...new Set(modes.flatMap(modeScoringStats))];

  return (
    <Stack gap="sm">
      <Text size="sm" c="dimmed">
        O resultado de cada partida (ou cada evento de participação) vira um <b>envelope</b> publicado no
        fan-out (SNS→SQS); cada consumidor o traduz para a sua língua. Todo fechamento de sessão publica também
        um <code>session_closed</code>, que alimenta a saúde das sessões no analytics.
      </Text>
      <Table striped withTableBorder verticalSpacing="xs">
        <Table.Thead>
          <Table.Tr>
            <Table.Th>Modo</Table.Th>
            <Table.Th>Arquétipo</Table.Th>
            <Table.Th>Envelope</Table.Th>
            <Table.Th>Stats declaradas</Table.Th>
            <Table.Th>Consumidores</Table.Th>
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {modes.map((m) => {
            const kind = KIND_BY_ARCHETYPE[m.archetype];
            const stats = modeScoringStats(m);
            return (
              <Table.Tr key={m.modeId}>
                <Table.Td ff="monospace">{m.modeId}</Table.Td>
                <Table.Td>{ARCHETYPE_META[m.archetype].label}</Table.Td>
                <Table.Td>
                  <code>{kind}</code>
                </Table.Td>
                <Table.Td c={stats.length ? undefined : "dimmed"}>{stats.length ? stats.join(", ") : "—"}</Table.Td>
                <Table.Td>
                  <Group gap={4}>
                    {(CONSUMERS_BY_KIND[kind] ?? []).map((c) => (
                      <Badge key={c} size="xs" variant="light">
                        {c}
                      </Badge>
                    ))}
                  </Group>
                </Table.Td>
              </Table.Tr>
            );
          })}
        </Table.Tbody>
      </Table>
      <Table withTableBorder verticalSpacing="xs">
        <Table.Tbody>
          <Table.Tr>
            <Table.Td w={140} fw={600}>achievements</Table.Td>
            <Table.Td>
              {visibleRules.length} regras → {eventTypes} event types · {byEvent} conquistas por evento, {byStat} por
              stat. Traduz o envelope pelas event rules (aba Event Rules).
            </Table.Td>
          </Table.Tr>
          <Table.Tr>
            <Table.Td fw={600}>leaderboard</Table.Td>
            <Table.Td>
              board <code>score</code>
              {allStats.length > 0 ? ` + um board por stat (${allStats.join(", ")})` : " (sem stats declaradas)"}. Não
              recebe <code>participation_event</code> — não há o que ranquear.
            </Table.Td>
          </Table.Tr>
          <Table.Tr>
            <Table.Td fw={600}>analytics</Table.Td>
            <Table.Td>
              Agrega o envelope bruto, antes de qualquer regra (a história não muda quando as regras mudam), e o{" "}
              <code>session_closed</code>.
            </Table.Td>
          </Table.Tr>
        </Table.Tbody>
      </Table>
    </Stack>
  );
}

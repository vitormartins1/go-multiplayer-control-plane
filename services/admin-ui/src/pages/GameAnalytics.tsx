import { useMemo, useState } from "react";
import {
  Alert,
  Grid,
  Group,
  Loader,
  Paper,
  SegmentedControl,
  Select,
  SimpleGrid,
  Stack,
  Table,
  Text,
} from "@mantine/core";
import { BarChart, DonutChart, LineChart } from "@mantine/charts";
import { IconActivity } from "@tabler/icons-react";
import { api, type MultiplayerMode, type MultiplayerProfile, type ParticipationStat } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { StatCard } from "../components/StatCard";
import { ChannelStrip } from "../components/ChannelStrip";
import { formatDuration } from "../lib/format";
import { ARCHETYPE_META } from "../lib/archetypeMeta";
import {
  CLOSE_CATEGORY_LABELS,
  ROLE_ORDER,
  closeReasonMeta,
  participationEventMeta,
  participationOutcomeMeta,
  roleMeta,
} from "../lib/vocab";

// arquétipos na ordem canônica (mesma de ARCHETYPE_META/badges do header do
// GameDetail) — define a ordem das seções do layout estático (spec 067).
const ARCHETYPE_ORDER: MultiplayerProfile["archetype"][] = [
  "queue_arena",
  "host_world_dropin",
  "battle_royale",
  "coop_mission",
];

// Card = um widget com posição fixa no grid de 12 colunas (spec 067: layout
// estático e responsivo, substitui o dashboard editável da spec 062).
interface CardDef {
  id: string;
  title: string;
  span: number; // colunas de 12 (Grid do Mantine) — responsivo via Grid.Col
  // arquétipo da seção onde o card aparece; ausente = seção "Geral"
  // (cross-arquétipo: stats, saúde das sessões). Com um modo selecionado, o
  // backend já devolve vazio para os outros arquétipos — não há filtro aqui.
  archetype?: MultiplayerProfile["archetype"];
  render: () => React.ReactNode;
}

// Section agrupa cards por arquétipo (spec 067) — archetype ausente = seção
// final cross-arquétipo (raw-stats/session-health), sem cabeçalho de ícone.
interface Section {
  archetype?: MultiplayerProfile["archetype"];
  cards: CardDef[];
}

function modeLabel(modeId: string, modes: MultiplayerMode[]): string {
  if (!modeId) return "Sem modo";
  return modes.find((m) => m.modeId === modeId)?.displayName || modeId;
}

// hostWorldKPIs traduz a tabela papel × evento × desfecho nos números que um
// operador de Souls-like acompanha (revisão 2026-09): invasões e co-ops que
// entraram, o que aconteceu com eles, e bosses derrotados com ajuda.
function hostWorldKPIs(stats: ParticipationStat[]) {
  const count = (role: string, event: string, outcome?: string) =>
    stats
      .filter((s) => s.role === role && s.event === event && (outcome === undefined || s.outcome === outcome))
      .reduce((acc, s) => acc + s.count, 0);
  const invasions = count("invader", "JOINED");
  const invasionsWon = count("invader", "HOST_KILLED", "SUCCESS");
  return {
    invasions,
    invasionsWon,
    invadersKilled: count("invader", "DIED"),
    invasionWinRate: invasions > 0 ? `${Math.round((invasionsWon / invasions) * 100)}%` : "—",
    coops: count("coop", "JOINED"),
    bossesHelped: count("coop", "BOSS_DEFEATED", "SUCCESS"),
    coopsDied: count("coop", "DIED"),
  };
}

// participationByRole agrupa por papel (Host/Co-op/Invasor) com um campo por
// tipo de evento — vira um bar chart empilhado de 3 categorias em vez de uma
// combinação "papel · evento" (achado do Vitor: com dado real, 10 combinações
// de rótulo longo colidiam e ficavam ilegíveis no eixo).
function participationByRole(stats: ParticipationStat[]) {
  const events = [...new Set(stats.map((s) => s.event))];
  const byRole = new Map<string, Record<string, number>>();
  for (const s of stats) {
    const row = byRole.get(s.role) ?? {};
    row[s.event] = (row[s.event] ?? 0) + s.count;
    byRole.set(s.role, row);
  }
  const roles = [...byRole.keys()].sort(
    (a, b) => (ROLE_ORDER.indexOf(a) === -1 ? 99 : ROLE_ORDER.indexOf(a)) -
      (ROLE_ORDER.indexOf(b) === -1 ? 99 : ROLE_ORDER.indexOf(b)),
  );
  const data = roles.map((role) => ({ role: roleMeta(role).label, ...byRole.get(role) }));
  const series = events.map((e) => ({
    name: e,
    label: participationEventMeta(e).label,
    color: participationEventMeta(e).color,
  }));
  return { data, series };
}

// dailyLineChart é o LineChart genérico reaproveitado pelas 4 séries diárias
// (queue_arena + as 3 novas da spec 010) — mesmo visual.
function DailyLineChart({ data, label }: { data: { day: string; matches: number }[]; label: string }) {
  return (
    <LineChart
      h={170}
      data={data}
      dataKey="day"
      series={[{ name: "matches", label, color: "signal.5" }]}
      curveType="monotone"
      withDots={data.length < 40}
      tickLine="x"
      gridAxis="xy"
    />
  );
}

export function GameAnalytics({ gameId, modes = [] }: { gameId: string; modes?: MultiplayerMode[] }) {
  // scope (spec 087, pedido do Vitor): a escolha PRIMÁRIA é binária — ver
  // tudo agregado ou entrar num modo específico — então virou um
  // SegmentedControl em vez de item perdido dentro de um Select. O Select
  // de modo só existe (e só faz sentido existir) quando scope="by-mode":
  // ali sim é "qual dos N modos", uma escolha genuinamente de lista.
  const [scope, setScope] = useState<"all" | "by-mode">("all");
  // selectedMode (modeId real) só é lido quando scope="by-mode" —
  // inicializado no clique do SegmentedControl (ver abaixo), nunca fica
  // "by-mode" com seleção vazia.
  const [selectedMode, setSelectedMode] = useState<string | null>(null);
  const modeIdParam = scope === "by-mode" ? (selectedMode ?? undefined) : undefined;
  const { data, loading, error } = useAsync(
    (s) => api.gameAnalytics(gameId, { modeId: modeIdParam }, s),
    [gameId, modeIdParam],
  );

  const hasData = data && data.matches > 0;
  const participations = data?.roleParticipations ?? [];
  // modos distintos NOS DADOS RECEBIDOS (spec 009) — só tem >1 quando
  // "Todos os modos" está selecionado (um modo específico já vem filtrado
  // pelo backend, spec 064); decide se a coluna "Modo" da tabela aparece.
  const modeIdsInData = useMemo(() => [...new Set(participations.map((p) => p.modeId))], [participations]);
  // total por partida (independente de arquétipo) — só some se não houver nenhum resultado.
  const hasOutcomes = !!data && data.wins + data.losses + data.draws > 0;
  const roleChart = participations.length > 0 ? participationByRole(participations) : null;

  // leitura mono de cada canal (ChannelStrip) — um total por arquétipo, o
  // dado mais "de assinatura" de cada um (partidas/eventos/participações/
  // missões), não uma soma genérica sem sentido pro domínio.
  const archetypeReadout = useMemo<Partial<Record<MultiplayerProfile["archetype"], string>>>(() => {
    const out: Partial<Record<MultiplayerProfile["archetype"], string>> = {};
    if (!data) return out;
    if (hasData) out.queue_arena = `${data.matches} partidas`;
    if (participations.length > 0) {
      const total = participations.reduce((acc, p) => acc + p.count, 0);
      out.host_world_dropin = `${total} eventos`;
    }
    if (data.placement) out.battle_royale = `${data.placement.entries} participações`;
    if (data.mission) out.coop_mission = `${data.mission.successes + data.mission.failures} missões`;
    return out;
  }, [data, hasData, participations]);

  const generalReadout = useMemo(() => {
    if (!data || data.sessionHealth.length === 0) return undefined;
    const total = data.sessionHealth.reduce((acc, h) => acc + h.count, 0);
    return `${total} sessões`;
  }, [data]);

  // Catálogo de widgets (spec 062): cada seção vira um widget com id estável,
  // disponível só quando a condição de dado (mesma de antes, agora
  // centralizada) é satisfeita. A ordem aqui é só o default inicial — o
  // operador reordena/redimensiona/esconde livremente (persistido por jogo).
  const cards = useMemo<CardDef[]>(() => {
    if (!data) return [];
    const out: CardDef[] = [];

    if (hasData) {
      out.push({
        id: "summary",
        archetype: "queue_arena",
        title: "Resumo",
        // span 8 (spec 086): a dupla de cards fundidos (grid de stats + donut,
        // spec 068) precisa de mais respiro horizontal que o par "por dia" ao
        // lado (que é só um LineChart) — 50/50 deixava os valores grandes
        // apertados nas caixinhas. 8/4 (≈67/33) em vez de 6/6.
        span: 8,
        render: () => (
          <Group align="center" wrap="wrap" gap="lg">
            <SimpleGrid cols={{ base: 2, sm: 3 }} style={{ flex: "1 1 260px" }}>
              <StatCard size="lg" label="Partidas" value={String(data.matches)} />
              <StatCard size="lg" label="Jogadores únicos" value={String(data.uniquePlayers)} />
              <StatCard size="lg" label="Score médio" value={data.avgScore.toFixed(1)} />
              <StatCard size="lg" label="Vitórias" value={String(data.wins)} />
              <StatCard size="lg" label="Derrotas" value={String(data.losses)} />
              <StatCard size="lg" label="Empates" value={String(data.draws)} />
            </SimpleGrid>
            {hasOutcomes && (
              <DonutChart
                size={120}
                thickness={18}
                withLabelsLine
                withLabels
                data={[
                  { name: "Vitórias", value: data.wins, color: "green.6" },
                  { name: "Derrotas", value: data.losses, color: "red.6" },
                  { name: "Empates", value: data.draws, color: "yellow.6" },
                ]}
              />
            )}
          </Group>
        ),
      });
    }

    if (data.daily.length > 0) {
      out.push({
        id: "queue-daily",
        archetype: "queue_arena",
        title: "Partidas por dia",
        span: 4,
        render: () => <DailyLineChart data={data.daily} label="Partidas" />,
      });
    }

    // host_world_dropin (spec 004/020/009): participações por papel/evento/outcome.
    // Seletor de modo interno REMOVIDO (spec 064) — superado pelo seletor de
    // página, que já filtra no backend; modeIdsInData só tem >1 elemento
    // quando "Todos os modos" está selecionado (um modo específico já vem
    // filtrado, então a coluna "Modo" não teria variação pra mostrar).
    if (roleChart) {
      const k = hostWorldKPIs(participations);
      out.push({
        id: "host-world-summary",
        archetype: "host_world_dropin",
        title: "Resumo do mundo",
        span: 12,
        render: () => (
          <SimpleGrid cols={{ base: 2, sm: 4, lg: 7 }}>
            <StatCard size="lg" label="Invasões" value={String(k.invasions)} />
            <StatCard size="lg" label="Invasões vencidas" value={String(k.invasionsWon)} />
            <StatCard size="lg" label="Vitória do invasor" value={k.invasionWinRate} />
            <StatCard size="lg" label="Invasores abatidos" value={String(k.invadersKilled)} />
            <StatCard size="lg" label="Co-ops invocados" value={String(k.coops)} />
            <StatCard size="lg" label="Bosses com ajuda" value={String(k.bossesHelped)} />
            <StatCard size="lg" label="Co-ops mortos" value={String(k.coopsDied)} />
          </SimpleGrid>
        ),
      });
      out.push({
        id: "participations-role",
        archetype: "host_world_dropin",
        title: "Participações por papel",
        span: 12,
        render: () => (
          <Stack gap="md">
            <BarChart
              h={Math.max(160, roleChart.data.length * 70)}
              data={roleChart.data}
              dataKey="role"
              series={roleChart.series}
              type="stacked"
              orientation="vertical"
              gridAxis="x"
              xAxisProps={{ allowDecimals: false }}
              withLegend
              legendProps={{ verticalAlign: "bottom" }}
            />
            <Table striped withTableBorder>
              <Table.Thead>
                <Table.Tr>
                  {modeIdsInData.length > 1 && <Table.Th>Modo</Table.Th>}
                  <Table.Th>Papel</Table.Th>
                  <Table.Th>Evento</Table.Th>
                  <Table.Th>Desfecho</Table.Th>
                  <Table.Th ta="right">Total</Table.Th>
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {participations.map((p) => (
                  <Table.Tr key={`${p.modeId}-${p.role}-${p.event}-${p.outcome}`}>
                    {modeIdsInData.length > 1 && <Table.Td c="dimmed">{modeLabel(p.modeId, modes)}</Table.Td>}
                    <Table.Td>{roleMeta(p.role).label}</Table.Td>
                    <Table.Td>{participationEventMeta(p.event).label}</Table.Td>
                    <Table.Td c="dimmed">{participationOutcomeMeta(p.outcome).label}</Table.Td>
                    <Table.Td ta="right" ff="monospace">{p.count}</Table.Td>
                  </Table.Tr>
                ))}
              </Table.Tbody>
            </Table>
          </Stack>
        ),
      });
    }

    // battle_royale (BR4): distribuição de colocação.
    if (data.placement) {
      const placement = data.placement;
      out.push({
        id: "placement-distribution",
        archetype: "battle_royale",
        title: "Distribuição de colocação",
        span: 8, // spec 086: mesmo tratamento de "summary" — ver comentário lá.
        render: () => (
          <Group align="center" wrap="wrap" gap="lg">
            <SimpleGrid cols={{ base: 2, sm: 3 }} style={{ flex: "1 1 260px" }}>
              <StatCard size="lg" label="1º lugar" value={String(placement.winners)} />
              <StatCard size="lg" label="Top 10" value={String(placement.topN)} />
              <StatCard size="lg" label="Resto" value={String(placement.rest)} />
              <StatCard size="lg" label="Participações" value={String(placement.entries)} />
              <StatCard size="lg" label="Sobrevivência média" value={formatDuration(placement.avgSurvivedMs)} />
            </SimpleGrid>
            <DonutChart
              size={120}
              thickness={18}
              withLabelsLine
              withLabels
              data={[
                { name: "1º lugar", value: placement.winners, color: "yellow.6" },
                { name: "Top 10", value: placement.topN, color: "blue.5" },
                { name: "Resto", value: placement.rest, color: "gray.5" },
              ]}
            />
          </Group>
        ),
      });
    }

    // battle_royale (spec 010): série "partidas por dia" — não existia antes.
    if (data.placementDaily.length > 0) {
      out.push({
        id: "placement-daily",
        archetype: "battle_royale",
        title: "Partidas de battle royale por dia",
        span: 4,
        render: () => <DailyLineChart data={data.placementDaily} label="Partidas" />,
      });
    }

    // coop_mission (C4): desfecho coletivo da missão.
    if (data.mission) {
      const mission = data.mission;
      out.push({
        id: "mission-distribution",
        archetype: "coop_mission",
        title: "Desfecho das missões",
        span: 8, // spec 086: mesmo tratamento de "summary" — ver comentário lá.
        render: () => (
          <Group align="center" wrap="wrap" gap="lg">
            <SimpleGrid cols={{ base: 2, sm: 3 }} style={{ flex: "1 1 260px" }}>
              <StatCard size="lg" label="Missões cumpridas" value={String(mission.successes)} />
              <StatCard size="lg" label="Missões falhas" value={String(mission.failures)} />
              <StatCard size="lg" label="Taxa de sucesso" value={`${(mission.successRate * 100).toFixed(1)}%`} />
            </SimpleGrid>
            <DonutChart
              size={120}
              thickness={18}
              withLabelsLine
              withLabels
              data={[
                { name: "Cumpridas", value: mission.successes, color: "green.6" },
                { name: "Falhas", value: mission.failures, color: "red.6" },
              ]}
            />
          </Group>
        ),
      });
    }

    // coop_mission (spec 010): série "missões por dia" — não existia antes.
    if (data.missionDaily.length > 0) {
      out.push({
        id: "mission-daily",
        archetype: "coop_mission",
        title: "Missões por dia",
        span: 4,
        render: () => <DailyLineChart data={data.missionDaily} label="Missões" />,
      });
    }

    // host_world_dropin (spec 010): série "eventos de participação por dia" —
    // não existia antes (o arquétipo não tem partida terminal).
    if (data.participationDaily.length > 0) {
      out.push({
        id: "participation-daily",
        archetype: "host_world_dropin",
        title: "Eventos de participação por dia",
        span: 12,
        render: () => <DailyLineChart data={data.participationDaily} label="Eventos" />,
      });
    }

    // stats brutos (spec 011): resumo agregado (soma/contagem/média) por métrica.
    // Sem ícone (spec 064): cross-arquétipo por natureza, já vem corretamente
    // filtrado pelo backend quando um modo está selecionado (spec 012).
    if (data.stats.length > 0) {
      const stats = data.stats;
      out.push({
        id: "raw-stats",
        title: "Stats agregados",
        // span 12 (spec 070): sem par natural na seção cross-arquétipo —
        // session-health e session-closed-daily formam o par "distribuição
        // + diário", raw-stats fica sozinho, full-width.
        span: 12,
        render: () => (
          <Table striped withTableBorder>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Stat</Table.Th>
                <Table.Th ta="right">Soma</Table.Th>
                <Table.Th ta="right">Ocorrências</Table.Th>
                <Table.Th ta="right">Média</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {stats.map((s) => (
                <Table.Tr key={s.key}>
                  <Table.Td>{s.key}</Table.Td>
                  <Table.Td ta="right" ff="monospace">{s.sum}</Table.Td>
                  <Table.Td ta="right" ff="monospace">{s.count}</Table.Td>
                  <Table.Td ta="right" ff="monospace">{s.avg.toFixed(1)}</Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        ),
      });
    }

    // saúde das sessões (spec 065/013): distribuição por motivo de
    // fechamento. Cross-arquétipo (mesmo tratamento de raw-stats): a
    // tabela de origem cobre TODO arquétipo (todo closeSession publica o
    // evento).
    if (data.sessionHealth.length > 0) {
      const health = data.sessionHealth;
      const healthModeIds = [...new Set(health.map((h) => h.modeId ?? ""))];
      // classificação (spec 069): soma por categoria / total — client-side
      // em cima do sessionHealth já filtrado por modo pelo backend.
      const categoryTotals = { concluded: 0, manual: 0, abandoned: 0 };
      let healthTotal = 0;
      for (const h of health) {
        categoryTotals[closeReasonMeta(h.closeReason).category] += h.count;
        healthTotal += h.count;
      }
      // "72% (5)" — percentual em destaque (é o que importa pra saúde),
      // contagem absoluta entre parênteses (StatCard só tem 1 campo de valor).
      const summary = (n: number) => (healthTotal > 0 ? `${((n / healthTotal) * 100).toFixed(0)}% (${n})` : `— (${n})`);
      out.push({
        id: "session-health",
        title: "Saúde das sessões",
        span: 8, // spec 086: mesmo tratamento de "summary" — ver comentário lá.
        render: () => (
          <Stack gap="md">
            <Group align="center" wrap="wrap" gap="lg">
              <SimpleGrid cols={3} style={{ flex: "1 1 260px" }}>
                <StatCard
                  size="lg"
                  label={CLOSE_CATEGORY_LABELS.concluded}
                  value={summary(categoryTotals.concluded)}
                />
                <StatCard size="lg" label={CLOSE_CATEGORY_LABELS.manual} value={summary(categoryTotals.manual)} />
                <StatCard
                  size="lg"
                  label={CLOSE_CATEGORY_LABELS.abandoned}
                  value={summary(categoryTotals.abandoned)}
                  bad={categoryTotals.abandoned > 0}
                />
              </SimpleGrid>
              <DonutChart
                size={120}
                thickness={18}
                withLabelsLine
                withLabels
                data={health.map((h) => ({
                  name: closeReasonMeta(h.closeReason).label,
                  value: h.count,
                  color: closeReasonMeta(h.closeReason).color,
                }))}
              />
            </Group>
            <Table striped withTableBorder>
              <Table.Thead>
                <Table.Tr>
                  {healthModeIds.length > 1 && <Table.Th>Modo</Table.Th>}
                  <Table.Th>Motivo</Table.Th>
                  <Table.Th ta="right">Contagem</Table.Th>
                  <Table.Th ta="right">Duração média</Table.Th>
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {health.map((h) => (
                  <Table.Tr key={`${h.modeId}-${h.closeReason}`}>
                    {healthModeIds.length > 1 && (
                      <Table.Td c="dimmed">{modeLabel(h.modeId ?? "", modes)}</Table.Td>
                    )}
                    <Table.Td>{closeReasonMeta(h.closeReason).label}</Table.Td>
                    <Table.Td ta="right" ff="monospace">{h.count}</Table.Td>
                    <Table.Td ta="right" ff="monospace">{formatDuration(h.avgDurationMs)}</Table.Td>
                  </Table.Tr>
                ))}
              </Table.Tbody>
            </Table>
          </Stack>
        ),
      });
    }

    // sessões fechadas por dia (spec 015/070): pareado com session-health,
    // mesmo tratamento cross-arquétipo (sem ícone, sempre elegível).
    if (data.sessionCloseDaily.length > 0) {
      out.push({
        id: "session-closed-daily",
        title: "Sessões fechadas por dia",
        span: 4,
        render: () => <DailyLineChart data={data.sessionCloseDaily} label="Sessões" />,
      });
    }

    return out;
  }, [data, hasData, hasOutcomes, roleChart, participations, modeIdsInData, modes]);

  // Agrupa os cards em seções por arquétipo, ordem fixa (spec 067: layout
  // estático — substitui o dashboard editável da spec 062). Cards sem
  // arquétipo (raw-stats/session-health) formam a seção final, sem cabeçalho.
  const sections = useMemo<Section[]>(() => {
    const byArchetype = new Map<MultiplayerProfile["archetype"], CardDef[]>();
    const crossArchetype: CardDef[] = [];
    for (const card of cards) {
      const archetype = card.archetype;
      if (!archetype) {
        crossArchetype.push(card);
        continue;
      }
      if (!byArchetype.has(archetype)) byArchetype.set(archetype, []);
      byArchetype.get(archetype)!.push(card);
    }
    const out: Section[] = ARCHETYPE_ORDER.filter((a) => byArchetype.has(a)).map((archetype) => ({
      archetype,
      cards: byArchetype.get(archetype)!,
    }));
    if (crossArchetype.length > 0) out.push({ archetype: undefined, cards: crossArchetype });
    return out;
  }, [cards]);

  return (
    <Stack>
      {loading && !data && <Loader />}
      {!!error && !data && <Alert color="critical">Analytics indisponível (analytics-service fora?).</Alert>}

      {modes.length > 1 && (
        <Group gap="md" align="flex-end">
          <SegmentedControl
            value={scope}
            onChange={(v) => {
              const next = v as "all" | "by-mode";
              setScope(next);
              // 1ª vez entrando em "by-mode" sem seleção prévia: assume o
              // 1º modo em vez de deixar o Select vazio (pedido do Vitor —
              // o filtro por modo só faz sentido ali dentro, então já
              // chega mostrando algo, não um estado "escolha um modo").
              if (next === "by-mode" && !selectedMode) setSelectedMode(modes[0]?.modeId ?? null);
            }}
            data={[
              { label: "Todos os modos", value: "all" },
              { label: "Por modo", value: "by-mode" },
            ]}
          />
          {scope === "by-mode" && (
            <Select
              label="Modo"
              w={240}
              allowDeselect={false}
              value={selectedMode}
              onChange={(v) => v && setSelectedMode(v)}
              data={modes.map((m) => ({ value: m.modeId, label: modeLabel(m.modeId, modes) }))}
            />
          )}
        </Group>
      )}

      {data && cards.length === 0 && (
        <Text c="dimmed">
          Sem dados ainda — os agregados aparecem conforme as sessões fecham e os servidores reportam resultados e eventos.
        </Text>
      )}

      {sections.map((section, i) => {
        const meta = section.archetype ? ARCHETYPE_META[section.archetype] : undefined;
        const readout = section.archetype ? archetypeReadout[section.archetype] : generalReadout;
        return (
          <Stack key={section.archetype ?? "geral"} gap="sm" mt={i > 0 ? "lg" : undefined}>
            <ChannelStrip icon={meta?.icon ?? IconActivity} label={meta?.label ?? "Geral"} readout={readout} />
            {/* align="stretch" (default do Grid, spec 086): os pares da seção
                (ex.: Resumo + Partidas por dia) ficam com a MESMA altura — a
                spec 068 tinha desligado isso (align="flex-start") porque o
                donut empilhado abaixo do grid de stats deixava o card
                desproporcionalmente alto; agora que o donut fica ao lado
                (não embaixo), as duas metades do par já nascem com altura
                parecida, então esticar pra igualar fica bem (pedido do
                Vitor: "cada subseção ali... ficar alinhada com a mesma
                altura"). Paper h="100%" é o que de fato preenche a coluna
                esticada — sem isso o Grid.Col cresce mas o Paper de dentro
                continua só do tamanho do conteúdo. */}
            <Grid>
              {section.cards.map((card) => (
                <Grid.Col key={card.id} span={{ base: 12, md: card.span }}>
                  <Paper withBorder p="md" h="100%">
                    <Text fw={600} size="sm" mb="xs">
                      {card.title}
                    </Text>
                    {card.render()}
                  </Paper>
                </Grid.Col>
              ))}
            </Grid>
          </Stack>
        );
      })}
    </Stack>
  );
}

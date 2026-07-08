import { Alert, Button, Divider, Group, Loader, Progress, SegmentedControl, Stack, Table, Text, Tooltip } from "@mantine/core";
import { IconRefresh } from "@tabler/icons-react";
import { useMemo, useState } from "react";
import { api, type PlayerAchievement } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { AchievementIcon } from "../components/AchievementIcon";

// pctOf: fração concluída de uma conquista em [0, 1]. Desbloqueada conta como
// inteira mesmo se o progresso registrado ficou aquém do alvo (dado antigo).
function pctOf(a: PlayerAchievement): number {
  if (a.unlocked) return 1;
  if (a.target <= 0) return 0;
  return Math.min(a.progress / a.target, 1);
}

function AchievementRow({ a }: { a: PlayerAchievement }) {
  return (
    <Table.Tr>
      <Table.Td>
        <Group gap="sm" wrap="nowrap">
          <AchievementIcon seed={a.id} size={28} />
          <Text size="sm" truncate>
            {a.name}
          </Text>
        </Group>
      </Table.Td>
      <Table.Td>
        <Group gap="sm" wrap="nowrap">
          <Text size="sm" w={70} style={{ whiteSpace: "nowrap" }}>
            {a.progress}/{a.target}
          </Text>
          <Progress value={pctOf(a) * 100} color={a.unlocked ? "green" : "blue"} size="sm" flex={1} />
        </Group>
      </Table.Td>
      <Table.Td>
        {a.unlocked ? (
          <Text span size="sm" c="green">
            desbloqueada
            {a.unlockedAt ? ` · ${new Date(a.unlockedAt).toLocaleString("pt-BR")}` : ""}
          </Text>
        ) : (
          <Text span size="sm" c="dimmed">
            {a.progress > 0 ? "em progresso" : "bloqueada"}
          </Text>
        )}
      </Table.Td>
    </Table.Tr>
  );
}

// Larguras fixas e idênticas em todas as tabelas (tableLayout: fixed): a leitura
// não "pula" entre a visão única e os grupos da visão por estado (spec 040).
function AchievementsTable({ rows }: { rows: PlayerAchievement[] }) {
  return (
    <Table striped highlightOnHover withTableBorder style={{ tableLayout: "fixed" }}>
      <Table.Thead>
        <Table.Tr>
          <Table.Th w="38%">Conquista</Table.Th>
          <Table.Th w="32%">Progresso</Table.Th>
          <Table.Th w="30%">Estado</Table.Th>
        </Table.Tr>
      </Table.Thead>
      <Table.Tbody>
        {rows.map((a) => (
          <AchievementRow key={a.id} a={a} />
        ))}
      </Table.Tbody>
    </Table>
  );
}

// GroupSection: uma das três seções da visão "Por estado", empilhadas em uma
// coluna com um Divider rotulado separando cada uma. Grupo vazio não some —
// mostra a contagem 0 e "nenhuma" (o admin vê que a categoria existe e está vazia).
function GroupSection({ title, rows }: { title: string; rows: PlayerAchievement[] }) {
  return (
    <Stack gap="xs">
      <Divider
        labelPosition="left"
        label={
          <Text size="sm" fw={600}>
            {title}{" "}
            <Text span size="sm" c="dimmed">
              · {rows.length}
            </Text>
          </Text>
        }
      />
      {rows.length > 0 ? <AchievementsTable rows={rows} /> : <Text c="dimmed">nenhuma</Text>}
    </Stack>
  );
}

// OverallProgress: barra geral compacta, na linha dos controles (spec 040,
// refinada). O percentual é ponderado pelo alvo (Σ progresso / Σ target):
// 1/3600 pesa mais que 1/3. O numeral "X de N" é o fato bruto, sem ponderação.
function OverallProgress({ data }: { data: PlayerAchievement[] }) {
  const { unlocked, pct } = useMemo(() => {
    const totalTarget = data.reduce((sum, a) => sum + Math.max(a.target, 0), 0);
    const done = data.reduce((sum, a) => sum + pctOf(a) * Math.max(a.target, 0), 0);
    return {
      unlocked: data.filter((a) => a.unlocked).length,
      pct: totalTarget > 0 ? (done / totalTarget) * 100 : 0,
    };
  }, [data]);

  const pctLabel = pct > 0 && pct < 0.1 ? "<0,1" : pct.toLocaleString("pt-BR", { maximumFractionDigits: 1 });
  return (
    <Group gap="xs" wrap="nowrap">
      <Text size="sm" fw={600} style={{ whiteSpace: "nowrap" }}>
        Progresso geral
      </Text>
      <Tooltip label="Ponderado pelo alvo de cada conquista: unidades de progresso cumpridas sobre o total de unidades do catálogo.">
        <Progress value={pct} color={unlocked === data.length ? "green" : "blue"} size="sm" w={180} />
      </Tooltip>
      <Text size="sm" c="dimmed" style={{ whiteSpace: "nowrap" }}>
        {unlocked} de {data.length} · {pctLabel}%
      </Text>
    </Group>
  );
}

// PlayerAchievements: progresso do jogador no jogo ativo (gameId vem do contexto
// global, via PlayerDetail). Sem gameId -> orienta a escolher no header.
export function PlayerAchievements({ playerId, gameId }: { playerId: string; gameId: string | null }) {
  const { data, loading, error, reload } = useAsync(
    (s) => (gameId ? api.playerAchievements(gameId, playerId, s) : Promise.resolve([])),
    [gameId, playerId],
  );
  const [view, setView] = useState<"table" | "grouped">("table");
  const [order, setOrder] = useState<"name" | "progress">("name");

  // Ordenação rápida (spec 040, refinada): A–Z ou % de progresso (desc, nome
  // como desempate). Vale nas duas visões; na "Por estado", dentro de cada grupo.
  const sorted = useMemo(() => {
    const list = [...(data ?? [])];
    if (order === "name") return list.sort((a, b) => a.name.localeCompare(b.name, "pt-BR"));
    return list.sort((a, b) => pctOf(b) - pctOf(a) || a.name.localeCompare(b.name, "pt-BR"));
  }, [data, order]);

  // Partição da visão "Por estado" (spec 040): conquistadas, em progresso
  // (progresso parcial) e bloqueadas (sem progresso), nesta ordem.
  const groups = useMemo(
    () => ({
      unlocked: sorted.filter((a) => a.unlocked),
      inProgress: sorted.filter((a) => !a.unlocked && a.progress > 0),
      locked: sorted.filter((a) => !a.unlocked && a.progress === 0),
    }),
    [sorted],
  );

  if (!gameId) return <Text c="dimmed">Selecione um jogo no topo para ver o progresso do jogador.</Text>;

  return (
    <Stack>
      <Group justify="space-between" gap="xs">
        {data && data.length > 0 ? <OverallProgress data={data} /> : <span />}
        <Group gap="xs">
          <SegmentedControl
            size="xs"
            value={order}
            onChange={(v) => setOrder(v as "name" | "progress")}
            data={[
              { value: "name", label: "A–Z" },
              { value: "progress", label: "Progresso" },
            ]}
          />
          <SegmentedControl
            size="xs"
            value={view}
            onChange={(v) => setView(v as "table" | "grouped")}
            data={[
              { value: "table", label: "Tabela" },
              { value: "grouped", label: "Por estado" },
            ]}
          />
          <Button variant="default" size="xs" leftSection={<IconRefresh size={14} />} onClick={reload}>
            Atualizar
          </Button>
        </Group>
      </Group>

      {loading && !data && <Loader />}
      {!!error && <Alert color="red">Falha ao carregar (achievements indisponível?).</Alert>}
      {data && data.length === 0 && <Text c="dimmed">Nenhum progresso para este jogador neste jogo.</Text>}
      {data && data.length > 0 &&
        (view === "table" ? (
          <AchievementsTable rows={sorted} />
        ) : (
          <>
            <GroupSection title="Conquistadas" rows={groups.unlocked} />
            <GroupSection title="Em progresso" rows={groups.inProgress} />
            <GroupSection title="Bloqueadas" rows={groups.locked} />
          </>
        ))}
    </Stack>
  );
}

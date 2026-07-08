import { Alert, Button, Group, Loader, Progress, SegmentedControl, Stack, Table, Text, Tooltip } from "@mantine/core";
import { IconRefresh } from "@tabler/icons-react";
import { useMemo, useState } from "react";
import { api, type PlayerAchievement } from "../lib/api";
import { useAsync } from "../lib/useAsync";

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
      <Table.Td>{a.name}</Table.Td>
      <Table.Td w={220}>
        <Group gap="sm" wrap="nowrap">
          <Text size="sm" w={80} style={{ whiteSpace: "nowrap" }}>
            {a.progress}/{a.target}
          </Text>
          <Progress value={pctOf(a) * 100} color={a.unlocked ? "green" : "blue"} size="sm" flex={1} />
        </Group>
      </Table.Td>
      <Table.Td>
        {a.unlocked ? (
          <Text span c="green">
            desbloqueada
            {a.unlockedAt ? ` · ${new Date(a.unlockedAt).toLocaleString("pt-BR")}` : ""}
          </Text>
        ) : (
          <Text span c="dimmed">
            {a.progress > 0 ? "em progresso" : "bloqueada"}
          </Text>
        )}
      </Table.Td>
    </Table.Tr>
  );
}

function AchievementsTable({ rows }: { rows: PlayerAchievement[] }) {
  return (
    <Table striped highlightOnHover withTableBorder>
      <Table.Thead>
        <Table.Tr>
          <Table.Th>Conquista</Table.Th>
          <Table.Th>Progresso</Table.Th>
          <Table.Th>Estado</Table.Th>
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

// GroupSection: uma das três tabelas da visão "Por estado". Grupo vazio não
// some — mostra o título com contagem 0 e "nenhuma" (o admin vê que a
// categoria existe e está vazia).
function GroupSection({ title, rows }: { title: string; rows: PlayerAchievement[] }) {
  return (
    <Stack gap="xs">
      <Text fw={600}>
        {title}{" "}
        <Text span c="dimmed">
          · {rows.length}
        </Text>
      </Text>
      {rows.length > 0 ? <AchievementsTable rows={rows} /> : <Text c="dimmed">nenhuma</Text>}
    </Stack>
  );
}

// OverallProgress: barra geral do jogador no jogo (spec 040). O percentual é
// ponderado pelo alvo (Σ progresso / Σ target): 1/3600 pesa mais que 1/3. O
// numeral "X de N desbloqueadas" fica junto como o fato bruto, sem ponderação.
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
    <Stack gap={4}>
      <Group justify="space-between">
        <Text size="sm" fw={600}>
          Progresso geral
        </Text>
        <Text size="sm" c="dimmed">
          {unlocked} de {data.length} desbloqueadas · {pctLabel}% do progresso total
        </Text>
      </Group>
      <Tooltip label="Ponderado pelo alvo de cada conquista: unidades de progresso cumpridas sobre o total de unidades do catálogo.">
        <Progress value={pct} color={unlocked === data.length ? "green" : "blue"} size="md" />
      </Tooltip>
    </Stack>
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

  // Partição da visão "Por estado" (spec 040): conquistadas, em progresso
  // (progresso parcial) e bloqueadas (sem progresso), nesta ordem.
  const groups = useMemo(() => {
    const list = data ?? [];
    return {
      unlocked: list.filter((a) => a.unlocked),
      inProgress: list.filter((a) => !a.unlocked && a.progress > 0),
      locked: list.filter((a) => !a.unlocked && a.progress === 0),
    };
  }, [data]);

  if (!gameId) return <Text c="dimmed">Selecione um jogo no topo para ver o progresso do jogador.</Text>;

  return (
    <Stack>
      <Group justify="flex-end" gap="xs">
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

      {loading && !data && <Loader />}
      {!!error && <Alert color="red">Falha ao carregar (achievements indisponível?).</Alert>}
      {data && data.length === 0 && <Text c="dimmed">Nenhum progresso para este jogador neste jogo.</Text>}
      {data && data.length > 0 && (
        <>
          <OverallProgress data={data} />
          {view === "table" ? (
            <AchievementsTable rows={data} />
          ) : (
            <>
              <GroupSection title="Conquistadas" rows={groups.unlocked} />
              <GroupSection title="Em progresso" rows={groups.inProgress} />
              <GroupSection title="Bloqueadas" rows={groups.locked} />
            </>
          )}
        </>
      )}
    </Stack>
  );
}

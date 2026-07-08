import { Badge, Group, Stack, Table, Text } from "@mantine/core";
import type { GameSession, MatchResult, SessionPlayer } from "../lib/api";
import { PlayerAvatar } from "./PlayerAvatar";
import { formatDuration } from "../lib/format";

// Blocos do detalhe rico da sessão (session spec 018), compartilhados entre a
// aba Sessions do jogo e o board Rooms: params do modo (snapshot ADR-0024),
// formação por time e resultado persistido. Cada bloco só aparece quando o
// dado existe — sessões antigas/legadas seguem com o expand de sempre.
export function SessionMatchDetail({ session: s }: { session: GameSession }) {
  const handleOf = (id: string) => s.players.find((p) => p.id === id)?.handle || "";

  return (
    <>
      {s.modeParams && Object.keys(s.modeParams).length > 0 && (
        <Stack gap={2}>
          <Group gap={6} wrap="nowrap">
            <Text size="xs" c="dimmed" fw={600}>
              PARAMS DO MODO
            </Text>
            {s.archetype && (
              <Badge size="xs" variant="light" color="grape">
                {s.archetype}
              </Badge>
            )}
            {s.profileHash && (
              <Text ff="monospace" size="xs" c="dimmed">
                snapshot {s.profileHash}
              </Text>
            )}
          </Group>
          <Group gap={6} wrap="wrap">
            {Object.entries(s.modeParams).map(([k, v]) => (
              <Badge key={k} size="sm" variant="outline" color="gray" tt="none">
                {k}: {typeof v === "object" ? JSON.stringify(v) : String(v)}
              </Badge>
            ))}
          </Group>
        </Stack>
      )}

      {(s.teams?.length ?? 0) > 0 && (
        <Stack gap={2}>
          <Text size="xs" c="dimmed" fw={600}>
            FORMAÇÃO
          </Text>
          <Group gap="xl" wrap="wrap">
            {s.teams!.map((team, i) => (
              <Group key={team.teamId} gap={6} wrap="nowrap">
                <Badge size="sm" variant="light" color={TEAM_COLORS[i % TEAM_COLORS.length]}>
                  {team.teamId}
                </Badge>
                <Group gap="sm" wrap="wrap">
                  {team.players.map((id) => (
                    <Group key={id} gap={4} wrap="nowrap">
                      <PlayerAvatar seed={id} size={18} />
                      <Text size="xs">{handleOf(id) || shortId(id)}</Text>
                    </Group>
                  ))}
                </Group>
              </Group>
            ))}
          </Group>
        </Stack>
      )}

      {s.result && (
        <Stack gap={2}>
          <Text size="xs" c="dimmed" fw={600}>
            RESULTADO
          </Text>
          <ResultBody result={s.result} handleOf={handleOf} players={s.players} />
        </Stack>
      )}
    </>
  );
}

const TEAM_COLORS = ["blue", "red", "teal", "orange"];

const OUTCOME_META: Record<string, { label: string; color: string }> = {
  win: { label: "vitória", color: "green" },
  loss: { label: "derrota", color: "red" },
  draw: { label: "empate", color: "yellow" },
};

function ResultBody({
  result,
  handleOf,
  players,
}: {
  result: MatchResult;
  handleOf: (id: string) => string;
  players: SessionPlayer[];
}) {
  if (result.kind === "win_loss_draw" && result.winLossDraw) {
    return (
      <Table withTableBorder striped w="auto">
        <Table.Thead>
          <Table.Tr>
            <Table.Th>Player</Table.Th>
            <Table.Th>Time</Table.Th>
            <Table.Th>Desfecho</Table.Th>
            <Table.Th>Score</Table.Th>
            <Table.Th>Stats</Table.Th>
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {result.winLossDraw.map((p) => (
            <Table.Tr key={p.playerId}>
              <Table.Td>
                <PlayerCell id={p.playerId} handle={handleOf(p.playerId)} />
              </Table.Td>
              <Table.Td>
                {p.teamId ? (
                  <Text ff="monospace" size="xs">
                    {p.teamId}
                  </Text>
                ) : (
                  <Text size="xs" c="dimmed">
                    —
                  </Text>
                )}
              </Table.Td>
              <Table.Td>
                <OutcomeBadge outcome={p.outcome} />
              </Table.Td>
              <Table.Td>
                <Text size="xs" fw={600}>
                  {p.score}
                </Text>
              </Table.Td>
              <Table.Td>
                <StatsText stats={p.stats} />
              </Table.Td>
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>
    );
  }

  if (result.kind === "placement" && result.placement) {
    return (
      <Table withTableBorder striped w="auto">
        <Table.Thead>
          <Table.Tr>
            <Table.Th>#</Table.Th>
            <Table.Th>Player</Table.Th>
            <Table.Th>Time</Table.Th>
            <Table.Th>Sobreviveu</Table.Th>
            <Table.Th>Stats</Table.Th>
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {result.placement.map((e) => (
            <Table.Tr key={e.playerId}>
              <Table.Td>
                <Text size="xs" fw={600}>
                  {e.placement}º
                </Text>
              </Table.Td>
              <Table.Td>
                <PlayerCell id={e.playerId} handle={handleOf(e.playerId)} />
              </Table.Td>
              <Table.Td>
                <Text ff="monospace" size="xs">
                  {e.teamId || "—"}
                </Text>
              </Table.Td>
              <Table.Td>
                <Text size="xs">{formatDuration(e.survivedMs)}</Text>
              </Table.Td>
              <Table.Td>
                <StatsText stats={e.stats} />
              </Table.Td>
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>
    );
  }

  if (result.kind === "collective" && result.collective) {
    const ok = result.collective.outcome === "MISSION_SUCCESS";
    return (
      <Stack gap={4}>
        <Badge variant="light" color={ok ? "green" : "red"} w="fit-content">
          {ok ? "missão cumprida" : "missão falhou"}
        </Badge>
        <Group gap="md" wrap="wrap">
          {(result.collective.players ?? []).map((p) => (
            <Group key={p.playerId} gap={4} wrap="nowrap">
              <PlayerCell id={p.playerId} handle={handleOf(p.playerId)} />
              <StatsText stats={p.stats} />
            </Group>
          ))}
        </Group>
      </Stack>
    );
  }

  // kind desconhecido (arquétipo futuro): não quebra — mostra o cru.
  void players;
  return (
    <Text ff="monospace" size="xs" c="dimmed">
      {JSON.stringify(result)}
    </Text>
  );
}

function PlayerCell({ id, handle }: { id: string; handle: string }) {
  return (
    <Group gap={4} wrap="nowrap">
      <PlayerAvatar seed={id} size={18} />
      <Text size="xs">{handle || shortId(id)}</Text>
    </Group>
  );
}

function OutcomeBadge({ outcome }: { outcome: string }) {
  const meta = OUTCOME_META[outcome] ?? { label: outcome, color: "gray" };
  return (
    <Badge size="sm" variant="light" color={meta.color}>
      {meta.label}
    </Badge>
  );
}

function StatsText({ stats }: { stats?: Record<string, number> }) {
  const entries = Object.entries(stats ?? {});
  if (entries.length === 0)
    return (
      <Text size="xs" c="dimmed">
        —
      </Text>
    );
  return (
    <Text ff="monospace" size="xs">
      {entries.map(([k, v]) => `${k}: ${v}`).join(" · ")}
    </Text>
  );
}

function shortId(id: string) {
  return id.length > 12 ? id.slice(0, 12) + "…" : id;
}

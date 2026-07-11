import {
  Alert,
  Badge,
  Box,
  Divider,
  Group,
  Paper,
  SimpleGrid,
  Stack,
  Table,
  Text,
  Tooltip,
} from "@mantine/core";
import type { ReactNode } from "react";
import type { Icon } from "@tabler/icons-react";
import { IconCrown, IconTargetArrow, IconTrophy } from "@tabler/icons-react";
import type { GameSession, PlacementEntry, PlayerOutcome, SessionPlayer } from "../lib/api";
import { PlayerAvatar } from "./PlayerAvatar";
import { GuestBadge } from "./GuestBadge";
import { ARCHETYPE_META } from "../lib/archetypeMeta";
import { formatDuration } from "../lib/format";

// Detalhe rico da sessão (session specs 018/019), compartilhado entre a aba
// Sessions do jogo e o board Rooms. O painel é específico por arquétipo:
//   queue_arena teams  → grid de cards por time (placar agregado + vencedor)
//   queue_arena ffa    → tabela ranqueada por pontuação
//   battle_royale      → tabela de colocação (pódio destacado, squads)
//   coop_mission       → banner do desfecho coletivo + stats por jogador
//   host_world_dropin  → cards por papel (host/co-op/invasor, vagas livres)
// Ele também exibe o roster: as páginas não renderizam mais um bloco PLAYERS
// separado — cada jogador aparece uma única vez, no contexto da partida.
// Sessão sem snapshot/resultado (legada) cai no roster simples.
export function SessionMatchDetail({ session: s }: { session: GameSession }) {
  const handleOf = (id: string) => s.players.find((p) => p.id === id)?.handle || "";

  return (
    <>
      <RulesBlock session={s} />
      <MatchPanel session={s} handleOf={handleOf} />
    </>
  );
}

// rótulos e cores dos papéis do host_world_dropin (compartilhado com o board
// Rooms, que usa a mesma leitura na linha da tabela).
export const ROLE_META: Record<string, { label: string; color: string }> = {
  coop: { label: "co-op", color: "teal" },
  invader: { label: "invasor", color: "red" },
  host: { label: "host", color: "blue" },
};

// 10 cores (config de times/squads não tem teto duro — MultiplayerSettings
// só valida mínimo — então o array cobre uma faixa mais folgada antes de
// repetir cor por módulo).
const TEAM_COLORS = ["blue", "red", "teal", "orange", "grape", "cyan", "pink", "lime", "indigo", "yellow"];

const OUTCOME_META: Record<string, { label: string; color: string }> = {
  win: { label: "vitória", color: "green" },
  loss: { label: "derrota", color: "red" },
  draw: { label: "empate", color: "yellow" },
};

const RESPAWN_LABELS: Record<string, string> = {
  one_life: "Uma vida (BR clássico)",
  teammate_revive: "Revive por aliado",
  arena_1v1: "Arena 1v1 (Gulag)",
  reboot_timer: "Reboot com janela",
};

// ---------------------------------------------------------------------------
// Regras do modo: o params_json cru do snapshot traduzido para o vocabulário
// da tela de Configurações → Multiplayer (mesmos rótulos que o operador usou
// ao cadastrar). Arquétipo desconhecido cai em chave:valor genérico.

type Rule = { label: string; value: string };

function ruleFields(archetype: string | undefined, params?: Record<string, unknown>): Rule[] {
  if (!params || Object.keys(params).length === 0) return [];
  const num = (k: string) => (typeof params[k] === "number" ? (params[k] as number) : 0);
  const str = (k: string) => (typeof params[k] === "string" ? (params[k] as string) : "");
  const bool = (k: string) => params[k] === true;
  const list = (k: string) => (Array.isArray(params[k]) ? (params[k] as unknown[]).map(String) : []);
  const rules: Rule[] = [];

  switch (archetype) {
    case "queue_arena": {
      rules.push({
        label: "Estrutura",
        value:
          str("structure") === "teams"
            ? `Times · ${num("teamCount")} × ${num("teamSize")}`
            : `FFA · ${num("playerCount")} jogadores`,
      });
      const wc = str("winCondition");
      rules.push({
        label: "Critério",
        value:
          wc === "first_to_score"
            ? `Primeiro a ${num("scoreTarget")} pontos`
            : wc === "highest_at_time"
              ? `Maior pontuação em ${formatDuration(num("durationSeconds") * 1000)}`
              : wc === "objective"
                ? "Objetivo"
                : wc === "elimination"
                  ? "Eliminação"
                  : wc || "—",
      });
      rules.push({
        label: "Formato",
        value:
          str("matchStructure") === "best_of_rounds"
            ? `Melhor de N · ${num("roundsToWin")} rounds p/ vencer`
            : "Partida única",
      });
      rules.push({ label: "Empate", value: bool("allowDraw") ? "permitido" : "não permitido" });
      break;
    }
    case "battle_royale": {
      rules.push({ label: "Lobby", value: `${num("lobbySize")} jogadores` });
      const ts = num("teamSize");
      rules.push({ label: "Formato", value: ts <= 1 ? "Solo/FFA" : `Squads de ${ts}` });
      const rm = str("respawnMode");
      rules.push({ label: "Respawn", value: RESPAWN_LABELS[rm] ?? rm ?? "—" });
      break;
    }
    case "coop_mission": {
      rules.push({ label: "Time", value: `${num("teamSize")} jogadores` });
      rules.push({ label: "Bot-fill", value: bool("botfillEnabled") ? "ligado" : "desligado" });
      if (list("missions").length > 0) rules.push({ label: "Missões", value: list("missions").join(", ") });
      if (list("difficulties").length > 0)
        rules.push({ label: "Dificuldades", value: list("difficulties").join(", ") });
      break;
    }
    case "host_world_dropin": {
      const roles: string[] = [];
      if (list("enabledRoles").includes("coop")) roles.push(`co-op ×${num("coopSlots")}`);
      if (list("enabledRoles").includes("invader")) roles.push(`invasor ×${num("invaderSlots")}`);
      rules.push({ label: "Papéis externos", value: roles.join(" · ") || "—" });
      const filters = [
        bool("levelRangeEnabled") && "faixa de nível",
        bool("weaponTierEnabled") && "tier de equipamento",
        bool("passwordEnabled") && "senha de sessão",
      ].filter((f): f is string => typeof f === "string");
      rules.push({ label: "Matchmaking", value: filters.length > 0 ? filters.join(" · ") : "aberto" });
      rules.push({ label: "Invasão", value: bool("invasionEnabled") ? "automática" : "desligada" });
      break;
    }
    default:
      // arquétipo futuro: não quebra — mostra chave:valor cru.
      for (const [k, v] of Object.entries(params))
        rules.push({ label: k, value: typeof v === "object" ? JSON.stringify(v) : String(v) });
      return rules;
  }

  if (list("scoringStats").length > 0) rules.push({ label: "Stats", value: list("scoringStats").join(", ") });
  return rules;
}

function RulesBlock({ session: s }: { session: GameSession }) {
  const meta = s.archetype
    ? (ARCHETYPE_META as Record<string, { label: string; icon: Icon } | undefined>)[s.archetype]
    : undefined;
  const rules = ruleFields(s.archetype, s.modeParams);
  if (!s.archetype && rules.length === 0) return null;

  return (
    <Stack gap={4}>
      <Group gap={6} wrap="nowrap">
        <SectionLabel>REGRAS DO MODO</SectionLabel>
        {s.archetype && (
          <Badge size="xs" variant="light" color="gray" leftSection={meta ? <meta.icon size={11} /> : undefined}>
            {meta?.label ?? s.archetype}
          </Badge>
        )}
        {s.profileHash && (
          <Text ff="monospace" size="xs" c="dimmed">
            snapshot {s.profileHash}
          </Text>
        )}
      </Group>
      {rules.length > 0 && (
        <Group gap="xl" wrap="wrap">
          {rules.map((r) => (
            <Stack key={r.label} gap={0}>
              <Text size="xs" c="dimmed">
                {r.label}
              </Text>
              <Text size="sm" fw={500}>
                {r.value}
              </Text>
            </Stack>
          ))}
        </Group>
      )}
    </Stack>
  );
}

// ---------------------------------------------------------------------------
// Painel da partida: um layout por arquétipo. Sessões antigas sem snapshot
// dispatcham pelo kind do resultado; sem nada, roster simples.

function MatchPanel({ session: s, handleOf }: { session: GameSession; handleOf: (id: string) => string }) {
  const kind = s.result?.kind;

  if (s.archetype === "host_world_dropin" || s.origin === "host_world")
    return <WorldRoleCards session={s} handleOf={handleOf} />;
  if (s.archetype === "coop_mission" || kind === "collective")
    return <CoopPanel session={s} handleOf={handleOf} />;
  if (s.archetype === "battle_royale" || kind === "placement")
    return <PlacementPanel session={s} handleOf={handleOf} />;
  if ((s.teams?.length ?? 0) > 0) return <TeamCards session={s} handleOf={handleOf} />;
  if (s.archetype === "queue_arena" || kind === "win_loss_draw")
    return <FfaPanel session={s} handleOf={handleOf} />;

  // sessão legada (sem snapshot nem resultado tipado): roster + resultado cru.
  return (
    <>
      <RosterBlock players={s.players} />
      {s.result && (
        <Stack gap={4}>
          <SectionLabel>RESULTADO</SectionLabel>
          <Text ff="monospace" size="xs" c="dimmed">
            {JSON.stringify(s.result)}
          </Text>
        </Stack>
      )}
    </>
  );
}

// queue_arena · teams: grid dinâmico com um card por time — placar agregado no
// topo, troféu no vencedor, linhas por jogador com pontos/stats individuais.
// Partida aberta (sem resultado) mostra só a formação.
function TeamCards({ session: s, handleOf }: { session: GameSession; handleOf: (id: string) => string }) {
  const teams = s.teams ?? [];
  const outcomes = new Map((s.result?.winLossDraw ?? []).map((o) => [o.playerId, o]));
  const inFormation = new Set(teams.flatMap((t) => t.players));
  const extras = (s.result?.winLossDraw ?? []).filter((o) => !inFormation.has(o.playerId));
  const cols = Math.min(Math.max(teams.length, 1), 4);

  return (
    <Stack gap={4}>
      <SectionLabel>{s.result ? "RESULTADO" : "FORMAÇÃO"}</SectionLabel>
      <SimpleGrid cols={{ base: 1, sm: Math.min(cols, 2), lg: cols }} spacing="sm">
        {teams.map((team, i) => {
          const members = team.players.map((id) => ({ id, outcome: outcomes.get(id) }));
          const score = members.reduce((acc, m) => acc + (m.outcome?.score ?? 0), 0);
          const verdict = members.find((m) => m.outcome)?.outcome?.outcome;
          return (
            <Paper
              key={team.teamId}
              withBorder
              radius="md"
              p="sm"
              style={verdict === "win" ? { borderColor: "var(--mantine-color-green-6)" } : undefined}
            >
              <Group justify="space-between" wrap="nowrap">
                <Badge variant="light" color={TEAM_COLORS[i % TEAM_COLORS.length]}>
                  {team.teamId}
                </Badge>
                {s.result && (
                  <Group gap={8} wrap="nowrap">
                    {verdict === "win" && (
                      <Badge size="sm" color="green" leftSection={<IconTrophy size={12} />}>
                        vencedor
                      </Badge>
                    )}
                    {verdict === "draw" && (
                      <Badge size="sm" color="yellow" variant="light">
                        empate
                      </Badge>
                    )}
                    <Text fw={700} size="lg" lh={1}>
                      {score}
                    </Text>
                  </Group>
                )}
              </Group>
              <Divider my="xs" />
              <Stack gap={6}>
                {members.map((m) => (
                  <Group key={m.id} justify="space-between" wrap="nowrap">
                    <PlayerCell id={m.id} handle={handleOf(m.id)} />
                    {m.outcome && (
                      <Group gap={10} wrap="nowrap">
                        <StatsText stats={m.outcome.stats} muted />
                        <Text size="sm" fw={600}>
                          {m.outcome.score}
                        </Text>
                      </Group>
                    )}
                  </Group>
                ))}
              </Stack>
            </Paper>
          );
        })}
      </SimpleGrid>
      {extras.length > 0 && (
        <Text size="xs" c="dimmed">
          fora da formação: {extras.map((o) => handleOf(o.playerId) || shortId(o.playerId)).join(", ")}
        </Text>
      )}
    </Stack>
  );
}

// queue_arena · ffa: tabela ranqueada por pontuação (vencedor primeiro). Sem
// resultado ainda (partida aberta), mostra o roster.
function FfaPanel({ session: s, handleOf }: { session: GameSession; handleOf: (id: string) => string }) {
  const rows = s.result?.winLossDraw;
  if (!rows || rows.length === 0) return <RosterBlock players={s.players} />;

  const sorted = [...rows].sort(
    (a, b) => (b.outcome === "win" ? 1 : 0) - (a.outcome === "win" ? 1 : 0) || Number(b.score - a.score),
  );
  const statCols = statColumns(rows);
  const missing = missingFromResult(s.players, rows);

  return (
    <Stack gap={4}>
      <SectionLabel>RESULTADO</SectionLabel>
      <Table withTableBorder striped w="auto">
        <Table.Thead>
          <Table.Tr>
            <Table.Th>#</Table.Th>
            <Table.Th>Player</Table.Th>
            <Table.Th>Desfecho</Table.Th>
            <Table.Th ta="right">Pontos</Table.Th>
            <StatHeaders cols={statCols} />
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {sorted.map((p, i) => (
            <Table.Tr key={p.playerId}>
              <Table.Td>
                <Text size="xs" c="dimmed">
                  {i + 1}º
                </Text>
              </Table.Td>
              <Table.Td>
                <PlayerCell id={p.playerId} handle={handleOf(p.playerId)} />
              </Table.Td>
              <Table.Td>
                <OutcomeBadge outcome={p.outcome} />
              </Table.Td>
              <Table.Td ta="right">
                <Text size="xs" fw={600}>
                  {p.score}
                </Text>
              </Table.Td>
              <StatCells cols={statCols} stats={p.stats} />
            </Table.Tr>
          ))}
          <MissingRows players={missing} colSpan={4 + statColSpan(statCols)} />
        </Table.Tbody>
      </Table>
    </Stack>
  );
}

// battle_royale: tabela de colocação com o pódio destacado; coluna de squad
// (badge colorido) quando há times. Sem resultado, roster do lobby.
function PlacementPanel({ session: s, handleOf }: { session: GameSession; handleOf: (id: string) => string }) {
  const rows = s.result?.placement;
  if (!rows || rows.length === 0) return <RosterBlock players={s.players} />;

  const sorted = [...rows].sort((a, b) => a.placement - b.placement);
  const squads = [...new Set(sorted.map((e) => e.teamId).filter((t): t is string => !!t))];
  const statCols = statColumns(rows);
  const missing = missingFromResult(s.players, rows);

  return (
    <Stack gap={4}>
      <SectionLabel>RESULTADO</SectionLabel>
      <Table withTableBorder striped w="auto">
        <Table.Thead>
          <Table.Tr>
            <Table.Th>#</Table.Th>
            <Table.Th>Player</Table.Th>
            {squads.length > 0 && <Table.Th>Squad</Table.Th>}
            <Table.Th>Sobreviveu</Table.Th>
            <StatHeaders cols={statCols} />
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {sorted.map((e) => (
            <Table.Tr key={e.playerId}>
              <Table.Td>
                <PodiumBadge placement={e.placement} />
              </Table.Td>
              <Table.Td>
                <PlayerCell id={e.playerId} handle={handleOf(e.playerId)} />
              </Table.Td>
              {squads.length > 0 && (
                <Table.Td>
                  {e.teamId ? (
                    <Badge
                      size="xs"
                      variant="light"
                      color={TEAM_COLORS[squads.indexOf(e.teamId) % TEAM_COLORS.length]}
                    >
                      {e.teamId}
                    </Badge>
                  ) : (
                    <Text size="xs" c="dimmed">
                      —
                    </Text>
                  )}
                </Table.Td>
              )}
              <Table.Td>
                <Text size="xs">{formatDuration(e.survivedMs)}</Text>
              </Table.Td>
              <StatCells cols={statCols} stats={e.stats} />
            </Table.Tr>
          ))}
          <MissingRows
            players={missing}
            colSpan={3 + (squads.length > 0 ? 1 : 0) + statColSpan(statCols)}
          />
        </Table.Tbody>
      </Table>
    </Stack>
  );
}

// coop_mission: o desfecho é COLETIVO — banner grande (cumprida/falhou) e uma
// tabela única do time com as stats individuais (kills, revives…).
function CoopPanel({ session: s, handleOf }: { session: GameSession; handleOf: (id: string) => string }) {
  const col = s.result?.collective;
  const rows = col?.players ?? [];
  const statCols = statColumns(rows);
  const ok = col?.outcome === "MISSION_SUCCESS";
  const label = ok ? "Missão cumprida" : col?.outcome === "MISSION_FAILURE" ? "Missão falhou" : col?.outcome;

  return (
    <Stack gap={4}>
      <SectionLabel>{col ? "RESULTADO DA MISSÃO" : "TIME"}</SectionLabel>
      {col && (
        <Alert
          variant="light"
          color={ok ? "green" : "red"}
          icon={<IconTargetArrow size={18} />}
          p="xs"
          w="fit-content"
        >
          <Text size="sm" fw={600}>
            {label}
          </Text>
        </Alert>
      )}
      {rows.length > 0 ? (
        <Table withTableBorder striped w="auto">
          <Table.Thead>
            <Table.Tr>
              <Table.Th>Player</Table.Th>
              <StatHeaders cols={statCols} />
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {rows.map((p) => (
              <Table.Tr key={p.playerId}>
                <Table.Td>
                  <PlayerCell id={p.playerId} handle={handleOf(p.playerId)} />
                </Table.Td>
                <StatCells cols={statCols} stats={p.stats} />
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      ) : (
        <RosterBlock players={s.players} title={null} />
      )}
    </Stack>
  );
}

// host_world_dropin: mundo não é partida — cards por papel: o host (dono do
// mundo) e um card por slot (co-op/invasor) com ocupantes e vagas livres.
function WorldRoleCards({ session: s, handleOf }: { session: GameSession; handleOf: (id: string) => string }) {
  const slots = s.roleSlots ?? [];
  const assigned = new Set([s.hostPlayerId, ...slots.flatMap((sl) => sl.occupants ?? [])]);
  const others = s.players.filter((p) => p.id !== s.hostPlayerId && !assigned.has(p.id));

  return (
    <Stack gap={4}>
      <SectionLabel>MUNDO</SectionLabel>
      <SimpleGrid cols={{ base: 1, sm: 2, lg: Math.min(1 + slots.length, 4) }} spacing="sm">
        <Paper withBorder radius="md" p="sm" style={{ borderColor: "var(--mantine-color-blue-6)" }}>
          <Group justify="space-between" wrap="nowrap">
            <Badge variant="light" color="blue" leftSection={<IconCrown size={12} />}>
              host
            </Badge>
            <Text size="xs" c="dimmed">
              {s.hostPlayerId ? "1/1" : "0/1"}
            </Text>
          </Group>
          <Divider my="xs" />
          {s.hostPlayerId ? (
            <PlayerCell id={s.hostPlayerId} handle={handleOf(s.hostPlayerId)} />
          ) : (
            <Text size="xs" c="dimmed">
              —
            </Text>
          )}
        </Paper>
        {slots.map((slot) => {
          const meta = ROLE_META[slot.role] ?? { label: slot.role, color: "gray" };
          const occ = slot.occupants ?? [];
          const unnamed = Math.max(0, slot.occupied - occ.length); // payload antigo sem occupants
          const free = Math.max(0, slot.capacity - slot.occupied);
          const full = slot.capacity > 0 && free === 0;
          return (
            <Paper
              key={slot.role}
              withBorder
              radius="md"
              p="sm"
              style={full ? { borderColor: "var(--mantine-color-nominal-6)" } : undefined}
            >
              <Group justify="space-between" wrap="nowrap">
                <Badge variant="light" color={meta.color}>
                  {meta.label}
                </Badge>
                {full ? (
                  <Badge size="xs" variant="light" color="nominal">
                    cheio
                  </Badge>
                ) : (
                  <Text size="xs" c="dimmed">
                    {slot.occupied}/{slot.capacity}
                  </Text>
                )}
              </Group>
              <Divider my="xs" />
              <Stack gap={6}>
                {occ.map((id) => (
                  <PlayerCell key={id} id={id} handle={handleOf(id)} />
                ))}
                {Array.from({ length: unnamed }, (_, i) => (
                  <Text key={`unnamed-${i}`} size="xs" c="dimmed">
                    ocupado
                  </Text>
                ))}
                {Array.from({ length: free }, (_, i) => (
                  <Box
                    key={`free-${i}`}
                    px={6}
                    py={2}
                    style={{ border: "1px dashed var(--mantine-color-default-border)", borderRadius: 4 }}
                  >
                    <Text size="xs" c="dimmed" fs="italic">
                      vaga livre
                    </Text>
                  </Box>
                ))}
              </Stack>
            </Paper>
          );
        })}
      </SimpleGrid>
      {others.length > 0 && <RosterBlock players={others} title="OUTROS PARTICIPANTES" />}
    </Stack>
  );
}

// ---------------------------------------------------------------------------
// Peças compartilhadas.

function SectionLabel({ children }: { children: ReactNode }) {
  return (
    <Text size="xs" c="dimmed" fw={600}>
      {children}
    </Text>
  );
}

// Roster simples: fallback de partidas abertas/legadas (o antigo bloco PLAYERS
// das páginas, agora dentro do painel).
function RosterBlock({ players, title = "PLAYERS" }: { players: SessionPlayer[]; title?: string | null }) {
  return (
    <Stack gap={4}>
      {title && <SectionLabel>{title}</SectionLabel>}
      <Group gap="md" wrap="wrap">
        {players.map((p) => (
          <Group key={p.id} gap={6} wrap="nowrap">
            <PlayerAvatar seed={p.id} size={24} />
            <Stack gap={0}>
              <Group gap={4} wrap="nowrap">
                <Text size="xs" fw={500}>
                  {p.handle || "—"}
                </Text>
                <GuestBadge playerId={p.id} />
              </Group>
              <Text ff="monospace" size="xs" c="dimmed">
                {p.id}
              </Text>
            </Stack>
          </Group>
        ))}
      </Group>
    </Stack>
  );
}

// Célula compacta de jogador (cards/tabelas): o id completo fica no tooltip.
function PlayerCell({ id, handle }: { id: string; handle: string }) {
  return (
    <Tooltip label={id} withArrow position="top-start" openDelay={300}>
      <Group gap={6} wrap="nowrap">
        <PlayerAvatar seed={id} size={20} />
        <Text size="xs" fw={500}>
          {handle || shortId(id)}
        </Text>
        <GuestBadge playerId={id} />
      </Group>
    </Tooltip>
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

// PodiumBadge destaca o pódio do battle_royale (1º ouro, 2º prata, 3º bronze).
function PodiumBadge({ placement }: { placement: number }) {
  const color = placement === 1 ? "yellow" : placement === 2 ? "gray" : placement === 3 ? "orange" : null;
  if (!color)
    return (
      <Text size="xs" c="dimmed">
        {placement}º
      </Text>
    );
  return (
    <Badge size="sm" variant="light" color={color} leftSection={placement === 1 ? <IconTrophy size={12} /> : null}>
      {placement}º
    </Badge>
  );
}

// Colunas dinâmicas de stats: até 4 chaves distintas viram colunas próprias;
// mais que isso (ou nenhuma) cai numa coluna única "Stats" agregada.
function statColumns(entries: { stats?: Record<string, number> }[]): string[] | null {
  const keys = new Set<string>();
  for (const e of entries) for (const k of Object.keys(e.stats ?? {})) keys.add(k);
  if (keys.size === 0) return [];
  const sorted = [...keys].sort();
  return sorted.length <= 4 ? sorted : null;
}

function statColSpan(cols: string[] | null): number {
  return cols === null ? 1 : cols.length;
}

function StatHeaders({ cols }: { cols: string[] | null }) {
  if (cols === null) return <Table.Th>Stats</Table.Th>;
  return (
    <>
      {cols.map((k) => (
        <Table.Th key={k} ta="right">
          {k}
        </Table.Th>
      ))}
    </>
  );
}

function StatCells({ cols, stats }: { cols: string[] | null; stats?: Record<string, number> }) {
  if (cols === null)
    return (
      <Table.Td>
        <StatsText stats={stats} />
      </Table.Td>
    );
  return (
    <>
      {cols.map((k) => (
        <Table.Td key={k} ta="right">
          <Text ff="monospace" size="xs">
            {stats?.[k] ?? "—"}
          </Text>
        </Table.Td>
      ))}
    </>
  );
}

// Linhas de jogadores da sessão que ficaram fora do resultado reportado
// (defensivo — o server normalmente reporta todo mundo, spec 009).
function MissingRows({ players, colSpan }: { players: SessionPlayer[]; colSpan: number }) {
  return (
    <>
      {players.map((p) => (
        <Table.Tr key={p.id}>
          <Table.Td />
          <Table.Td>
            <PlayerCell id={p.id} handle={p.handle} />
          </Table.Td>
          <Table.Td colSpan={Math.max(1, colSpan - 2)}>
            <Text size="xs" c="dimmed" fs="italic">
              fora do resultado
            </Text>
          </Table.Td>
        </Table.Tr>
      ))}
    </>
  );
}

function missingFromResult(
  players: SessionPlayer[],
  entries: (PlayerOutcome | PlacementEntry)[],
): SessionPlayer[] {
  const inResult = new Set(entries.map((e) => e.playerId));
  return players.filter((p) => !inResult.has(p.id));
}

function StatsText({ stats, muted }: { stats?: Record<string, number>; muted?: boolean }) {
  const entries = Object.entries(stats ?? {});
  if (entries.length === 0)
    return (
      <Text size="xs" c="dimmed">
        —
      </Text>
    );
  return (
    <Text ff="monospace" size="xs" c={muted ? "dimmed" : undefined}>
      {entries.map(([k, v]) => `${k}: ${v}`).join(" · ")}
    </Text>
  );
}

function shortId(id: string) {
  return id.length > 12 ? id.slice(0, 12) + "…" : id;
}

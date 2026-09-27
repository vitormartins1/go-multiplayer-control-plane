import { Fragment, type ReactNode } from "react";
import { Badge, Box, Button, Collapse, Group, Stack, Table, Text } from "@mantine/core";
import { IconChevronDown, IconChevronUp } from "@tabler/icons-react";
import type { Game, GameSession, RoleSlot } from "../lib/api";
import { archetypeMetaOf } from "../lib/archetypeMeta";
import { closeReasonLabel, formatDuration, shortId, truncate } from "../lib/format";
import { hostingOf } from "../lib/hosting";
import type { SortDir, SortKey } from "../lib/sessionSort";
import { sessionStateLabel } from "../lib/vocab";
import { GameCover } from "./GameCover";
import { HeartbeatBadge, HostingCell, P2PTrustNote } from "./Hosting";
import { PlayersSummary } from "./PlayersSummary";
import { SessionAvatar } from "./SessionAvatar";
import { ROLE_META, SessionMatchDetail } from "./SessionMatchDetail";

// SessionTable é a tabela de sessões do console (admin-ui/092): a mesma linha e
// o mesmo painel expandido servem o board global (Rooms, com jogo/arquétipo,
// ordenação e "Encerrar") e a aba Sessões do jogo (sem essas colunas). Antes
// eram duas cópias quase iguais que divergiam a cada mudança.

export interface SessionSort {
  by: SortKey;
  dir: SortDir;
  onSort: (key: SortKey) => void;
}

export interface SessionTableProps {
  rows: GameSession[];
  // colunas opcionais: jogo (board global) e arquétipo (a aba do jogo já sabe)
  showGame?: boolean;
  showArchetype?: boolean;
  gamesById?: Map<string, Game>;
  // sem sort: cabeçalhos fixos, na ordem recebida
  sort?: SessionSort;
  // rótulos extras do modo por sessão (lobby de battle_royale, time do co-op)
  modeExtra?: (s: GameSession) => ReactNode;
  // capacidade para o resumo de jogadores (lobby/time)
  capacityOf?: (s: GameSession) => number | undefined;
  expandedId: string | null;
  onToggle: (sessionId: string) => void;
  // com onClose, a sessão aberta ganha o botão "Encerrar"
  onClose?: (s: GameSession) => void;
}

export function SessionTable(props: SessionTableProps) {
  const { rows, showGame, showArchetype, sort, onClose } = props;
  const th = (label: string, key: SortKey) =>
    sort ? <SortableTh label={label} sortKey={key} sort={sort} /> : <Table.Th>{label}</Table.Th>;
  const colSpan = 7 + (showGame ? 1 : 0) + (showArchetype ? 1 : 0) + (onClose ? 1 : 0);

  return (
    <Table.ScrollContainer minWidth={showGame ? 720 : 640}>
      <Table striped withTableBorder>
        <Table.Thead>
          <Table.Tr>
            <Table.Th />
            {showGame && th("Jogo", "game")}
            {showArchetype && th("Arquétipo", "archetype")}
            {th("Modo", "mode")}
            <Table.Th>Sessão</Table.Th>
            {th("Estado", "state")}
            {th("Players", "players")}
            <Table.Th>Hospedagem</Table.Th>
            {th("Tempos", "time")}
            {onClose && <Table.Th />}
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {rows.map((s) => (
            <Fragment key={s.sessionId}>
              <SessionRow {...props} session={s} expanded={props.expandedId === s.sessionId} />
              {props.expandedId === s.sessionId && (
                <Table.Tr>
                  <Table.Td colSpan={colSpan} p={0}>
                    <Collapse in>
                      <Box p="md" style={{ borderTop: "1px solid var(--mantine-color-default-border)" }}>
                        <SessionDetailPanel session={s} />
                      </Box>
                    </Collapse>
                  </Table.Td>
                </Table.Tr>
              )}
            </Fragment>
          ))}
        </Table.Tbody>
      </Table>
    </Table.ScrollContainer>
  );
}

function SessionRow({
  session: s,
  expanded,
  showGame,
  showArchetype,
  gamesById,
  modeExtra,
  capacityOf,
  onToggle,
  onClose,
}: SessionTableProps & { session: GameSession; expanded: boolean }) {
  const game = s.gameId ? gamesById?.get(s.gameId) : undefined;
  const meta = archetypeMetaOf(s.archetype);
  return (
    <Table.Tr
      style={{ cursor: "pointer" }}
      bg={expanded ? "var(--mantine-color-default-hover)" : undefined}
      onClick={() => onToggle(s.sessionId)}
    >
      <Table.Td>
        {game ? (
          <Box style={{ position: "relative", width: 28, height: 28 }}>
            <GameCover game={game} variant="mini" />
            <Box
              style={{
                position: "absolute",
                bottom: -4,
                right: -4,
                borderRadius: 8,
                boxShadow: "0 0 0 2px var(--mantine-color-body)",
              }}
            >
              <SessionAvatar seed={s.profileHash || s.sessionId} size={16} />
            </Box>
          </Box>
        ) : (
          <SessionAvatar seed={s.profileHash || s.sessionId} size={28} />
        )}
      </Table.Td>
      {showGame && (
        <Table.Td>
          <Text size="sm" fw={500}>
            {s.gameName || truncate(s.gameId || "—")}
          </Text>
        </Table.Td>
      )}
      {showArchetype && (
        <Table.Td>
          {meta ? (
            <Badge size="xs" variant="light" color="gray" leftSection={<meta.icon size={10} />}>
              {meta.label}
            </Badge>
          ) : (
            <Text size="sm" c="dimmed">
              —
            </Text>
          )}
        </Table.Td>
      )}
      <Table.Td>
        <Stack gap={2} align="flex-start">
          <Badge
            size="xs"
            variant="light"
            color="gray"
            leftSection={!showArchetype && meta ? <meta.icon size={10} /> : undefined}
          >
            {s.modeId || "default"}
          </Badge>
          {modeExtra?.(s)}
        </Stack>
      </Table.Td>
      <Table.Td>
        <Text ff="monospace" size="sm">
          {shortId(s.matchId || s.sessionId)}
        </Text>
      </Table.Td>
      <Table.Td>
        <Stack gap={4} align="flex-start">
          <Badge color={s.state === "open" ? "nominal" : "gray"} variant="light">
            {sessionStateLabel(s.state, s.closeReason)}
          </Badge>
          <HeartbeatBadge session={s} />
        </Stack>
      </Table.Td>
      <Table.Td>
        {s.origin === "host_world" && s.roleSlots?.length ? (
          <RoleSlots slots={s.roleSlots} />
        ) : (
          // resumo compacto (034): a lista completa vive no painel expandido
          <PlayersSummary players={s.players} capacity={capacityOf?.(s)} />
        )}
      </Table.Td>
      <Table.Td>
        <HostingCell session={s} />
      </Table.Td>
      <Table.Td>
        {s.state === "open" ? (
          <Text size="sm">há {formatDuration(Date.now() - Date.parse(s.openedAt))}</Text>
        ) : (
          <Text size="sm">
            durou {s.closedAt ? formatDuration(Date.parse(s.closedAt) - Date.parse(s.openedAt)) : "?"}
            {s.closedAt ? ` · fechou ${new Date(s.closedAt).toLocaleTimeString("pt-BR")}` : ""}
          </Text>
        )}
      </Table.Td>
      {onClose && (
        <Table.Td onClick={(e) => e.stopPropagation()}>
          {s.state === "open" && (
            <Button size="compact-xs" color="critical" variant="light" onClick={() => onClose(s)}>
              Encerrar
            </Button>
          )}
        </Table.Td>
      )}
    </Table.Tr>
  );
}

// SessionDetailPanel é o painel expandido: identidade, hospedagem, detalhe por
// arquétipo (roster/slots/formação/resultado — spec 019) e ciclo de vida.
export function SessionDetailPanel({ session: s }: { session: GameSession }) {
  const p2p = hostingOf(s) === "p2p";
  const world = s.origin === "host_world";
  return (
    <Stack gap="xs">
      <Group gap="xl" wrap="wrap">
        <SessionAvatar seed={s.profileHash || s.sessionId} size={40} />
        {s.gameName && <Field label="JOGO" value={s.gameName} />}
        <Field label="MODO" value={s.modeId || "default"} mono />
        {/* host_world não tem match: a identidade do mundo é o session id */}
        <Field label={world ? "MUNDO" : "SESSION ID"} value={s.sessionId} mono />
        {!world && s.matchId && <Field label="MATCH ID" value={s.matchId} mono />}
        {p2p ? (
          <>
            <Field label="HOSPEDAGEM" value="P2P — jogo do host" />
            <Field label="CONEXÃO DO HOST" value={s.hostConnection || "—"} mono />
          </>
        ) : (
          <Field label="SERVIDOR" value={s.serverAddress || "—"} mono />
        )}
        {s.lastHeartbeatAt && (
          <Field label="ÚLTIMO HEARTBEAT" value={new Date(s.lastHeartbeatAt).toLocaleTimeString("pt-BR")} />
        )}
        {world && s.hostPlayerId && <Field label="HOST" value={s.hostPlayerId} mono />}
      </Group>

      {p2p && <P2PTrustNote />}

      <SessionMatchDetail session={s} />

      <Group gap="xl" wrap="wrap">
        <Field label="ABERTA EM" value={new Date(s.openedAt).toLocaleString("pt-BR")} />
        {s.closedAt && <Field label="FECHADA EM" value={new Date(s.closedAt).toLocaleString("pt-BR")} />}
        {s.closeReason && <Field label="MOTIVO" value={closeReasonLabel(s.closeReason)} />}
      </Group>
    </Stack>
  );
}

// SortableTh é um cabeçalho clicável com seta quando ativo (spec 060).
function SortableTh({ label, sortKey, sort }: { label: string; sortKey: SortKey; sort: SessionSort }) {
  const active = sort.by === sortKey;
  return (
    <Table.Th style={{ cursor: "pointer", userSelect: "none" }} onClick={() => sort.onSort(sortKey)}>
      <Group gap={4} wrap="nowrap">
        {/* span: herda a tipografia do cabeçalho, igual às colunas fixas */}
        <span style={{ fontWeight: active ? 700 : undefined }}>{label}</span>
        {active && (sort.dir === "asc" ? <IconChevronUp size={12} /> : <IconChevronDown size={12} />)}
      </Group>
    </Table.Th>
  );
}

// RoleSlots mostra a ocupação por papel de um mundo (co-op X/2, invasor Y/1 —
// spec 020); slot cheio fica destacado.
function RoleSlots({ slots }: { slots: RoleSlot[] }) {
  return (
    <Group gap="xs" wrap="wrap">
      {slots.map((slot) => {
        const meta = ROLE_META[slot.role] ?? { label: slot.role, color: "gray" };
        const full = slot.occupied >= slot.capacity;
        return (
          <Badge key={slot.role} color={full ? "gray" : meta.color} variant={full ? "outline" : "light"}>
            {meta.label} {slot.occupied}/{slot.capacity}
          </Badge>
        );
      })}
    </Group>
  );
}

function Field({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <Stack gap={2}>
      <Text size="xs" c="dimmed" fw={600}>
        {label}
      </Text>
      <Text ff={mono ? "monospace" : undefined} size="xs">
        {value}
      </Text>
    </Stack>
  );
}

import { Alert, Box, Button, Group, Loader, Stack, Table, Text, Tooltip } from "@mantine/core";
import { IconRefresh } from "@tabler/icons-react";
import { useNavigate } from "react-router-dom";
import { api, type GamePlayer } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { PlayerAvatar } from "../components/PlayerAvatar";
import { GuestBadge } from "../components/GuestBadge";

// Bolinha de presença sobreposta ao avatar — aparece apenas quando online.
function PresenceDot() {
  return (
    <Box
      style={{
        position: "absolute",
        bottom: -2,
        right: -2,
        width: 10,
        height: 10,
        borderRadius: "50%",
        backgroundColor: "var(--mantine-color-green-5)",
        border: "2px solid var(--mantine-color-body)",
      }}
    />
  );
}

function PlayerRow({
  player: p,
  online,
  onNavigate,
}: {
  player: GamePlayer;
  online: boolean;
  onNavigate: () => void;
}) {
  return (
    <Table.Tr style={{ cursor: "pointer" }} onClick={onNavigate}>
      <Table.Td>
        <Group gap="sm" wrap="nowrap">
          <Box style={{ position: "relative", flexShrink: 0 }}>
            <PlayerAvatar seed={p.playerId} />
            {online && <PresenceDot />}
          </Box>
          <span>{p.handle}</span>
          <GuestBadge playerId={p.playerId} />
        </Group>
      </Table.Td>
      <Table.Td c="dimmed">{p.playerId.slice(0, 8)}…</Table.Td>
      <Table.Td>{new Date(p.firstSeen).toLocaleString("pt-BR")}</Table.Td>
    </Table.Tr>
  );
}

function SectionRow({ label, count, color }: { label: string; count: number; color?: string }) {
  return (
    <Table.Tr>
      <Table.Td colSpan={3} py={6} px="sm" style={{ backgroundColor: "var(--mantine-color-default-hover)" }}>
        <Group gap={6}>
          {color === "green" && (
            <Box
              w={8}
              h={8}
              style={{ borderRadius: "50%", backgroundColor: "var(--mantine-color-green-5)", flexShrink: 0 }}
            />
          )}
          <Text size="xs" fw={600} c={color === "green" ? "green" : "dimmed"}>
            {label} · {count}
          </Text>
        </Group>
      </Table.Td>
    </Table.Tr>
  );
}

// GamePlayers é a player base de UM jogo (presença, ADR-0019): quem entrou no
// jogo ao menos uma vez. Clicar abre o detalhe do jogador. (Aba de GameDetail.)
export function GamePlayers({
  gameId,
  onlinePlayerIds,
}: {
  gameId: string;
  onlinePlayerIds?: Set<string>;
}) {
  const nav = useNavigate();
  const { data, loading, error, reload } = useAsync((s) => api.gamePlayers(gameId, s), [gameId]);

  // Separa players registrados em online (bateram com um session player) e offline.
  // Guests anônimos têm IDs "guest-<uuid>" que não aparecem no game-registry,
  // por isso guestCount > 0 indica sessões com jogadores não listados aqui.
  const online = data?.filter((p) => onlinePlayerIds?.has(p.playerId)) ?? [];
  const offline = data?.filter((p) => !onlinePlayerIds?.has(p.playerId)) ?? [];
  const hasOnline = online.length > 0;
  const guestCount = Math.max(0, (onlinePlayerIds?.size ?? 0) - online.length);

  return (
    <Stack>
      <Group justify="space-between">
        <Text fw={600}>
          Jogadores no jogo{" "}
          {data ? (
            <Text span c="dimmed">
              · {data.length}
            </Text>
          ) : null}
        </Text>
        <Button variant="default" size="xs" leftSection={<IconRefresh size={14} />} onClick={reload}>
          Atualizar
        </Button>
      </Group>

      {loading && !data && <Loader />}
      {!!error && <Alert color="red">game-registry indisponível.</Alert>}
      {data && data.length === 0 && (
        <Text c="dimmed">Nenhum jogador entrou neste jogo ainda (a presença é registrada no login).</Text>
      )}
      {data && data.length > 0 && (
        <Table striped highlightOnHover withTableBorder>
          <Table.Thead>
            <Table.Tr>
              <Table.Th>Handle</Table.Th>
              <Table.Th>Id</Table.Th>
              <Table.Th>Entrou em</Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {hasOnline && <SectionRow label="Online" count={online.length} color="green" />}
            {online.map((p) => (
              <PlayerRow key={p.playerId} player={p} online={true} onNavigate={() => nav(`/players/${p.playerId}`, { state: { fromGameId: gameId } })} />
            ))}
            {hasOnline && offline.length > 0 && <SectionRow label="Outros" count={offline.length} />}
            {offline.map((p) => (
              <PlayerRow key={p.playerId} player={p} online={false} onNavigate={() => nav(`/players/${p.playerId}`, { state: { fromGameId: gameId } })} />
            ))}
          </Table.Tbody>
        </Table>
      )}
      {guestCount > 0 && (
        <Tooltip
          label="Jogadores anônimos entram via LoginAnonymous com um ID efêmero (guest-<uuid>). Eles aparecem em sessões ativas mas não têm conta registrada no game-registry."
          multiline
          w={340}
        >
          <Text size="xs" c="dimmed" style={{ cursor: "default", alignSelf: "flex-start" }}>
            + {guestCount} jogador{guestCount > 1 ? "es" : ""} anônimo{guestCount > 1 ? "s" : ""} em sessão ativa (guest — não listado{guestCount > 1 ? "s" : ""} aqui)
          </Text>
        </Tooltip>
      )}
    </Stack>
  );
}

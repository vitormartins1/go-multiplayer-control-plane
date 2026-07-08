import { Box, Group, Text, Tooltip } from "@mantine/core";
import type { SessionPlayer } from "../lib/api";
import { PlayerAvatar, isGuest } from "./PlayerAvatar";

const MAX_AVATARS = 3;

// PlayersSummary é o resumo compacto da coluna Players nas listagens de sessão
// (034): stack dos 3 primeiros avatares + contador (N ou N/capacidade), com a
// lista completa no tooltip. A lista aberta vive no painel expandido.
export function PlayersSummary({ players, capacity }: { players: SessionPlayer[]; capacity?: number }) {
  const label = players
    .map((p) => `${p.handle || p.id}${isGuest(p.id) ? " (guest)" : ""}`)
    .join("\n");

  return (
    <Tooltip label={label} multiline style={{ whiteSpace: "pre-line" }} withArrow>
      <Group gap={6} wrap="nowrap" style={{ cursor: "default" }}>
        <Group gap={0} wrap="nowrap">
          {players.slice(0, MAX_AVATARS).map((p, i) => (
            <Box
              key={p.id}
              style={{
                marginLeft: i === 0 ? 0 : -8,
                borderRadius: 8,
                // aro da cor do fundo para separar os avatares sobrepostos
                boxShadow: "0 0 0 2px var(--mantine-color-body)",
              }}
            >
              <PlayerAvatar seed={p.id} size={24} />
            </Box>
          ))}
        </Group>
        <Text size="sm" c="dimmed">
          {players.length}
          {capacity ? `/${capacity}` : ""}
        </Text>
      </Group>
    </Tooltip>
  );
}

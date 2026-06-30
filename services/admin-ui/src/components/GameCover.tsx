import { AspectRatio, Box, Text } from "@mantine/core";
import type { Game } from "../lib/api";

// hueFromSlug deriva um matiz estável (0–359) do slug — mesma capa para o mesmo
// jogo sempre. Hash simples (djb2-ish) suficiente para espalhar as cores.
function hueFromSlug(slug: string): number {
  let h = 0;
  for (let i = 0; i < slug.length; i++) h = (h * 31 + slug.charCodeAt(i)) | 0;
  return Math.abs(h) % 360;
}

function initial(name: string): string {
  return (name.trim()[0] ?? "?").toUpperCase();
}

interface GameCoverProps {
  game: Game;
  variant?: "cover" | "mini";
}

// GameCover é a capa procedural de um jogo (placeholder até existir coverUrl
// real no game-registry). "cover" = card 9:16 da biblioteca; "mini" = swatch
// quadrado para o seletor do header.
export function GameCover({ game, variant = "cover" }: GameCoverProps) {
  const hue = hueFromSlug(game.slug);
  const gradient = `linear-gradient(135deg, hsl(${hue}, 58%, 38%), hsl(${(hue + 40) % 360}, 62%, 20%))`;

  if (variant === "mini") {
    return (
      <Box
        w={28}
        h={28}
        style={{
          background: gradient,
          borderRadius: 6,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          flexShrink: 0,
        }}
      >
        <Text fw={700} size="sm" c="white">
          {initial(game.name)}
        </Text>
      </Box>
    );
  }

  return (
    <AspectRatio ratio={9 / 16}>
      <Box
        style={{
          background: gradient,
          borderRadius: 10,
          display: "flex",
          flexDirection: "column",
          justifyContent: "space-between",
          padding: 12,
          overflow: "hidden",
        }}
      >
        <Text fw={800} size="3rem" c="rgba(255,255,255,0.92)" lh={1} style={{ textShadow: "0 2px 8px rgba(0,0,0,0.35)" }}>
          {initial(game.name)}
        </Text>
        <div>
          <Text fw={700} c="white" lineClamp={2} style={{ textShadow: "0 1px 4px rgba(0,0,0,0.5)" }}>
            {game.name}
          </Text>
          <Text size="xs" c="rgba(255,255,255,0.7)">
            {game.slug}
          </Text>
        </div>
      </Box>
    </AspectRatio>
  );
}

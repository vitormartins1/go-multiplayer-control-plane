import { AspectRatio, Box, Text } from "@mantine/core";
import { useMemo } from "react";
import { createAvatar, type Style } from "@dicebear/core";
import { glass, rings, shapes } from "@dicebear/collection";
import type { Game } from "../lib/api";

// Estilos suportados (abstratos/geométricos, não-avatar). O backend manda o
// nome em generatedImageStyle; trocar/adicionar gerador é só mexer aqui.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const STYLES: Record<string, Style<any>> = { shapes, glass, rings };
const DEFAULT_STYLE = "shapes";

// coverSeed: a seed determinística da capa. Cai no slug se o backend não mandou
// generatedImageSeed (registros antigos / respostas sem o campo).
export function coverSeed(game: Game): string {
  return game.generatedImageSeed || game.slug;
}

// coverDataUri gera a imagem determinística (DiceBear) como data-URI SVG. Se no
// futuro houver upload (imageSource="upload" + image), usa a imagem direto.
function coverDataUri(game: Game): string {
  if (game.imageSource === "upload" && game.image) return game.image;
  const style = STYLES[game.generatedImageStyle ?? DEFAULT_STYLE] ?? shapes;
  return createAvatar(style, { seed: coverSeed(game) }).toDataUri();
}

function initial(name: string): string {
  return (name.trim()[0] ?? "?").toUpperCase();
}

interface GameCoverProps {
  game: Game;
  variant?: "cover" | "mini";
}

// GameCover é a capa procedural/determinística de um jogo (003-generated-cover).
// "cover" = card 9:16 da biblioteca (imagem + faixa com o nome); "mini" = swatch
// quadrado para o seletor do header.
export function GameCover({ game, variant = "cover" }: GameCoverProps) {
  const uri = useMemo(() => coverDataUri(game), [game.id, game.generatedImageSeed, game.generatedImageStyle, game.slug, game.image, game.imageSource]);

  if (variant === "mini") {
    return (
      <Box
        w={28}
        h={28}
        style={{
          borderRadius: 6,
          overflow: "hidden",
          flexShrink: 0,
          backgroundImage: `url("${uri}")`,
          backgroundSize: "cover",
          backgroundPosition: "center",
        }}
      />
    );
  }

  return (
    <AspectRatio ratio={9 / 16}>
      <Box
        style={{
          borderRadius: 10,
          overflow: "hidden",
          position: "relative",
          backgroundImage: `url("${uri}")`,
          backgroundSize: "cover",
          backgroundPosition: "center",
        }}
      >
        {/* faixa inferior para leitura do nome sobre a imagem */}
        <Box
          style={{
            position: "absolute",
            insetInline: 0,
            bottom: 0,
            padding: "24px 12px 10px",
            background: "linear-gradient(to top, rgba(0,0,0,0.78), rgba(0,0,0,0))",
          }}
        >
          <Text fw={700} c="white" lineClamp={2} style={{ textShadow: "0 1px 4px rgba(0,0,0,0.6)" }}>
            {game.name}
          </Text>
          <Text size="xs" c="rgba(255,255,255,0.75)">
            {game.slug}
          </Text>
        </Box>
        {/* acessibilidade: inicial não visível, só rótulo */}
        <span style={{ position: "absolute", width: 1, height: 1, overflow: "hidden", clip: "rect(0 0 0 0)" }}>
          {initial(game.name)}
        </span>
      </Box>
    </AspectRatio>
  );
}

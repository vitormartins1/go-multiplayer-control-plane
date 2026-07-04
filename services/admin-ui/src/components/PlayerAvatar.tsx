import { Box } from "@mantine/core";
import { useMemo } from "react";
import { Style, Avatar } from "@dicebear/core";
import adventurerDef from "@dicebear/styles/adventurer.json";

// PlayerAvatar é a "fotinha" determinística de um jogador (006-player-avatars):
// DiceBear estilo humanoide (adventurer), gerada no cliente a partir da seed
// (o playerId — estável e único). Sem persistência, como a capa do jogo.
const adventurerStyle = new Style(adventurerDef);

export function PlayerAvatar({ seed, size = 32 }: { seed: string; size?: number }) {
  const uri = useMemo(
    () =>
      new Avatar(adventurerStyle, {
        seed,
        // paleta de fundo determinística (escolhida pela própria seed) — vira um
        // tile preenchido em vez de cabeça flutuante sobre o tema escuro.
        backgroundColor: ["b6e3f4", "c0aede", "d1d4f9", "ffd5dc", "ffdfbf"],
      }).toDataUri(),
    [seed],
  );

  return (
    <Box
      w={size}
      h={size}
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

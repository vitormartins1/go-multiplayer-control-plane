import { Box } from "@mantine/core";
import { useMemo } from "react";
import { Style, Avatar } from "@dicebear/core";
import botttsDef from "@dicebear/styles/bottts.json";
import openPeepsDef from "@dicebear/styles/open-peeps.json";

// PlayerAvatar é a "fotinha" determinística de um jogador (006-player-avatars,
// estilos por tipo de login na 024): DiceBear gerado no cliente a partir da
// seed (o playerId — estável e único). Guests anônimos têm subject
// "guest-<uuid>" (identity/002-anonymous-login) → robô (Bottts); contas
// registradas → humano (Open Peeps). Sem persistência, como a capa do jogo.
const botttsStyle = new Style(botttsDef);
const openPeepsStyle = new Style(openPeepsDef);

// isGuest decide o tipo de login pelo prefixo do playerId (fonte: identity).
export function isGuest(playerId: string): boolean {
  return playerId.startsWith("guest-");
}

export function PlayerAvatar({ seed, size = 32 }: { seed: string; size?: number }) {
  const uri = useMemo(
    () =>
      new Avatar(isGuest(seed) ? botttsStyle : openPeepsStyle, {
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

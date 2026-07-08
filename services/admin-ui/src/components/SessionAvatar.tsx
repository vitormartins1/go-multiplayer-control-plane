import { Box } from "@mantine/core";
import { useMemo } from "react";
import { Style, Avatar } from "@dicebear/core";
import ringsDef from "@dicebear/styles/rings.json";

// SessionAvatar é a âncora visual determinística de uma sessão (032): DiceBear
// Rings gerado no cliente a partir do sessionId — mesma mecânica do
// PlayerAvatar (024) e da capa do jogo, sem backend nem persistência.
const ringsStyle = new Style(ringsDef);

export function SessionAvatar({ seed, size = 24 }: { seed: string; size?: number }) {
  const uri = useMemo(
    () =>
      new Avatar(ringsStyle, {
        seed,
        // fundo escuro fixo: os anéis coloridos já dão a identidade; a paleta
        // clara dos players destoaria numa célula que é só ornamento.
        backgroundColor: ["1a1b1e"],
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

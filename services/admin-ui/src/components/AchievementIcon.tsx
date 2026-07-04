import { Box } from "@mantine/core";
import { useMemo } from "react";
import { Style, Avatar } from "@dicebear/core";
import glyphsDef from "@dicebear/styles/glyphs.json";

const glyphsStyle = new Style(glyphsDef);

export function AchievementIcon({ seed, size = 32 }: { seed: string; size?: number }) {
  const uri = useMemo(
    () => new Avatar(glyphsStyle, { seed }).toDataUri(),
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

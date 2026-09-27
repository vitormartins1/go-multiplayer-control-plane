import { AspectRatio, Box, Group, Text, Tooltip } from "@mantine/core";
import { useMemo } from "react";
import { Style, Avatar } from "@dicebear/core";
import shapesDef from "@dicebear/styles/shapes.json";
import stripesDef from "@dicebear/styles/stripes.json";
import trianglesDef from "@dicebear/styles/triangles.json";
import shapeGridDef from "@dicebear/styles/shape-grid.json";
import initialsDef from "@dicebear/styles/initials.json";
import glassDef from "@dicebear/styles/glass.json";
import discoDef from "@dicebear/styles/disco.json";
import type { Game } from "../lib/api";
import { ARCHETYPE_META } from "../lib/archetypeMeta";
import { useSettings, type GameCoverStyle } from "../lib/settings";

// Estilos suportados (abstratos/geométricos, não-avatar) — os 7 do picker de
// Settings (spec 082). `generatedImageStyle` do jogo (backend) não é mais
// consultado aqui: hoje é sempre "shapes" fixo (game-registry, sem endpoint
// de update) — quem decide o estilo é a preferência global (useSettings),
// não o campo por-jogo. Ver lib/settings.tsx.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export const COVER_STYLES: Record<GameCoverStyle, Style<any>> = {
  shapes: new Style(shapesDef),
  stripes: new Style(stripesDef),
  triangles: new Style(trianglesDef),
  "shape-grid": new Style(shapeGridDef),
  initials: new Style(initialsDef),
  glass: new Style(glassDef),
  disco: new Style(discoDef),
};

// coverSeed: a seed determinística da capa. Cai no slug se o backend não mandou
// generatedImageSeed (registros antigos / respostas sem o campo).
export function coverSeed(game: Game): string {
  return game.generatedImageSeed || game.slug;
}

// coverDataUri gera a imagem determinística (DiceBear) como data-URI SVG. Se no
// futuro houver upload (imageSource="upload" + image), usa a imagem direto.
function coverDataUri(game: Game, style: GameCoverStyle): string {
  if (game.imageSource === "upload" && game.image) return game.image;
  return new Avatar(COVER_STYLES[style], { seed: coverSeed(game) }).toDataUri();
}

function initial(name: string): string {
  return (name.trim()[0] ?? "?").toUpperCase();
}

interface GameCoverProps {
  game: Game;
  variant?: "cover" | "mini";
  // withName=false esconde a faixa do nome (036): em contextos onde o nome já
  // está ao lado (ex.: Dados do jogo), a faixa só quebrava texto em larguras menores.
  withName?: boolean;
  // Ícones de arquétipo no canto (biblioteca de jogos, revisão de layout
  // 2026-07-11) — só quando o caller quer esse metadado (a capa em si é
  // reusada em contextos sem isso, ex.: preview do wizard). Um ícone por
  // arquétipo DISTINTO — vários modos do mesmo arquétipo (ex.: 6 modos
  // host_world_dropin) não repetem o ícone.
  modes?: Game["multiplayerModes"];
  // Override do 9:16 padrão do "cover" — a capa é gerada proceduralmente
  // (DiceBear) e aceita qualquer proporção; o drawer de registro usa um
  // formato banner largo (revisão de layout, 2026-07-11).
  ratio?: number;
}

// GameCover é a capa procedural/determinística de um jogo (003-generated-cover).
// "cover" = card 9:16 da biblioteca (imagem + faixa com o nome, proporção
// customizável via `ratio`); "mini" = swatch quadrado para o seletor do header.
export function GameCover({ game, variant = "cover", withName = true, modes, ratio }: GameCoverProps) {
  const { gameCoverStyle } = useSettings();
  const uri = useMemo(
    () => coverDataUri(game, gameCoverStyle),
    // de propósito pelos campos, não pelo objeto: o polling devolve um objeto
    // novo a cada ciclo e regenerar a capa (DiceBear) a cada vez é caro.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [game.id, game.generatedImageSeed, game.slug, game.image, game.imageSource, gameCoverStyle],
  );

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

  const distinctArchetypes = modes ? [...new Set(modes.map((m) => m.archetype))] : [];

  return (
    <AspectRatio ratio={ratio ?? 9 / 16}>
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
        {distinctArchetypes.length > 0 && (
          <Group gap={4} style={{ position: "absolute", top: 8, left: 8 }}>
            {distinctArchetypes.map((a) => {
              const meta = ARCHETYPE_META[a];
              const Icon = meta.icon;
              return (
                <Tooltip key={a} label={meta.label} withArrow>
                  <Box
                    style={{
                      width: 22,
                      height: 22,
                      borderRadius: 6,
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "center",
                      background: "rgba(10, 13, 18, 0.65)",
                      backdropFilter: "blur(2px)",
                    }}
                  >
                    <Icon size={13} color="white" />
                  </Box>
                </Tooltip>
              );
            })}
          </Group>
        )}
        {/* faixa inferior para leitura do nome sobre a imagem */}
        {withName && (
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
        )}
        {/* acessibilidade: inicial não visível, só rótulo */}
        <span style={{ position: "absolute", width: 1, height: 1, overflow: "hidden", clip: "rect(0 0 0 0)" }}>
          {initial(game.name)}
        </span>
      </Box>
    </AspectRatio>
  );
}

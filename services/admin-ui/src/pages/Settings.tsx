import { Avatar as DicebearAvatar } from "@dicebear/core";
import { Badge, Box, Group, SimpleGrid, Stack, Text, Title, Tooltip, UnstyledButton } from "@mantine/core";
import { useMemo } from "react";
import { FormSection } from "../components/FormSection";
import { COVER_STYLES } from "../components/GameCover";
import { GAME_COVER_STYLES, useSettings, type GameCoverStyle } from "../lib/settings";

// PREVIEW_SEED: fixa, igual pra todo estilo — o picker compara o MESMO
// "jogo" em estilos diferentes, não estilos diferentes em jogos diferentes
// (que confundiria "isso é o estilo ou é a seed?").
const PREVIEW_SEED = "control-plane-preview";

function styleThumbUri(style: GameCoverStyle): string {
  return new DicebearAvatar(COVER_STYLES[style], { seed: PREVIEW_SEED }).toDataUri();
}

// StyleOption: um card clicável do picker — mesma mecânica visual de seleção
// (borda destacada quando ativo) que o resto do console usa pra escolha
// exclusiva (ex.: SegmentedControl), só que com preview real da imagem em
// vez de só texto — a decisão É visual, um dropdown de texto não mostra o
// que "stripes" ou "disco" realmente parecem.
function StyleOption({
  value,
  label,
  active,
  onSelect,
}: {
  value: GameCoverStyle;
  label: string;
  active: boolean;
  onSelect: () => void;
}) {
  const uri = useMemo(() => styleThumbUri(value), [value]);
  return (
    <UnstyledButton onClick={onSelect}>
      <Stack gap={6} align="center">
        <Box
          w={72}
          h={72}
          style={{
            borderRadius: 10,
            overflow: "hidden",
            backgroundImage: `url("${uri}")`,
            backgroundSize: "cover",
            backgroundPosition: "center",
            border: active ? "2px solid var(--mantine-color-signal-5)" : "1px solid var(--mantine-color-dark-4)",
            boxShadow: active ? "0 0 0 3px var(--mantine-color-signal-8)" : undefined,
          }}
        />
        <Text size="xs" c={active ? undefined : "dimmed"} fw={active ? 600 : 400}>
          {label}
        </Text>
      </Stack>
    </UnstyledButton>
  );
}

// Settings (spec 082): configurações do CONTROL PLANE em geral — distinto
// da aba "Configurações" de cada jogo (essa é por-jogo; esta é global,
// preferências de UI do console). Primeira: estilo das capas dos jogos
// (DiceBear). Guardado no navegador (lib/settings.tsx) — ver comentário lá
// pra por que não é um campo por-jogo no backend.
export function Settings() {
  const { gameCoverStyle, setGameCoverStyle } = useSettings();

  return (
    <Stack>
      <Title order={3}>Settings</Title>
      <Text size="sm" c="dimmed" mt={-8}>
        Configurações do control plane — preferências de UI do console, não configuração de um jogo específico
        (isso fica na aba Configurações de cada jogo).
      </Text>

      <FormSection label="Aparência">
        <Stack gap="md">
          <div>
            <Text size="sm" fw={500} mb={2}>
              Estilo das capas dos jogos
            </Text>
            <Text size="xs" c="dimmed" mb="sm">
              Gerado proceduralmente (DiceBear) a partir do slug do jogo — muda a aparência de todas as capas do
              console, não é preciso re-registrar nenhum jogo.
            </Text>
            <SimpleGrid cols={{ base: 3, xs: 4, sm: 7 }} spacing="md">
              {GAME_COVER_STYLES.map((s) => (
                <StyleOption
                  key={s.value}
                  value={s.value}
                  label={s.label}
                  active={s.value === gameCoverStyle}
                  onSelect={() => setGameCoverStyle(s.value)}
                />
              ))}
            </SimpleGrid>
          </div>
        </Stack>
      </FormSection>

      {/* Players (avatares de jogadores registrados + guests): fora de
          escopo por ora (pedido explícito do Vitor, 2026-07-13) — a seção
          já existe, desabilitada, pra não precisar redesenhar a página
          quando isso for implementado. pointerEvents:none + opacity é o
          mesmo tratamento "em breve" já usado no campo de repositório de
          imagem da Fleet (spec 077/078), só que na seção inteira. */}
      <FormSection label="Aparência dos jogadores">
        <Group justify="space-between" mb="sm">
          <Text size="sm" c="dimmed">
            Avatares de jogadores registrados e convidados (guests) — hoje fixos (Open Peeps / Bottts).
          </Text>
          <Tooltip label="Ainda não implementado — vem depois do estilo das capas dos jogos">
            <Badge size="sm" variant="light" color="gray">
              em breve
            </Badge>
          </Tooltip>
        </Group>
        <Box style={{ opacity: 0.4, pointerEvents: "none" }}>
          <SimpleGrid cols={{ base: 3, xs: 4, sm: 7 }} spacing="md">
            {GAME_COVER_STYLES.map((s) => (
              <StyleOption key={s.value} value={s.value} label={s.label} active={false} onSelect={() => {}} />
            ))}
          </SimpleGrid>
        </Box>
      </FormSection>
    </Stack>
  );
}

import { Stack, Text } from "@mantine/core";
import type { Icon } from "@tabler/icons-react";

// EmptyState (spec 088): estados vazios eram só uma linha de <Text c="dimmed">
// solta — funcionava (a frase já direcionava a ação), mas não tinha nenhum
// traço visual do resto do console (achado do Vitor, revisão de design).
// Mesmo vocabulário do hint já usado no canvas de Event Rules (ícone dimmed
// 28px + texto centralizado, spec 080) — generalizado aqui pros outros
// lugares onde uma lista pode ficar vazia. Não é um estado de erro (isso é
// Alert) nem uma zona ilustrativa (isso é MockZone) — é "nada aqui ainda",
// deliberadamente quieto.
export function EmptyState({ icon: StateIcon, message }: { icon: Icon; message: string }) {
  return (
    <Stack align="center" gap={6} py="xl">
      <StateIcon size={28} color="var(--mantine-color-dimmed)" stroke={1.5} />
      <Text size="sm" c="dimmed" ta="center">
        {message}
      </Text>
    </Stack>
  );
}

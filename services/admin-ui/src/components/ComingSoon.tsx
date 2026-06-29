import { Center, Stack, Text, ThemeIcon } from "@mantine/core";
import { IconTool } from "@tabler/icons-react";

// ComingSoon: esqueleto reutilizável para as abas/seções do futuro (roadmap
// visível sem backend pronto).
export function ComingSoon({ what }: { what: string }) {
  return (
    <Center mih={220}>
      <Stack align="center" gap="xs">
        <ThemeIcon variant="light" color="gray" size={48} radius="xl">
          <IconTool size={26} />
        </ThemeIcon>
        <Text fw={600}>{what}</Text>
        <Text c="dimmed" size="sm">
          Em breve — funcionalidade planejada para uma próxima iteração.
        </Text>
      </Stack>
    </Center>
  );
}

import { Badge, Box, Group, Text } from "@mantine/core";
import type { ReactNode } from "react";

// MockZone: contêiner com borda tracejada + badge "mock" — vocabulário visual
// único do console pra "isto é ilustrativo, sem endpoint real por trás hoje"
// (nasceu no redesign da Fleet, spec 077/078: o achado real foi que dado
// real e mock lado a lado sem distinção visual confundem). Qualquer seção
// mockada reusa este componente em vez de duplicar a borda tracejada solta.
export function MockZone({
  label,
  children,
  h,
}: {
  label: string;
  children: ReactNode;
  h?: string | number;
}) {
  return (
    <Box
      p="sm"
      h={h}
      style={{
        border: "1px dashed var(--mantine-color-dark-4)",
        borderRadius: 8,
        background: "var(--mantine-color-dark-8)",
      }}
    >
      <Group justify="space-between" mb={6}>
        <Text size="xs" c="dimmed" fw={600} tt="uppercase" style={{ letterSpacing: "0.04em" }}>
          {label}
        </Text>
        <Badge size="xs" variant="light" color="yellow">
          mock
        </Badge>
      </Group>
      {children}
    </Box>
  );
}

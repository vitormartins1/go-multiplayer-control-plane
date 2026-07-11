import { Card, Text } from "@mantine/core";
import type { ReactNode } from "react";

// FormSection substitui o <Fieldset> do Mantine (borda fina + legenda cortando
// a linha — o visual "formulário burocrático" padrão) por um cartão com o
// mesmo tratamento de rótulo de painel usado em StatCard/SectionLabel: mais
// consistente com o resto do console (revisão de layout, 2026-07-11).
export function FormSection({ label, children }: { label: string; children: ReactNode }) {
  return (
    <Card withBorder radius="md" p="md">
      <Text size="xs" fw={600} tt="uppercase" c="dimmed" mb="sm" style={{ letterSpacing: "0.06em" }}>
        {label}
      </Text>
      {children}
    </Card>
  );
}

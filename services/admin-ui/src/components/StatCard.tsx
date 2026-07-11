import { Card, Text } from "@mantine/core";

// StatCard é um card de métrica (label + valor) — tratamento de "leitura de
// instrumento" (POC de redesign): valor em monospace (tabular-nums, não
// muda de largura ao trocar de dígito) e label em versalete espaçado, como
// um rótulo de painel. Reusado pelos sinais ao vivo do Infrastructure
// (matchmaking/fan-out) e pelos totais da Analytics.
export function StatCard({ label, value, bad }: { label: string; value: string; bad?: boolean }) {
  return (
    <Card withBorder padding="sm">
      <Text size="xs" c="dimmed" tt="uppercase" fw={600} style={{ letterSpacing: "0.06em" }}>
        {label}
      </Text>
      <Text
        size="xl"
        fw={600}
        ff="monospace"
        c={bad ? "critical.4" : undefined}
        style={{ fontVariantNumeric: "tabular-nums" }}
      >
        {value}
      </Text>
    </Card>
  );
}

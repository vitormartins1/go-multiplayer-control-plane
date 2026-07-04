import { Card, Text } from "@mantine/core";

// StatCard é um card de métrica (label + valor). Reusado pelos sinais ao vivo do
// Infrastructure (matchmaking/fan-out) e pelos totais da Analytics.
export function StatCard({ label, value, bad }: { label: string; value: string; bad?: boolean }) {
  return (
    <Card withBorder padding="sm">
      <Text size="xs" c="dimmed">
        {label}
      </Text>
      <Text size="xl" fw={700} c={bad ? "red" : undefined}>
        {value}
      </Text>
    </Card>
  );
}

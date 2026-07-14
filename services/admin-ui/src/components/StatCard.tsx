import { Card, Text } from "@mantine/core";

// StatCard é um card de métrica (label + valor) — tratamento de "leitura de
// instrumento" (POC de redesign): valor em monospace (tabular-nums, não
// muda de largura ao trocar de dígito) e label em versalete espaçado, como
// um rótulo de painel. Reusado pelos sinais ao vivo do Infrastructure
// (matchmaking/fan-out) e pelos totais da Analytics.
//
// size="lg" (spec 086, pedido do Vitor): usado só nos widgets de Analytics
// que colocam o grid de stats ao lado de um DonutChart (~180px de altura
// com os rótulos) — um card "sm" de 1 linha (3 stats, sem par de 2 linhas)
// ficava baixo demais e desproporcional ao lado do círculo. Opt-in — os
// outros usos do StatCard (Infrastructure, header do jogo) continuam "sm".
export function StatCard({
  label,
  value,
  bad,
  size = "sm",
}: {
  label: string;
  value: string;
  bad?: boolean;
  size?: "sm" | "lg";
}) {
  return (
    <Card withBorder padding={size === "lg" ? "lg" : "sm"}>
      <Text size="xs" c="dimmed" tt="uppercase" fw={600} style={{ letterSpacing: "0.06em" }}>
        {label}
      </Text>
      <Text
        size={size === "lg" ? "1.75rem" : "xl"}
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

export type Status = "up" | "down" | "unknown";

export function statusClass(up: boolean | null | undefined): Status {
  if (up === null || up === undefined) return "unknown";
  return up ? "up" : "down";
}

// Cor Mantine por status.
export function statusColor(up: boolean | null | undefined): string {
  const s = statusClass(up);
  return s === "up" ? "green" : s === "down" ? "red" : "gray";
}

export function statusLabel(up: boolean | null | undefined): string {
  const s = statusClass(up);
  return s === "up" ? "saudável" : s === "down" ? "fora" : "desconhecido";
}

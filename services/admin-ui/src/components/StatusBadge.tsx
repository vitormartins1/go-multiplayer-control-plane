import { Badge } from "@mantine/core";
import { statusColor, statusLabel } from "../lib/status";

export function StatusBadge({ up }: { up: boolean | null | undefined }) {
  return (
    <Badge color={statusColor(up)} variant="light" size="sm">
      {statusLabel(up)}
    </Badge>
  );
}

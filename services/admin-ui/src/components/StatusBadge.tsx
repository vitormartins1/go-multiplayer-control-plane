export function statusClass(up: boolean | null | undefined): "up" | "down" | "unknown" {
  if (up === null || up === undefined) return "unknown";
  return up ? "up" : "down";
}

export function StatusBadge({ up }: { up: boolean | null | undefined }) {
  const cls = statusClass(up);
  const label = cls === "up" ? "up" : cls === "down" ? "down" : "desconhecido";
  return (
    <span className="badge">
      <span className={`dot ${cls}`} />
      {label}
    </span>
  );
}

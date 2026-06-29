// MetricCard mostra um valor; null/undefined vira "—" (métrica ausente, não zero).
export function MetricCard({
  label,
  value,
  suffix,
}: {
  label: string;
  value: number | null | undefined;
  suffix?: string;
}) {
  const text = value === null || value === undefined ? "—" : format(value) + (suffix ?? "");
  return (
    <div className="card">
      <div className="label">{label}</div>
      <div className="value">{text}</div>
    </div>
  );
}

function format(v: number): string {
  if (Number.isInteger(v)) return String(v);
  return v.toFixed(2);
}

import type { RefreshOptions } from "../lib/useLive";

const INTERVALS = [
  { label: "5s", ms: 5000 },
  { label: "15s", ms: 15000 },
  { label: "30s", ms: 30000 },
];

// RefreshControls: toggle de auto-refresh + seletor de intervalo (aplica aos
// dados ao vivo — Visão geral e métricas das páginas de domínio).
export function RefreshControls({
  opts,
  onChange,
}: {
  opts: RefreshOptions;
  onChange: (o: RefreshOptions) => void;
}) {
  return (
    <div className="refresh">
      <label className="badge">
        <input
          type="checkbox"
          checked={opts.enabled}
          onChange={(e) => onChange({ ...opts, enabled: e.target.checked })}
        />
        auto-refresh
      </label>
      <select
        value={opts.intervalMs}
        disabled={!opts.enabled}
        onChange={(e) => onChange({ ...opts, intervalMs: Number(e.target.value) })}
      >
        {INTERVALS.map((i) => (
          <option key={i.ms} value={i.ms}>
            {i.label}
          </option>
        ))}
      </select>
    </div>
  );
}

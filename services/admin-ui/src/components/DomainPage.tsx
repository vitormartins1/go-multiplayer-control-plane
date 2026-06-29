import { api } from "../lib/api";
import { useLive, type RefreshOptions } from "../lib/useLive";
import type { Domain } from "../lib/domains";
import { MetricCard } from "./MetricCard";
import { StatusBadge } from "./StatusBadge";
import { ErrorState } from "./ErrorState";
import { Loading } from "./Loading";
import type { Game } from "../lib/api";
import { AchievementsPanel } from "./AchievementsPanel";
import { UsersPanel } from "./UsersPanel";

// DomainPage genérico (apenas domínios de serviço; "planned" é tratado no App):
// saúde + RED do /api/services/{id}/metrics; contadores específicos (do
// /api/metrics/live); bloco "Iteração 2"; e o painel de achievements quando o
// domínio o tem.
export function DomainPage({
  domain,
  refresh,
  selectedGame,
}: {
  domain: Domain;
  refresh: RefreshOptions;
  selectedGame: Game | null;
}) {
  const metrics = useLive((s) => api.serviceMetrics(domain.id, s), [domain.id], refresh);
  const live = useLive((s) => api.liveMetrics(s), [domain.id], refresh);

  return (
    <div>
      {/* Saúde + RED */}
      {metrics.loading && !metrics.data ? (
        <Loading what="as métricas" />
      ) : metrics.error && !metrics.data ? (
        <ErrorState message="Métricas do serviço indisponíveis (Prometheus ou BFF fora)." onRetry={metrics.reload} />
      ) : metrics.data ? (
        <>
          <div style={{ marginBottom: 12 }}>
            <StatusBadge up={metrics.data.up} />
          </div>
          <div className="cards">
            <MetricCard label="Req/s" value={metrics.data.requestRate} />
            <MetricCard label="Erro" value={ratePct(metrics.data.errorRate)} suffix="%" />
            <MetricCard label="Latência p95" value={ms(metrics.data.latencyP95)} suffix=" ms" />
            <MetricCard label="Goroutines" value={metrics.data.goroutines} />
            <MetricCard label="Memória" value={mib(metrics.data.memoryBytes)} suffix=" MiB" />
          </div>
          {metrics.data.requestRate === null && (
            <p className="muted" style={{ marginTop: 8 }}>
              Serviço gRPC — sem métricas RED HTTP (só saúde + runtime).
            </p>
          )}
        </>
      ) : null}

      {/* Contadores de produto específicos do domínio (do /api/metrics/live) */}
      {domain.counters && domain.counters.length > 0 && (
        <>
          <h3 style={{ margin: "18px 0 10px" }}>Contadores</h3>
          <div className="cards">
            {domain.counters.map((c) => (
              <MetricCard key={c.label} label={c.label} value={live.data ? c.pick(live.data) : undefined} />
            ))}
          </div>
        </>
      )}

      {/* User base: lista de usuários (só no domínio identity) */}
      {domain.hasUsers && (
        <>
          <h3 style={{ margin: "18px 0 10px" }}>Usuários</h3>
          <UsersPanel />
        </>
      )}

      {/* Achievements (só no domínio achievements) */}
      {domain.hasAchievements && <AchievementsPanel selectedGame={selectedGame} />}

      {/* Roadmap visível */}
      {domain.iter2 && domain.iter2.length > 0 && (
        <div className="iter2">
          <h4>Iteração 2</h4>
          <ul style={{ margin: 0, paddingLeft: 18 }}>
            {domain.iter2.map((x) => (
              <li key={x}>{x}</li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}

function ratePct(v: number | null): number | null {
  return v === null ? null : Math.round(v * 1000) / 10; // fração -> %
}
function ms(secs: number | null): number | null {
  return secs === null ? null : Math.round(secs * 1000);
}
function mib(bytes: number | null): number | null {
  return bytes === null ? null : Math.round((bytes / (1024 * 1024)) * 10) / 10;
}

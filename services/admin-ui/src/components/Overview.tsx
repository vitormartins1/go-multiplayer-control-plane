import { api } from "../lib/api";
import { useLive, type RefreshOptions } from "../lib/useLive";
import { TopologyGraph } from "./TopologyGraph";
import { MetricCard } from "./MetricCard";
import { Loading } from "./Loading";
import { ErrorState } from "./ErrorState";

// Visão geral: topologia (domínios que se comunicam) + contadores globais.
export function Overview({
  refresh,
  onSelectDomain,
}: {
  refresh: RefreshOptions;
  onSelectDomain: (id: string) => void;
}) {
  const topo = useLive((s) => api.topology(s), [], refresh);
  const live = useLive((s) => api.liveMetrics(s), [], refresh);

  return (
    <div>
      {topo.loading && !topo.data ? (
        <Loading what="a topologia" />
      ) : topo.error && !topo.data ? (
        <ErrorState message="Não foi possível carregar a topologia (BFF indisponível)." onRetry={topo.reload} />
      ) : topo.data ? (
        <>
          {!topo.data.sourceAvailable && (
            <div className="banner">
              Prometheus indisponível — status dos serviços desconhecido.
            </div>
          )}
          <TopologyGraph topo={topo.data} onSelectDomain={onSelectDomain} />
        </>
      ) : null}

      <h3 style={{ margin: "8px 0 12px" }}>Contadores ao vivo</h3>
      {live.error && !live.data ? (
        <ErrorState message="Métricas ao vivo indisponíveis." onRetry={live.reload} />
      ) : (
        <div className="cards">
          <MetricCard label="Fila matchmaking" value={live.data?.queueSize} />
          <MetricCard label="Sessions ativas" value={live.data?.activeSessions} />
          <MetricCard label="Matches" value={live.data?.matchesTotal} />
          <MetricCard label="Matches falhos" value={live.data?.matchesFailed} />
        </div>
      )}
    </div>
  );
}

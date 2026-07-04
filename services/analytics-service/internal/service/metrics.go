package service

import "github.com/prometheus/client_golang/prometheus"

// Metrics são as métricas RED do ingest (ADR-0021 / OBSERVABILITY): eventos
// recebidos/processados e workers ativos.
type Metrics struct {
	received      prometheus.Counter
	processed     *prometheus.CounterVec
	workersActive prometheus.Gauge
}

// NewMetrics registra as métricas do worker pool.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		received: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "analytics_events_received_total",
			Help: "Partidas recebidas para agregação (submetidas ao pool).",
		}),
		processed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "analytics_events_processed_total",
			Help: "Partidas agregadas por resultado (ok/dup/error).",
		}, []string{"result"}),
		workersActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "analytics_workers_active",
			Help: "Workers agregando uma partida no momento.",
		}),
	}
	reg.MustRegister(m.received, m.processed, m.workersActive)
	return m
}

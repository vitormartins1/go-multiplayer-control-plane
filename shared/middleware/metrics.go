// Package middleware contém middlewares HTTP compartilhados entre serviços.
package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics expõe métricas RED (Rate, Errors, Duration) das requisições HTTP.
// Cada serviço cria uma instância e envolve seu handler com Wrap. Ver ADR-0006.
type Metrics struct {
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

// NewMetrics registra os coletores no registry informado (use
// prometheus.DefaultRegisterer para o registry padrão, que já inclui as
// métricas de runtime do Go).
func NewMetrics(reg prometheus.Registerer, service string) *Metrics {
	f := promauto.With(reg)
	labels := prometheus.Labels{"service": service}
	return &Metrics{
		requests: f.NewCounterVec(prometheus.CounterOpts{
			Name:        "http_requests_total",
			Help:        "Total de requisições HTTP.",
			ConstLabels: labels,
		}, []string{"method", "route", "status"}),
		duration: f.NewHistogramVec(prometheus.HistogramOpts{
			Name:        "http_request_duration_seconds",
			Help:        "Duração das requisições HTTP em segundos.",
			Buckets:     prometheus.DefBuckets,
			ConstLabels: labels,
		}, []string{"method", "route"}),
	}
}

// Wrap instrumenta um handler: mede latência e conta requisições por
// método/rota/status. O label de rota usa o pattern do ServeMux (baixa
// cardinalidade), não o path bruto.
func (m *Metrics) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		route := r.Pattern
		if route == "" {
			route = "unmatched"
		}
		m.requests.WithLabelValues(r.Method, route, strconv.Itoa(rec.status)).Inc()
		m.duration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
	})
}

// statusRecorder captura o status code escrito pelo handler.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

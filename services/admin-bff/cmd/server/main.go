// Command server é o entrypoint do admin-bff: o back-end-for-frontend do painel
// de administração (ADR-0016). Agregador read-only e stateless — lê o Prometheus
// e a API admin do achievements e expõe uma API HTTP/JSON para o admin-ui.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	achievementsclient "github.com/c130n/game-backend/services/admin-bff/internal/achievements"
	gameregistryclient "github.com/c130n/game-backend/services/admin-bff/internal/gameregistry"
	identityclient "github.com/c130n/game-backend/services/admin-bff/internal/identity"
	prometheusclient "github.com/c130n/game-backend/services/admin-bff/internal/prometheus"
	"github.com/c130n/game-backend/services/admin-bff/internal/service"
	"github.com/c130n/game-backend/services/admin-bff/internal/transport"
	"github.com/c130n/game-backend/shared/config"
	"github.com/c130n/game-backend/shared/logger"
	"github.com/c130n/game-backend/shared/middleware"
	"github.com/c130n/game-backend/shared/telemetry"
)

const serviceName = "admin-bff"

func main() {
	log := logger.New(serviceName)
	if err := run(log); err != nil {
		log.Error("encerrando com erro", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg := config.Load()

	shutdownTracing, err := telemetry.SetupTracing(context.Background(), telemetry.Config{
		ServiceName:  serviceName,
		OTLPEndpoint: cfg.OTLPEndpoint,
		SampleRatio:  cfg.TraceSampleRatio,
	})
	if err != nil {
		return fmt.Errorf("telemetry: %w", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTracing(ctx); err != nil {
			log.Error("telemetry shutdown", "err", err)
		}
	}()

	promURL := os.Getenv("PROMETHEUS_URL")
	if promURL == "" {
		log.Warn("PROMETHEUS_URL ausente — topologia/contadores vão degradar; aponte para o Prometheus")
	}
	achURL := os.Getenv("ACHIEVEMENTS_URL")
	if achURL == "" {
		log.Warn("ACHIEVEMENTS_URL ausente — o painel de achievements vai falhar; aponte para o achievements-service")
	}
	identityURL := os.Getenv("IDENTITY_URL")
	if identityURL == "" {
		log.Warn("IDENTITY_URL ausente — a User base vai falhar; aponte para o identity-service")
	}
	gameRegistryURL := os.Getenv("GAME_REGISTRY_URL")
	if gameRegistryURL == "" {
		log.Warn("GAME_REGISTRY_URL ausente — a gestão de jogos vai falhar; aponte para o game-registry")
	}

	// Clients de saída com otelhttp: propaga o trace do BFF até as fontes (ADR-0014).
	httpClient := &http.Client{
		Timeout:   5 * time.Second,
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}
	metrics := prometheusclient.New(promURL, httpClient)
	reader := achievementsclient.New(achURL, httpClient)
	directory := identityclient.New(identityURL, httpClient)
	registry := gameregistryclient.New(gameRegistryURL, httpClient)
	svc := service.New(metrics, reader, directory, registry)

	red := middleware.NewMetrics(prometheus.DefaultRegisterer, serviceName)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.Handle("GET /metrics", promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{EnableOpenMetrics: true}))
	transport.NewHandler(svc).Register(mux)

	// otelhttp externo abre o span por request; RouteSpanName nomeia pela rota.
	// /healthz e /metrics ficam de fora para não poluir os traces.
	traced := otelhttp.NewHandler(middleware.RouteSpanName(red.Wrap(mux)), "http.server",
		otelhttp.WithFilter(func(r *http.Request) bool {
			return r.URL.Path != "/healthz" && r.URL.Path != "/metrics"
		}),
	)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      traced,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("http server iniciando", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
	}

	log.Info("desligando...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	log.Info("encerrado")
	return nil
}

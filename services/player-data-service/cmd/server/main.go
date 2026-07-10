// Command server é o entrypoint do player-data-service: faz o wire-up das
// dependências (config, logger, tracing, banco, verificador de JWT, service,
// transport) e sobe o HTTP server com graceful shutdown.
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

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/player-data-service/internal/repository"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/player-data-service/internal/service"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/player-data-service/internal/transport"
	"github.com/vitormartins1/go-multiplayer-control-plane/shared/auth"
	"github.com/vitormartins1/go-multiplayer-control-plane/shared/config"
	"github.com/vitormartins1/go-multiplayer-control-plane/shared/logger"
	"github.com/vitormartins1/go-multiplayer-control-plane/shared/middleware"
	"github.com/vitormartins1/go-multiplayer-control-plane/shared/telemetry"
)

const serviceName = "player-data-service"

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

	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("pgxpool config: %w", err)
	}
	poolCfg.ConnConfig.Tracer = otelpgx.NewTracer() // spans de query dentro do trace (ADR-0014)
	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		return fmt.Errorf("pgxpool: %w", err)
	}
	defer pool.Close()

	verifier, err := auth.NewVerifier(cfg.IdentityJWKSURL, 5*time.Minute)
	if err != nil {
		return fmt.Errorf("IDENTITY_JWKS_URL: %w", err)
	}

	repo := repository.NewPostgresPlayerDataRepository(pool)
	svc := service.New(repo)
	handler := transport.NewHandler(svc, verifier)

	metrics := middleware.NewMetrics(prometheus.DefaultRegisterer, serviceName)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.Handle("GET /metrics", promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{EnableOpenMetrics: true}))
	handler.Register(mux)

	// /admin exige a internal key serviço-a-serviço (ADR-0018).
	internalKey := os.Getenv("INTERNAL_API_KEY")
	if internalKey == "" {
		log.Warn("INTERNAL_API_KEY ausente — os endpoints /admin vão negar (401); configure a chave")
	}
	guarded := auth.RequireInternalKey(internalKey, "/admin")(mux)

	traced := otelhttp.NewHandler(middleware.RouteSpanName(metrics.Wrap(guarded)), "http.server",
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

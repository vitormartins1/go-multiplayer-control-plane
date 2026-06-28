// Command server é o entrypoint do cloud-save-service: faz o wire-up das
// dependências (config, logger, tracing, object storage, verificador de JWT,
// service, transport) e sobe o HTTP server com graceful shutdown.
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

	"github.com/c130n/game-backend/services/cloud-save-service/internal/service"
	"github.com/c130n/game-backend/services/cloud-save-service/internal/storage"
	"github.com/c130n/game-backend/services/cloud-save-service/internal/transport"
	"github.com/c130n/game-backend/shared/auth"
	"github.com/c130n/game-backend/shared/config"
	"github.com/c130n/game-backend/shared/logger"
	"github.com/c130n/game-backend/shared/middleware"
	"github.com/c130n/game-backend/shared/telemetry"
)

const serviceName = "cloud-save-service"

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

	store, err := connectStore(context.Background(), cfg, log)
	if err != nil {
		return fmt.Errorf("object storage: %w", err)
	}

	if cfg.IdentityJWKSURL == "" {
		log.Warn("IDENTITY_JWKS_URL ausente — a validação de token vai falhar; aponte para o JWKS do identity")
	}
	verifier := auth.NewVerifier(cfg.IdentityJWKSURL, 5*time.Minute)

	svc := service.New(store)
	handler := transport.NewHandler(svc, verifier)

	metrics := middleware.NewMetrics(prometheus.DefaultRegisterer, serviceName)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.Handle("GET /metrics", promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{EnableOpenMetrics: true}))
	handler.Register(mux)

	traced := otelhttp.NewHandler(middleware.RouteSpanName(metrics.Wrap(mux)), "http.server",
		otelhttp.WithFilter(func(r *http.Request) bool {
			return r.URL.Path != "/healthz" && r.URL.Path != "/metrics"
		}),
	)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      traced,
		ReadTimeout:  30 * time.Second, // saves podem ser grandes
		WriteTimeout: 30 * time.Second,
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

// connectStore conecta ao object storage com backoff — o MinIO pode ainda
// estar subindo no compose quando o serviço inicia.
func connectStore(ctx context.Context, cfg config.Config, log *slog.Logger) (*storage.S3Store, error) {
	var lastErr error
	for i := 0; i < 15; i++ {
		store, err := storage.NewS3Store(ctx, cfg.S3Endpoint, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3Bucket)
		if err == nil {
			return store, nil
		}
		lastErr = err
		log.Warn("object storage ainda não pronto; tentando de novo", "err", err)
		time.Sleep(time.Second)
	}
	return nil, lastErr
}

// Command server é o entrypoint do analytics-service (ADR-0021): sobe o worker
// pool (core), o consumidor SQS (driving adapter) e um HTTP com a read API do
// painel + /metrics + /healthz. Shutdown ordenado: para o consumidor, então
// drena o pool.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/vitormartins1/go-game-backend/services/analytics-service/internal/awsx"
	"github.com/vitormartins1/go-game-backend/services/analytics-service/internal/consumer"
	"github.com/vitormartins1/go-game-backend/services/analytics-service/internal/repository"
	"github.com/vitormartins1/go-game-backend/services/analytics-service/internal/service"
	"github.com/vitormartins1/go-game-backend/services/analytics-service/internal/transport"
	"github.com/vitormartins1/go-game-backend/shared/config"
	"github.com/vitormartins1/go-game-backend/shared/logger"
	"github.com/vitormartins1/go-game-backend/shared/middleware"
	"github.com/vitormartins1/go-game-backend/shared/telemetry"
)

const serviceName = "analytics-service"

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
	poolCfg.ConnConfig.Tracer = otelpgx.NewTracer()
	db, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		return fmt.Errorf("pgxpool: %w", err)
	}
	defer db.Close()

	repo := repository.New(db)
	metrics := service.NewMetrics(prometheus.DefaultRegisterer)
	workers := intEnv("ANALYTICS_WORKERS", 4)
	pool := service.NewPool(repo, workers, workers*2, log, metrics)

	// ctx do processo: cancelado no SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool.Start(ctx)

	// Consumidor SQS (se configurado). Roda em goroutine; o pool é o Ingestor.
	var consumerWG sync.WaitGroup
	if queueURL := os.Getenv("ANALYTICS_QUEUE_URL"); queueURL != "" {
		awsCfg, err := awsx.Config(context.Background())
		if err != nil {
			return fmt.Errorf("aws config: %w", err)
		}
		sqsClient := sqs.NewFromConfig(awsCfg, func(o *sqs.Options) {
			o.BaseEndpoint = awsx.BaseEndpoint(o.BaseEndpoint)
		})
		c := consumer.New(sqsClient, queueURL, pool, log)
		consumerWG.Add(1)
		go func() {
			defer consumerWG.Done()
			c.Run(ctx)
		}()
	} else {
		log.Warn("ANALYTICS_QUEUE_URL ausente — só a read API sobe (sem ingestão)")
	}

	// HTTP: read API (admin) + observabilidade.
	red := middleware.NewMetrics(prometheus.DefaultRegisterer, serviceName)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.Handle("GET /metrics", promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{EnableOpenMetrics: true}))
	transport.NewAdminHandler(repo).Register(mux)

	traced := otelhttp.NewHandler(middleware.RouteSpanName(red.Wrap(mux)), "http.server",
		otelhttp.WithFilter(func(r *http.Request) bool {
			return r.URL.Path != "/healthz" && r.URL.Path != "/metrics"
		}),
	)
	srv := &http.Server{Addr: ":" + cfg.Port, Handler: traced, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second}

	errCh := make(chan error, 1)
	go func() {
		log.Info("http server iniciando", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		stop()
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
	}

	// Shutdown ordenado: HTTP, depois consumidor (para de submeter), depois pool.
	log.Info("desligando...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	consumerWG.Wait() // consumidor para de submeter
	pool.Stop()       // drena e encerra os workers
	log.Info("encerrado")
	return nil
}

func intEnv(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

// Command server é o entrypoint do identity-service: faz o wire-up das
// dependências (config, logger, banco, token issuer, service, transport) e
// sobe o HTTP server com graceful shutdown.
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/c130n/game-backend/services/identity-service/internal/repository"
	"github.com/c130n/game-backend/services/identity-service/internal/service"
	"github.com/c130n/game-backend/services/identity-service/internal/token"
	"github.com/c130n/game-backend/services/identity-service/internal/transport"
	"github.com/c130n/game-backend/shared/config"
	"github.com/c130n/game-backend/shared/logger"
	"github.com/c130n/game-backend/shared/middleware"
	"github.com/c130n/game-backend/shared/telemetry"
)

func main() {
	log := logger.New("identity-service")
	if err := run(log); err != nil {
		log.Error("encerrando com erro", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg := config.Load()

	shutdownTracing, err := telemetry.SetupTracing(context.Background(), telemetry.Config{
		ServiceName:  "identity-service",
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

	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("pgxpool: %w", err)
	}
	defer pool.Close()

	issuer, err := buildIssuer(cfg, log)
	if err != nil {
		return err
	}

	repo := repository.NewPostgresAccountRepository(pool)
	svc := service.New(repo, issuer)
	handler := transport.NewHandler(svc, issuer)

	metrics := middleware.NewMetrics(prometheus.DefaultRegisterer, "identity-service")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.Handle("GET /metrics", promhttp.Handler())
	handler.Register(mux)

	// otelhttp é o handler mais externo: abre o span por request e continua o
	// trace do client via traceparent (propagator global). RouteSpanName renomeia
	// o span para a rota casada (ex. "POST /accounts"). /healthz e /metrics ficam
	// de fora para não poluir os traces.
	traced := otelhttp.NewHandler(middleware.RouteSpanName(metrics.Wrap(mux)), "http.server",
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

// buildIssuer carrega a chave Ed25519 do config (PEM PKCS8) ou, em dev,
// gera uma chave efêmera com aviso.
func buildIssuer(cfg config.Config, log *slog.Logger) (*token.Issuer, error) {
	if cfg.JWTPrivateKey == "" {
		log.Warn("JWT_PRIVATE_KEY ausente — gerando chave Ed25519 efêmera (apenas dev); tokens não sobrevivem a restart")
		return token.Generate()
	}
	block, _ := pem.Decode([]byte(cfg.JWTPrivateKey))
	if block == nil {
		return nil, errors.New("JWT_PRIVATE_KEY: PEM inválido")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("JWT_PRIVATE_KEY: %w", err)
	}
	priv, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("JWT_PRIVATE_KEY: não é uma chave Ed25519")
	}
	return token.New(priv), nil
}

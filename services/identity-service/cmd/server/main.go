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

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/identity-service/internal/domain"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/identity-service/internal/presence"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/identity-service/internal/repository"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/identity-service/internal/service"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/identity-service/internal/token"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/identity-service/internal/transport"
	"github.com/vitormartins1/go-multiplayer-control-plane/shared/auth"
	"github.com/vitormartins1/go-multiplayer-control-plane/shared/config"
	"github.com/vitormartins1/go-multiplayer-control-plane/shared/logger"
	"github.com/vitormartins1/go-multiplayer-control-plane/shared/middleware"
	"github.com/vitormartins1/go-multiplayer-control-plane/shared/telemetry"
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

	issuer, err := buildIssuer(cfg, log)
	if err != nil {
		return err
	}

	// internalKey serve tanto o guard de entrada (/admin) quanto a chamada de
	// saída ao game-registry (login-into-game, ADR-0018/0019).
	internalKey := os.Getenv("INTERNAL_API_KEY")
	if internalKey == "" {
		log.Warn("INTERNAL_API_KEY ausente — /admin nega (401) e a presença não será registrada; configure a chave")
	}

	// notifier de presença: caller serviço-a-serviço para o game-registry. Sem
	// GAME_REGISTRY_URL, fica nil (login funciona, só não marca presença).
	var notifier domain.PresenceNotifier
	if gameRegistryURL := os.Getenv("GAME_REGISTRY_URL"); gameRegistryURL != "" {
		presenceClient := &http.Client{
			Timeout:   5 * time.Second,
			Transport: auth.InternalKeyTransport(internalKey, otelhttp.NewTransport(http.DefaultTransport)),
		}
		notifier = presence.New(gameRegistryURL, presenceClient)
	} else {
		log.Warn("GAME_REGISTRY_URL ausente — login-into-game não registra presença (ADR-0019)")
	}

	repo := repository.NewPostgresAccountRepository(pool)
	svc := service.New(repo, issuer, notifier, log)
	lister := service.NewPlayerLister(repo) // listing admin (003-admin-list-users)
	handler := transport.NewHandler(svc, issuer, lister)

	metrics := middleware.NewMetrics(prometheus.DefaultRegisterer, "identity-service")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	// OpenMetrics habilita a exposição de exemplars (trace_id no histograma de
	// latência) para o Grafana saltar métrica -> trace. Ver ADR-0014.
	mux.Handle("GET /metrics", promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{EnableOpenMetrics: true}))
	handler.Register(mux)

	// /admin exige a internal key serviço-a-serviço (ADR-0018); /healthz e
	// /metrics ficam livres para o Prometheus raspar. (internalKey lido acima.)
	guarded := auth.RequireInternalKey(internalKey, "/admin")(mux)

	// otelhttp é o handler mais externo: abre o span por request e continua o
	// trace do client via traceparent (propagator global). RouteSpanName renomeia
	// o span para a rota casada (ex. "POST /accounts"). /healthz e /metrics ficam
	// de fora para não poluir os traces.
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

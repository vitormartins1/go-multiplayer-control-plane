// Command server é o entrypoint do session-service: sobe o servidor gRPC
// (CreateSession, ReportResult — unary) + um HTTP pequeno para o JWKS do connect
// token, /healthz e /metrics. Faz o wire-up dos adapters (allocator mock, connect
// token issuer Ed25519, publisher stub) e graceful shutdown dos dois listeners.
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "github.com/c130n/game-backend/services/session-service/gen/session/v1"
	"github.com/c130n/game-backend/services/session-service/internal/allocator"
	"github.com/c130n/game-backend/services/session-service/internal/domain"
	"github.com/c130n/game-backend/services/session-service/internal/publisher"
	"github.com/c130n/game-backend/services/session-service/internal/service"
	"github.com/c130n/game-backend/services/session-service/internal/token"
	"github.com/c130n/game-backend/services/session-service/internal/transport"
	"github.com/c130n/game-backend/shared/auth"
	"github.com/c130n/game-backend/shared/config"
	"github.com/c130n/game-backend/shared/logger"
	"github.com/c130n/game-backend/shared/telemetry"
)

const serviceName = "session-service"

// defaultServerPool é o pool mock de game servers em dev (SERVER_POOL vazio).
// Em prod o adapter Agones substitui o allocator (ADR-0011/0012).
var defaultServerPool = []string{"10.0.0.1:7777", "10.0.0.2:7777", "10.0.0.3:7777"}

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

	issuer, err := buildIssuer(cfg, log)
	if err != nil {
		return err
	}

	pool := serverPool()
	log.Info("pool de game servers (mock)", "size", len(pool))
	svc := service.New(allocator.New(pool), issuer, buildPublisher(log))

	// métrica custom: sessões ativas é o que interessa observar aqui.
	prometheus.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "session_active_sessions", Help: "Sessões de jogo ativas.",
	}, svc.ActiveSessions))

	grpcSrv := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()), // tracing gRPC (ADR-0014)
	)
	pb.RegisterSessionServer(grpcSrv, transport.NewServer(svc))
	reflection.Register(grpcSrv) // permite grpcurl sem o .proto

	grpcLis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		return fmt.Errorf("grpc listen: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	// JWKS do connect token: o game server valida o token offline (ADR-0010/0011).
	mux.HandleFunc("GET /.well-known/jwks.json", func(w http.ResponseWriter, r *http.Request) {
		b, err := issuer.JWKS(r.Context())
		if err != nil {
			http.Error(w, "internal", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	})
	mux.Handle("GET /metrics", promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{EnableOpenMetrics: true}))
	httpSrv := &http.Server{Addr: ":" + cfg.Port, Handler: mux, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 2)
	go func() {
		log.Info("grpc server iniciando", "addr", grpcLis.Addr().String())
		if err := grpcSrv.Serve(grpcLis); err != nil {
			errCh <- fmt.Errorf("grpc server: %w", err)
		}
	}()
	go func() {
		log.Info("http server iniciando", "addr", httpSrv.Addr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("http server: %w", err)
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	log.Info("desligando...")
	grpcSrv.GracefulStop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("http shutdown: %w", err)
	}
	log.Info("encerrado")
	return nil
}

// serverPool lê o pool de game servers do ambiente (SERVER_POOL, lista separada
// por vírgula) ou usa o pool mock de dev.
func serverPool() []string {
	raw := os.Getenv("SERVER_POOL")
	if raw == "" {
		return defaultServerPool
	}
	var out []string
	for _, addr := range strings.Split(raw, ",") {
		if addr = strings.TrimSpace(addr); addr != "" {
			out = append(out, addr)
		}
	}
	return out
}

// buildPublisher escolhe o fan-out do resultado: se ACHIEVEMENTS_URL estiver
// setado, usa o adapter real (HTTP + internal key, ADR-0018, trace via otelhttp);
// senão, cai no fallback que só loga.
func buildPublisher(log *slog.Logger) domain.ResultPublisher {
	achURL := os.Getenv("ACHIEVEMENTS_URL")
	if achURL == "" {
		log.Warn("ACHIEVEMENTS_URL ausente — fan-out do resultado só vai logar (fallback)")
		return publisher.New(log)
	}
	internalKey := os.Getenv("INTERNAL_API_KEY")
	if internalKey == "" {
		log.Warn("INTERNAL_API_KEY ausente — o fan-out vai dar 401 no achievements")
	}
	client := &http.Client{
		Timeout:   5 * time.Second,
		Transport: auth.InternalKeyTransport(internalKey, otelhttp.NewTransport(http.DefaultTransport)),
	}
	log.Info("fan-out do resultado via achievements", "url", achURL)
	return publisher.NewAchievements(achURL, client, log)
}

// buildIssuer carrega a chave Ed25519 do config (PEM PKCS8) ou, em dev, gera uma
// chave efêmera com aviso. É a chave PRÓPRIA do session (distinta da do identity).
func buildIssuer(cfg config.Config, log *slog.Logger) (*token.ConnectIssuer, error) {
	if cfg.JWTPrivateKey == "" {
		log.Warn("JWT_PRIVATE_KEY ausente — gerando chave Ed25519 efêmera (apenas dev); connect tokens não sobrevivem a restart")
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

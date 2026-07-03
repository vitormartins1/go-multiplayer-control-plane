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
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	pb "github.com/vitormartins1/go-game-backend/services/session-service/gen/session/v1"
	"github.com/vitormartins1/go-game-backend/services/session-service/internal/allocator"
	"github.com/vitormartins1/go-game-backend/services/session-service/internal/domain"
	"github.com/vitormartins1/go-game-backend/services/session-service/internal/publisher"
	"github.com/vitormartins1/go-game-backend/services/session-service/internal/service"
	"github.com/vitormartins1/go-game-backend/services/session-service/internal/token"
	"github.com/vitormartins1/go-game-backend/services/session-service/internal/transport"
	"github.com/vitormartins1/go-game-backend/shared/auth"
	"github.com/vitormartins1/go-game-backend/shared/config"
	"github.com/vitormartins1/go-game-backend/shared/logger"
	"github.com/vitormartins1/go-game-backend/shared/telemetry"
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

	alloc, err := buildAllocator(log)
	if err != nil {
		return err
	}
	svc := service.New(alloc, issuer, buildPublisher(log),
		service.WithHistoryLimit(intEnv("SESSION_HISTORY_LIMIT", 200, log)))

	// métricas custom: sessões ativas + fechamentos por motivo (spec 004).
	prometheus.MustRegister(
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "session_active_sessions", Help: "Sessões de jogo ativas.",
		}, svc.ActiveSessions),
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Name: "sessions_closed_total", Help: "Sessões fechadas, por motivo.",
			ConstLabels: prometheus.Labels{"reason": string(domain.CloseResult)},
		}, func() float64 { return svc.ClosedTotal(domain.CloseResult) }),
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Name: "sessions_closed_total", Help: "Sessões fechadas, por motivo.",
			ConstLabels: prometheus.Labels{"reason": string(domain.CloseTTL)},
		}, func() float64 { return svc.ClosedTotal(domain.CloseTTL) }),
	)

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
	// superfície admin (spec 005): a aba Sessions do painel lê via BFF.
	transport.NewAdminHandler(svc).Register(mux)
	httpSrv := &http.Server{Addr: ":" + cfg.Port, Handler: mux, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Sweeper de TTL (spec 004): sessões abandonadas fecham sozinhas e o
	// GameServer é reciclado. Morre com o ctx no shutdown.
	sessionTTL := durationEnv("SESSION_TTL", 30*time.Minute, log)
	go svc.RunSweeper(ctx, sessionTTL, time.Minute, log)

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

// buildAllocator monta o ServerAllocator conforme ALLOCATOR_MODE (spec 004):
// `static` (default) mantém o pool mock em memória; `agones` cria
// GameServerAllocations no cluster (kubeconfig via KUBECONFIG ou in-cluster).
// DEV_PUBLIC_HOST (dev) decora qualquer modo reescrevendo o host devolvido.
func buildAllocator(log *slog.Logger) (domain.ServerAllocator, error) {
	var alloc domain.ServerAllocator
	switch mode := os.Getenv("ALLOCATOR_MODE"); mode {
	case "", "static":
		pool := serverPool()
		log.Info("allocator static (pool mock)", "size", len(pool))
		alloc = allocator.New(pool)
	case "agones":
		restCfg, err := kubeRestConfig()
		if err != nil {
			return nil, fmt.Errorf("allocator agones: %w", err)
		}
		client, err := dynamic.NewForConfig(restCfg)
		if err != nil {
			return nil, fmt.Errorf("allocator agones: client: %w", err)
		}
		namespace := envOr("AGONES_NAMESPACE", "default")
		fleet := envOr("AGONES_FLEET", "packet-arena")
		log.Info("allocator agones", "namespace", namespace, "fleet", fleet)
		alloc = allocator.NewAgones(client, namespace, fleet)
	default:
		return nil, fmt.Errorf("ALLOCATOR_MODE inválido: %q (static|agones)", mode)
	}

	if host := os.Getenv("DEV_PUBLIC_HOST"); host != "" {
		log.Warn("DEV_PUBLIC_HOST ativo — host dos endereços alocados reescrito (config de dev)", "host", host)
		alloc = allocator.WithPublicHost(alloc, host)
	}
	return alloc, nil
}

// kubeRestConfig resolve a config do k8s: KUBECONFIG (dev/compose, arquivo
// montado) ou in-cluster (deploy no k3s).
func kubeRestConfig() (*rest.Config, error) {
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("kubeconfig %q: %w", kubeconfig, err)
		}
		return cfg, nil
	}
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("in-cluster config (defina KUBECONFIG fora do cluster): %w", err)
	}
	return cfg, nil
}

// envOr lê a env ou devolve o default.
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// intEnv lê um inteiro da env; inválido/ausente cai no default (com log).
func intEnv(key string, def int, log *slog.Logger) int {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		log.Warn("inteiro inválido — usando default", "env", key, "valor", raw, "default", def)
		return def
	}
	return n
}

// durationEnv lê uma duração da env; inválida/ausente cai no default (com log).
func durationEnv(key string, def time.Duration, log *slog.Logger) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		log.Warn("duração inválida — usando default", "env", key, "valor", raw, "default", def)
		return def
	}
	return d
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

// buildPublisher monta o fan-out do resultado: um sink por destino configurado
// (achievements via ACHIEVEMENTS_URL, leaderboard via LEADERBOARD_URL), agrupados
// num Composite (fan-out N sinks). Sem nenhum destino, cai no fallback de log.
// Todos compartilham um client com a internal key (ADR-0018) + trace (otelhttp).
func buildPublisher(log *slog.Logger) domain.ResultPublisher {
	internalKey := os.Getenv("INTERNAL_API_KEY")
	client := &http.Client{
		Timeout:   5 * time.Second,
		Transport: auth.InternalKeyTransport(internalKey, otelhttp.NewTransport(http.DefaultTransport)),
	}

	var sinks []domain.ResultPublisher
	if u := os.Getenv("ACHIEVEMENTS_URL"); u != "" {
		log.Info("fan-out: achievements", "url", u)
		sinks = append(sinks, publisher.NewAchievements(u, client, log))
	}
	if u := os.Getenv("LEADERBOARD_URL"); u != "" {
		log.Info("fan-out: leaderboard", "url", u)
		sinks = append(sinks, publisher.NewLeaderboard(u, client, log))
	}

	if len(sinks) == 0 {
		log.Warn("nenhum destino de fan-out (ACHIEVEMENTS_URL/LEADERBOARD_URL) — só vai logar (fallback)")
		return publisher.New(log)
	}
	if internalKey == "" {
		log.Warn("INTERNAL_API_KEY ausente — o fan-out vai dar 401 nos destinos")
	}
	return publisher.NewComposite(log, sinks...)
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

// Command server é o entrypoint do matchmaking-service: sobe o servidor gRPC
// (FindMatch streaming) + um HTTP pequeno só para /healthz e /metrics, roda o
// loop do matchmaker e faz graceful shutdown dos dois listeners.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	pb "github.com/vitormartins1/go-game-backend/services/matchmaking-service/gen/matchmaking/v1"
	"github.com/vitormartins1/go-game-backend/services/matchmaking-service/internal/profilecache"
	"github.com/vitormartins1/go-game-backend/services/matchmaking-service/internal/registryclient"
	"github.com/vitormartins1/go-game-backend/services/matchmaking-service/internal/service"
	"github.com/vitormartins1/go-game-backend/services/matchmaking-service/internal/sessionclient"
	"github.com/vitormartins1/go-game-backend/services/matchmaking-service/internal/transport"
	sessionv1 "github.com/vitormartins1/go-game-backend/services/session-service/gen/session/v1"
	"github.com/vitormartins1/go-game-backend/shared/auth"
	"github.com/vitormartins1/go-game-backend/shared/config"
	"github.com/vitormartins1/go-game-backend/shared/logger"
	"github.com/vitormartins1/go-game-backend/shared/telemetry"
)

const (
	serviceName     = "matchmaking-service"
	formInterval    = time.Second
	profileCacheTTL = 30 * time.Second // TTL do cache do perfil de multiplayer (ADR-0023)
)

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

	if cfg.IdentityJWKSURL == "" {
		log.Warn("IDENTITY_JWKS_URL ausente — a validação de token vai falhar; aponte para o JWKS do identity")
	}
	verifier := auth.NewVerifier(cfg.IdentityJWKSURL, 5*time.Minute)

	// Conexão gRPC com o session-service: ao formar um match, o matchmaking
	// chama o CreateSession para alocar server + emitir os connect tokens
	// (ADR-0003/0010). otelgrpc no client propaga o trace cross-service (ADR-0014).
	sessionAddr := os.Getenv("SESSION_GRPC_ADDR")
	if sessionAddr == "" {
		log.Warn("SESSION_GRPC_ADDR ausente — o provisionamento de sessão vai falhar; aponte para o gRPC do session")
	}
	sessionConn, err := grpc.NewClient(sessionAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return fmt.Errorf("session grpc client: %w", err)
	}
	defer func() { _ = sessionConn.Close() }()
	provisioner := sessionclient.New(sessionv1.NewSessionClient(sessionConn))

	// Perfil de multiplayer por jogo (ADR-0023): o tamanho do match vem do
	// game-registry. Sem ele o matchmaking não forma nenhum match -> fail-fast no
	// boot. A internal key (ADR-0018) é injetada pelo transport do http.Client.
	gameRegistryURL := os.Getenv("GAME_REGISTRY_URL")
	if gameRegistryURL == "" {
		return fmt.Errorf("GAME_REGISTRY_URL ausente: necessário para derivar o tamanho do match do perfil (ADR-0023)")
	}
	internalKey := os.Getenv("INTERNAL_API_KEY")
	if internalKey == "" {
		log.Warn("INTERNAL_API_KEY ausente — o game-registry vai negar (401); configure a chave")
	}
	profileHTTP := &http.Client{
		Timeout:   5 * time.Second,
		Transport: auth.InternalKeyTransport(internalKey, otelhttp.NewTransport(http.DefaultTransport)),
	}
	provider := profilecache.New(registryclient.New(gameRegistryURL, profileHTTP), profileCacheTTL)

	mm := service.New(formInterval, provider, provisioner)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go mm.Run(ctx)

	// métricas custom (a fila e os matches são o que interessa observar aqui).
	prometheus.MustRegister(
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "matchmaking_queue_size", Help: "Tickets aguardando na fila.",
		}, func() float64 { return float64(mm.QueueSize()) }),
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Name: "matchmaking_matches_total", Help: "Total de matches formados com sucesso.",
		}, mm.MatchesTotal),
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Name: "matchmaking_matches_failed_total", Help: "Total de matches que falharam no provisionamento da sessão.",
		}, mm.MatchesFailed),
	)

	grpcSrv := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()), // tracing gRPC (ADR-0014)
		grpc.StreamInterceptor(transport.StreamAuthInterceptor(verifier)),
	)
	pb.RegisterMatchmakingServer(grpcSrv, transport.NewServer(mm))
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
	mux.Handle("GET /metrics", promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{EnableOpenMetrics: true}))
	httpSrv := &http.Server{Addr: ":" + cfg.Port, Handler: mux, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second}

	errCh := make(chan error, 2)
	go func() {
		log.Info("grpc server iniciando", "addr", grpcLis.Addr().String())
		if err := grpcSrv.Serve(grpcLis); err != nil {
			errCh <- fmt.Errorf("grpc server: %w", err)
		}
	}()
	go func() {
		log.Info("http metrics iniciando", "addr", httpSrv.Addr)
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

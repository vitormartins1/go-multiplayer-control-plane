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

	pb "github.com/vitormartins1/go-multiplayer-control-plane/services/matchmaking-service/gen/matchmaking/v1"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/matchmaking-service/internal/profilecache"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/matchmaking-service/internal/registryclient"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/matchmaking-service/internal/service"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/matchmaking-service/internal/sessionclient"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/matchmaking-service/internal/transport"
	sessionv1 "github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/gen/session/v1"
	"github.com/vitormartins1/go-multiplayer-control-plane/shared/auth"
	"github.com/vitormartins1/go-multiplayer-control-plane/shared/config"
	"github.com/vitormartins1/go-multiplayer-control-plane/shared/logger"
	"github.com/vitormartins1/go-multiplayer-control-plane/shared/telemetry"
)

const (
	serviceName     = "matchmaking-service"
	formInterval    = time.Second
	profileCacheTTL = 30 * time.Second // TTL do cache do perfil de multiplayer (ADR-0023)
	// quadro de sinais de co-op (host_world_dropin, spec 007): TTL do sinal e
	// intervalo do sweeper que expira os vencidos.
	signTTL       = 2 * time.Minute
	sweepInterval = 15 * time.Second
)

// invasionsCounter monta o CounterFunc de invasions_total para um resultado,
// com o rótulo result fixo (ConstLabels) — o valor vem do contador do Invader.
func invasionsCounter(inv *service.Invader, result string) prometheus.Collector {
	return prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name:        "invasions_total",
		Help:        "Total de invasões por resultado (host_world_dropin, spec 006).",
		ConstLabels: prometheus.Labels{"result": result},
	}, func() float64 { return inv.InvasionsTotal(result) })
}

// summonsCounter monta o CounterFunc de summons_total para um resultado, com o
// rótulo result fixo (ConstLabels) — o valor vem do contador do Summoner (spec 007).
func summonsCounter(sum *service.Summoner, result string) prometheus.Collector {
	return prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name:        "summons_total",
		Help:        "Total de sinais de co-op por desfecho (host_world_dropin, spec 007).",
		ConstLabels: prometheus.Labels{"result": result},
	}, func() float64 { return sum.SummonsTotal(result) })
}

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

	verifier, err := auth.NewVerifier(cfg.IdentityJWKSURL, 5*time.Minute)
	if err != nil {
		return fmt.Errorf("IDENTITY_JWKS_URL: %w", err)
	}

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
		// o session reconhece o matchmaking pela internal key (session/030, ADR-0030)
		grpc.WithPerRPCCredentials(sessionclient.InternalKeyCredentials(os.Getenv("INTERNAL_API_KEY"))),
	)
	if err != nil {
		return fmt.Errorf("session grpc client: %w", err)
	}
	defer func() { _ = sessionConn.Close() }()
	// o mesmo adapter satisfaz SessionProvisioner (queue-form), WorldDirectory e
	// WorldInjector (invasão, spec 006) — reusa o client gRPC do session.
	sessionClient := sessionclient.New(sessionv1.NewSessionClient(sessionConn))

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
	// o registryclient cru serve as duas leituras: Format (queue-form, embrulhado
	// no cache do caminho crítico) e InvasionPolicy (invasão, sem cache — spec 006 D2).
	registry := registryclient.New(gameRegistryURL, profileHTTP)
	provider := profilecache.New(registry, profileCacheTTL)

	mm := service.New(formInterval, provider, sessionClient)
	inv := service.NewInvader(registry, sessionClient, sessionClient)
	sum := service.NewSummoner(registry, sessionClient) // co-op por sinais (spec 007)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go mm.Run(ctx)
	go sum.RunSweeper(ctx, signTTL, sweepInterval, log) // expira sinais vencidos

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
		// funil de invasão (host_world_dropin, spec 006): injetadas / sem mundo /
		// não suportado. ConstLabels por resultado (padrão do sessions_closed_total).
		invasionsCounter(inv, "injected"),
		invasionsCounter(inv, "no_world"),
		invasionsCounter(inv, "not_supported"),
		// funil de co-op (host_world_dropin, spec 007): desfecho dos sinais + ativos.
		summonsCounter(sum, "summoned"),
		summonsCounter(sum, "expired"),
		summonsCounter(sum, "cancelled"),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "summon_signs_active", Help: "Sinais de co-op no quadro (host_world_dropin, spec 007).",
		}, sum.SignsActive),
	)

	grpcSrv := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()), // tracing gRPC (ADR-0014)
		grpc.StreamInterceptor(transport.StreamAuthInterceptor(verifier)),
		grpc.ChainUnaryInterceptor(transport.UnaryAuthInterceptor(verifier)), // Invade/SummonSign unary (spec 006/007)
	)
	pb.RegisterMatchmakingServer(grpcSrv, transport.NewServer(mm, inv, sum))
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

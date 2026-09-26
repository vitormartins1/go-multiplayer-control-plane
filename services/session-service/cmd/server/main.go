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

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	pb "github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/gen/session/v1"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/allocator"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/domain"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/profilecache"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/publisher"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/registryclient"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/repository"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/service"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/token"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/transport"
	"github.com/vitormartins1/go-multiplayer-control-plane/shared/auth"
	"github.com/vitormartins1/go-multiplayer-control-plane/shared/config"
	"github.com/vitormartins1/go-multiplayer-control-plane/shared/logger"
	"github.com/vitormartins1/go-multiplayer-control-plane/shared/telemetry"
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

	alloc, registry, fleets, clusterLabel, err := buildAllocator(log)
	if err != nil {
		return err
	}
	pub, err := buildPublisher(context.Background(), log)
	if err != nil {
		return err
	}

	// Postgres próprio (spec 025): primeiro banco do session-service — persiste
	// o histórico de sessões fechadas (a aba Sessions/History some no restart
	// sem isto). Mesmo padrão do achievements-service (pgxpool + otelpgx).
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
	historyStore := repository.NewPostgresHistoryRepository(pool)

	// registry nil (modo static) => WithServerRegistry no-op; reconciliação só no
	// modo agones. Grace protege contra falso-positivo por staleness do LIST.
	svc := service.New(alloc, issuer, pub,
		service.WithProfileProvider(buildProfileProvider(log)), // snapshot do modo (specs 011/017)
		service.WithParticipationPublisher(pub),                // eventos de participação (spec 013)
		service.WithSessionClosedPublisher(pub),                // session health do analytics (spec 028)
		service.WithHistoryLimit(intEnv("SESSION_HISTORY_LIMIT", 200, log)),
		service.WithHistoryStore(historyStore), // persistência de fechadas (spec 025)
		service.WithLogger(log),
		service.WithServerRegistry(registry),
		service.WithOrphanGrace(durationEnv("SESSION_ORPHAN_GRACE", time.Minute, log)))

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
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Name: "sessions_closed_total", Help: "Sessões fechadas, por motivo.",
			ConstLabels: prometheus.Labels{"reason": string(domain.CloseAbandoned)},
		}, func() float64 { return svc.ClosedTotal(domain.CloseAbandoned) }),
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Name: "sessions_closed_total", Help: "Sessões fechadas, por motivo.",
			ConstLabels: prometheus.Labels{"reason": string(domain.CloseHostLeft)},
		}, func() float64 { return svc.ClosedTotal(domain.CloseHostLeft) }),
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Name: "sessions_closed_total", Help: "Sessões fechadas, por motivo.",
			ConstLabels: prometheus.Labels{"reason": string(domain.CloseHostKilled)},
		}, func() float64 { return svc.ClosedTotal(domain.CloseHostKilled) }),
		// eventos de participação por tipo (host_world_dropin, spec 013).
		participationCounter(svc, domain.EventJoined),
		participationCounter(svc, domain.EventBossDefeated),
		participationCounter(svc, domain.EventHostKilled),
		participationCounter(svc, domain.EventDied),
		participationCounter(svc, domain.EventLeft),
		// slots ocupados por papel nos mundos host_world_dropin (spec 011).
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "world_role_slots", Help: "Slots ocupados em mundos host_world_dropin, por papel.",
			ConstLabels: prometheus.Labels{"role": domain.RoleCoop},
		}, func() float64 { return svc.OccupiedSlots(domain.RoleCoop) }),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "world_role_slots", Help: "Slots ocupados em mundos host_world_dropin, por papel.",
			ConstLabels: prometheus.Labels{"role": domain.RoleInvader},
		}, func() float64 { return svc.OccupiedSlots(domain.RoleInvader) }),
		// falhas ao persistir o histórico de sessões fechadas (spec 025) — o
		// fechamento nunca falha por causa disto, mas a métrica denuncia se o
		// Postgres do session-service está fora.
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Name: "session_history_persist_failures_total",
			Help: "Falhas ao persistir sessão fechada no histórico (Postgres indisponível).",
		}, svc.HistoryPersistFailures),
		// falhas ao publicar o fechamento no fan-out (spec 028, session health
		// do analytics-service) — mesmo espírito best-effort da métrica acima.
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Name: "session_closed_publish_failures_total",
			Help: "Falhas ao publicar o fechamento de sessão no fan-out (SNS indisponível).",
		}, svc.SessionClosedPublishFailures),
	)

	authCfg, err := buildGRPCAuth(cfg, log)
	if err != nil {
		return err
	}
	grpcSrv := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()), // tracing gRPC (ADR-0014)
		grpc.UnaryInterceptor(transport.UnaryAuthInterceptor(authCfg)), // spec 030 / ADR-0030
	)
	pb.RegisterSessionServer(grpcSrv, transport.NewServer(svc, transport.WithAuth(authCfg)))
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
	// historyStore alimenta as rotas paginadas do histórico persistido (spec 025).
	transport.NewAdminHandler(svc, historyStore, fleets, clusterLabel).Register(mux)
	// /admin exige a internal key serviço-a-serviço (ADR-0018), como nos demais
	// serviços — inclui o force-close. JWKS, /metrics e /healthz ficam abertos.
	adminKey := os.Getenv("INTERNAL_API_KEY")
	if adminKey == "" {
		log.Warn("INTERNAL_API_KEY ausente — os endpoints /admin vão negar (401); configure a chave")
	}
	httpSrv := &http.Server{Addr: ":" + cfg.Port, Handler: auth.RequireInternalKey(adminKey, "/admin")(mux), ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second}

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
// Devolve também o ServerRegistry (spec 008): no modo agones é o próprio adapter
// (liveness via LIST da fleet); no static é nil — sem fonte de verdade de
// liveness, o sweeper cai só no TTL.
func buildAllocator(log *slog.Logger) (domain.ServerAllocator, domain.ServerRegistry, domain.FleetInspector, string, error) {
	var alloc domain.ServerAllocator
	var registry domain.ServerRegistry
	var fleets domain.FleetInspector
	var clusterLabel string
	switch mode := os.Getenv("ALLOCATOR_MODE"); mode {
	case "", "static":
		pool := serverPool()
		log.Info("allocator static (pool mock)", "size", len(pool))
		alloc = allocator.New(pool)
	case "agones":
		restCfg, err := kubeRestConfig()
		if err != nil {
			return nil, nil, nil, "", fmt.Errorf("allocator agones: %w", err)
		}
		client, err := dynamic.NewForConfig(restCfg)
		if err != nil {
			return nil, nil, nil, "", fmt.Errorf("allocator agones: client: %w", err)
		}
		namespace := envOr("AGONES_NAMESPACE", "default")
		// Fleet não é mais fixa por env var (ADR-0026): cada sessão resolve a
		// Fleet do perfil do jogo (game-registry). Ver rollout na spec 022 —
		// jogos que já usam Agones precisam ter a Fleet cadastrada antes do deploy.
		log.Info("allocator agones", "namespace", namespace)
		ag := allocator.NewAgones(client, namespace)
		alloc, registry, fleets = ag, ag, ag // mesmo adapter serve alocação, liveness e status de fleet
		// AGONES_CLUSTER_LABEL (spec 023): identifica qual cluster este processo
		// observa (ex. "local"/"pi5") — a aba Fleet do console usa isso pra
		// mostrar onde a Fleet está rodando. Vazio = não setado no compose ativo.
		clusterLabel = os.Getenv("AGONES_CLUSTER_LABEL")
	default:
		return nil, nil, nil, "", fmt.Errorf("ALLOCATOR_MODE inválido: %q (static|agones)", mode)
	}

	if host := os.Getenv("DEV_PUBLIC_HOST"); host != "" {
		log.Warn("DEV_PUBLIC_HOST ativo — host dos endereços alocados reescrito (config de dev)", "host", host)
		alloc = allocator.WithPublicHost(alloc, host)
	}
	return alloc, registry, fleets, clusterLabel, nil
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
// participationCounter monta o CounterFunc de participation_events_total para um
// tipo de evento, com o rótulo event fixo (ConstLabels) — o valor vem do contador
// do service (spec 013).
func participationCounter(svc *service.Service, event string) prometheus.Collector {
	return prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name:        "participation_events_total",
		Help:        "Eventos de participação por tipo (host_world_dropin, spec 013).",
		ConstLabels: prometheus.Labels{"event": event},
	}, func() float64 { return svc.ParticipationTotal(event) })
}

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

// fanoutPublisher junta as três Ports do fan-out (win_loss_draw, participation
// e session_closed, spec 028); os adapters SNS e Log satisfazem as três,
// então um valor só é passado como ResultPublisher (posicional) e
// ParticipationPublisher/SessionClosedPublisher (options) no wire-up.
type fanoutPublisher interface {
	domain.ResultPublisher
	domain.ParticipationPublisher
	domain.SessionClosedPublisher
}

// buildPublisher escolhe o adapter do fan-out: SNS (publica no tópico
// match-results) se MATCH_RESULTS_TOPIC_ARN estiver setado; senão o fallback de
// log (dev). O endpoint AWS vem de AWS_ENDPOINT_URL (LocalStack) — ver awsx e
// ADR-0020. A tradução do resultado em eventos migrou para os consumidores.
func buildPublisher(ctx context.Context, log *slog.Logger) (fanoutPublisher, error) {
	arn := os.Getenv("MATCH_RESULTS_TOPIC_ARN")
	if arn == "" {
		log.Warn("MATCH_RESULTS_TOPIC_ARN ausente — fan-out só vai logar (fallback dev)")
		return publisher.New(log), nil
	}
	log.Info("fan-out: SNS", "topic", arn)
	return publisher.NewSNS(ctx, arn, log)
}

// buildProfileProvider monta o adapter que lê o perfil de multiplayer do jogo no
// game-registry (ADR-0023/0024, specs 011/017), com cache TTL por (game, mode).
// Desde o ADR-0024 é obrigatório para QUALQUER criação de sessão (snapshot do
// modo). A internal key (ADR-0018) é injetada no transport do http.Client.
// GAME_REGISTRY_URL ausente => provider nil: Create/OpenWorld falham alto
// (wiring incompleto aparece no primeiro uso, não silenciosamente).
func buildProfileProvider(log *slog.Logger) domain.ProfileProvider {
	baseURL := os.Getenv("GAME_REGISTRY_URL")
	if baseURL == "" {
		log.Warn("GAME_REGISTRY_URL ausente — OpenWorld (host_world_dropin) indisponível; queue_arena não é afetado")
		return nil
	}
	internalKey := os.Getenv("INTERNAL_API_KEY")
	if internalKey == "" {
		log.Warn("INTERNAL_API_KEY ausente — o game-registry vai negar (401) a leitura de perfil; configure a chave")
	}
	httpClient := &http.Client{
		Timeout:   5 * time.Second,
		Transport: auth.InternalKeyTransport(internalKey, otelhttp.NewTransport(http.DefaultTransport)),
	}
	log.Info("profile provider: game-registry", "url", baseURL)
	// cache TTL por (game, mode): o Create não bate no registry a cada partida
	// (ADR-0024, spec 017). Mesmo padrão do matchmaking.
	return profilecache.New(registryclient.New(baseURL, httpClient), 15*time.Second)
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

// buildGRPCAuth monta a autenticação do gRPC (spec 030 / ADR-0030): JWT do
// jogador via JWKS do identity, internal key do matchmaking (ADR-0018) e a
// chave do Dedicated Server, no modo SESSION_AUTH_MODE (padrão warn).
func buildGRPCAuth(cfg config.Config, log *slog.Logger) (transport.AuthConfig, error) {
	mode := transport.ParseAuthMode(os.Getenv("SESSION_AUTH_MODE"))
	ac := transport.AuthConfig{
		Mode:          mode,
		InternalKey:   os.Getenv("INTERNAL_API_KEY"),
		GameServerKey: os.Getenv("GAME_SERVER_KEY"),
		Rejections:    newRejectionCounter(),
		Log:           log,
	}
	if cfg.IdentityJWKSURL != "" {
		v, err := auth.NewVerifier(cfg.IdentityJWKSURL, 5*time.Minute)
		if err != nil {
			return ac, fmt.Errorf("grpc auth: %w", err)
		}
		ac.Players = v
	}
	switch {
	case mode == transport.AuthOff:
		log.Warn("SESSION_AUTH_MODE=off — o gRPC do session aceita qualquer chamador (só para teste local)")
	case ac.Players == nil || ac.GameServerKey == "":
		log.Warn("auth do gRPC incompleta: sem IDENTITY_JWKS_URL nenhum jogador autentica; sem GAME_SERVER_KEY nenhum game server autentica",
			"mode", string(mode))
	}
	log.Info("auth do gRPC do session", "mode", string(mode))
	return ac, nil
}

// rejectionCounter adapta a CounterVec session_auth_rejections_total à Port
// do transport.
type rejectionCounter struct{ vec *prometheus.CounterVec }

func newRejectionCounter() rejectionCounter {
	vec := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "session_auth_rejections_total",
		Help: "Chamadas gRPC ao session sem credencial válida ou sem permissão (spec 030), por RPC e motivo. Em warn a chamada segue.",
	}, []string{"method", "reason"})
	prometheus.MustRegister(vec)
	return rejectionCounter{vec: vec}
}

func (r rejectionCounter) Inc(method, reason string) { r.vec.WithLabelValues(method, reason).Inc() }


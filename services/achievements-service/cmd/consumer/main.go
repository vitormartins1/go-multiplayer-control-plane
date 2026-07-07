// Command consumer é o worker do fan-out durável (ADR-0020): consome a fila SQS
// match-results-achievements e aplica o resultado da partida (match_played /
// match_won) de forma idempotente por matchID. Expõe /healthz e /metrics para o
// Prometheus. Graceful shutdown por context + WaitGroup.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/vitormartins1/go-game-backend/services/achievements-service/internal/awsx"
	"github.com/vitormartins1/go-game-backend/services/achievements-service/internal/consumer"
	"github.com/vitormartins1/go-game-backend/services/achievements-service/internal/domain"
	"github.com/vitormartins1/go-game-backend/services/achievements-service/internal/repository"
	"github.com/vitormartins1/go-game-backend/services/achievements-service/internal/service"
	"github.com/vitormartins1/go-game-backend/shared/config"
	"github.com/vitormartins1/go-game-backend/shared/logger"
)

const serviceName = "achievements-consumer"

func main() {
	log := logger.New(serviceName)
	if err := run(log); err != nil {
		log.Error("encerrando com erro", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg := config.Load()

	queueURL := os.Getenv("MATCH_RESULTS_QUEUE_URL")
	if queueURL == "" {
		return errors.New("MATCH_RESULTS_QUEUE_URL ausente")
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("pgxpool config: %w", err)
	}
	poolCfg.ConnConfig.Tracer = otelpgx.NewTracer()
	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		return fmt.Errorf("pgxpool: %w", err)
	}
	defer pool.Close()

	svc := service.New(repository.NewPostgresAchievementRepository(pool))

	awsCfg, err := awsx.Config(context.Background())
	if err != nil {
		return fmt.Errorf("aws config: %w", err)
	}
	sqsClient := sqs.NewFromConfig(awsCfg, func(o *sqs.Options) {
		o.BaseEndpoint = awsx.BaseEndpoint(o.BaseEndpoint)
	})

	metrics := consumer.NewMetrics(prometheus.DefaultRegisterer)
	c := consumer.New(sqsClient, queueURL, makeHandler(svc, log), log, metrics)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.Run(ctx)
	}()

	srv := metricsServer(cfg.Port)
	errCh := make(chan error, 1)
	go func() {
		log.Info("http (metrics/healthz) iniciando", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		stop()
		wg.Wait()
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
	}

	log.Info("desligando...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	wg.Wait() // espera o loop de consumo encerrar
	log.Info("encerrado")
	return nil
}

// makeHandler desserializa o envelope do fan-out e o despacha por kind (ADR-0023,
// spec 013/009/010): win_loss_draw (queue_arena) → ApplyMatchResult;
// participation_event (host_world_dropin) → ApplyParticipationEvent; placement
// (battle_royale) → ApplyPlacementResult; kind desconhecido → descarta (o SNS faz
// fan-out para todas as filas). Mensagem malformada vai à DLQ.
func makeHandler(svc *service.Service, log *slog.Logger) consumer.Handler {
	return func(ctx context.Context, body []byte) (bool, error) {
		var head struct {
			Kind    string `json:"kind"`
			MatchID string `json:"matchId"`
		}
		if err := json.Unmarshal(body, &head); err != nil {
			return false, fmt.Errorf("unmarshal envelope head: %w", err)
		}

		switch {
		case head.Kind == "participation_event":
			return handleParticipation(ctx, svc, log, body)
		case head.Kind == "placement":
			// battle_royale (session/015): tem matchId, então precede o ramo abaixo.
			return handlePlacement(ctx, svc, log, body)
		case head.Kind == "win_loss_draw" || head.MatchID != "":
			// win_loss_draw explícito, ou envelope v1 legado (sem kind, com matchId).
			return handleMatchResult(ctx, svc, log, body)
		default:
			// kind desconhecido/ausente sem matchId: não é nosso — descarta (não DLQ).
			log.Debug("envelope ignorado (kind não tratado)", slog.String("kind", head.Kind))
			return false, nil
		}
	}
}

// handleMatchResult aplica o resultado win_loss_draw (queue_arena), idempotente
// por matchId.
func handleMatchResult(ctx context.Context, svc *service.Service, log *slog.Logger, body []byte) (bool, error) {
	var env struct {
		MatchID string `json:"matchId"`
		GameID  string `json:"gameId"`
		Results []struct {
			PlayerID string           `json:"playerId"`
			Won      bool             `json:"won"`
			Outcome  string           `json:"outcome"` // ADR-0023 (v2); ausente no v1 -> fallback no Won
			Stats    map[string]int64 `json:"stats"`   // ADR-0023 A1: critério por métrica
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return false, fmt.Errorf("unmarshal match envelope: %w", err)
	}
	results := make([]domain.MatchResult, 0, len(env.Results))
	for _, r := range env.Results {
		results = append(results, domain.MatchResult{PlayerID: r.PlayerID, Won: r.Won, Outcome: r.Outcome, Stats: r.Stats})
	}
	dup, err := svc.ApplyMatchResult(ctx, env.GameID, env.MatchID, results)
	if err != nil {
		return false, err
	}
	if dup {
		log.Info("partida já processada (no-op)", slog.String("match_id", env.MatchID))
	}
	return dup, nil
}

// handlePlacement aplica o resultado por colocação (battle_royale, session/015),
// idempotente por matchId. survivedMs é exposto como stat no service.
func handlePlacement(ctx context.Context, svc *service.Service, log *slog.Logger, body []byte) (bool, error) {
	var env struct {
		MatchID string `json:"matchId"`
		GameID  string `json:"gameId"`
		Entries []struct {
			PlayerID   string           `json:"playerId"`
			Placement  int              `json:"placement"`
			SurvivedMs int64            `json:"survivedMs"`
			Stats      map[string]int64 `json:"stats"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return false, fmt.Errorf("unmarshal placement envelope: %w", err)
	}
	entries := make([]domain.PlacementEntry, 0, len(env.Entries))
	for _, e := range env.Entries {
		entries = append(entries, domain.PlacementEntry{
			PlayerID: e.PlayerID, Placement: e.Placement, SurvivedMs: e.SurvivedMs, Stats: e.Stats,
		})
	}
	dup, err := svc.ApplyPlacementResult(ctx, env.GameID, env.MatchID, entries)
	if err != nil {
		return false, err
	}
	if dup {
		log.Info("partida (placement) já processada (no-op)", slog.String("match_id", env.MatchID))
	}
	return dup, nil
}

// handleParticipation aplica um evento de participação (host_world_dropin),
// idempotente por (sessionId, playerId, event).
func handleParticipation(ctx context.Context, svc *service.Service, log *slog.Logger, body []byte) (bool, error) {
	var env struct {
		GameID    string `json:"gameId"`
		SessionID string `json:"sessionId"`
		PlayerID  string `json:"playerId"`
		Role      string `json:"role"`
		Event     string `json:"event"`
		Outcome   string `json:"outcome"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return false, fmt.Errorf("unmarshal participation envelope: %w", err)
	}
	dup, err := svc.ApplyParticipationEvent(ctx, env.GameID, env.SessionID, env.PlayerID, env.Role, env.Event, env.Outcome)
	if err != nil {
		return false, err
	}
	if dup {
		log.Info("evento de participação já processado (no-op)",
			slog.String("session_id", env.SessionID), slog.String("player_id", env.PlayerID), slog.String("event", env.Event))
	}
	return dup, nil
}

func metricsServer(port string) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.Handle("GET /metrics", promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{EnableOpenMetrics: true}))
	return &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
}

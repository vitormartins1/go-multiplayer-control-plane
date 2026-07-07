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

// makeHandler desserializa o envelope do fan-out e aplica o resultado idempotente.
// Mensagem malformada retorna erro (vai à DLQ) em vez de ser descartada.
func makeHandler(svc *service.Service, log *slog.Logger) consumer.Handler {
	return func(ctx context.Context, body []byte) (bool, error) {
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
			return false, fmt.Errorf("unmarshal envelope: %w", err)
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

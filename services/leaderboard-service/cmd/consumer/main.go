// Command consumer é o worker do fan-out durável (ADR-0020): consome a fila SQS
// match-results-leaderboard e submete o score de cada jogador. Não precisa de
// dedup: o Submit usa ZADD GT (só sobe se maior), então reprocessar a mesma
// partida (entrega at-least-once) é no-op. Expõe /healthz e /metrics.
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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/vitormartins1/go-game-backend/services/leaderboard-service/internal/awsx"
	"github.com/vitormartins1/go-game-backend/services/leaderboard-service/internal/consumer"
	"github.com/vitormartins1/go-game-backend/services/leaderboard-service/internal/domain"
	"github.com/vitormartins1/go-game-backend/services/leaderboard-service/internal/service"
	"github.com/vitormartins1/go-game-backend/services/leaderboard-service/internal/storage"
	"github.com/vitormartins1/go-game-backend/shared/config"
	"github.com/vitormartins1/go-game-backend/shared/logger"
)

const (
	serviceName          = "leaderboard-consumer"
	leaderboardKeyPrefix = "leaderboard"
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

	queueURL := os.Getenv("MATCH_RESULTS_QUEUE_URL")
	if queueURL == "" {
		return errors.New("MATCH_RESULTS_QUEUE_URL ausente")
	}

	store := storage.NewRedisStore(cfg.RedisAddr, leaderboardKeyPrefix)
	if err := pingWithRetry(context.Background(), store, log); err != nil {
		return fmt.Errorf("redis: %w", err)
	}
	svc := service.New(store)

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
	wg.Wait()
	log.Info("encerrado")
	return nil
}

// makeHandler desserializa o envelope e submete o score de cada jogador. Sem
// dedup: ZADD GT é idempotente (reprocessar não altera o top-N).
func makeHandler(svc *service.Service, log *slog.Logger) consumer.Handler {
	return func(ctx context.Context, body []byte) (bool, error) {
		var env struct {
			MatchID string `json:"matchId"`
			GameID  string `json:"gameId"`
			Results []struct {
				PlayerID string           `json:"playerId"`
				Score    int64            `json:"score"`
				Stats    map[string]int64 `json:"stats"` // ADR-0023: um board por métrica
			} `json:"results"`
		}
		if err := json.Unmarshal(body, &env); err != nil {
			return false, fmt.Errorf("unmarshal envelope: %w", err)
		}
		for _, r := range env.Results {
			// board "score" (como sempre) + um board por stat presente.
			if _, err := svc.Submit(ctx, env.GameID, domain.BoardScore, r.PlayerID, r.Score); err != nil {
				return false, fmt.Errorf("submit %s score: %w", r.PlayerID, err)
			}
			for stat, val := range r.Stats {
				if !domain.ValidBoard(stat) {
					continue // nome inválido: pula (o game-registry já valida scoringStats)
				}
				if _, err := svc.Submit(ctx, env.GameID, stat, r.PlayerID, val); err != nil {
					return false, fmt.Errorf("submit %s %s: %w", r.PlayerID, stat, err)
				}
			}
		}
		return false, nil
	}
}

func pingWithRetry(ctx context.Context, store *storage.RedisStore, log *slog.Logger) error {
	var lastErr error
	for i := 0; i < 15; i++ {
		if err := store.Ping(ctx); err == nil {
			return nil
		} else {
			lastErr = err
		}
		log.Warn("redis ainda não pronto; tentando de novo")
		time.Sleep(time.Second)
	}
	return lastErr
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

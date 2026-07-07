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

// makeHandler despacha o envelope por kind (ADR-0023): win_loss_draw (queue_arena)
// e placement (battle_royale, BR3) alimentam boards; kinds sem ranking (ex.:
// participation_event) e desconhecidos são descartados. Cada modo aplica highscore
// (ZADD GT — idempotente) + cumulative (ZINCRBY idempotente por matchID via
// ApplyCumulative, uma chamada atômica por partida).
func makeHandler(svc *service.Service, log *slog.Logger) consumer.Handler {
	return func(ctx context.Context, body []byte) (bool, error) {
		kind, matchID := peekKind(body)
		switch {
		case kind == "placement":
			return handlePlacement(ctx, svc, log, body)
		case kind == "win_loss_draw" || matchID != "":
			// win_loss_draw explícito, ou envelope v1 legado (sem kind, com matchId).
			return handleWinLossDraw(ctx, svc, log, body)
		default:
			// kind sem ranking (participation_event) ou desconhecido: não é nosso.
			log.Debug("envelope ignorado (kind não rankeável)", slog.String("kind", kind))
			return false, nil
		}
	}
}

// peekKind espia kind e matchId sem desserializar o corpo todo (dispatch).
func peekKind(body []byte) (kind, matchID string) {
	var head struct {
		Kind    string `json:"kind"`
		MatchID string `json:"matchId"`
	}
	_ = json.Unmarshal(body, &head) // malformado cai no parse específico (DLQ)
	return head.Kind, head.MatchID
}

// handleWinLossDraw aplica o resultado do queue_arena: highscore (board "score" +
// um por stat) e cumulative (vitórias + "<stat>_total").
func handleWinLossDraw(ctx context.Context, svc *service.Service, log *slog.Logger, body []byte) (bool, error) {
	var env struct {
		MatchID string `json:"matchId"`
		GameID  string `json:"gameId"`
		Results []struct {
			PlayerID string           `json:"playerId"`
			Score    int64            `json:"score"`
			Outcome  string           `json:"outcome"` // ADR-0023 A2: ranking de vitórias
			Stats    map[string]int64 `json:"stats"`   // um board highscore + um _total por métrica
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return false, fmt.Errorf("unmarshal envelope: %w", err)
	}

	var incrs []domain.Increment // parcelas cumulativas (todas as partes numa chamada)
	for _, r := range env.Results {
		// highscore: board "score" (como sempre) + um board por stat presente.
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
			// cumulative: soma do stat na temporada ("<stat>_total").
			incrs = append(incrs, domain.Increment{Board: domain.TotalBoard(stat), PlayerID: r.PlayerID, Delta: val})
		}
		// cumulative: ranking de vitórias.
		if r.Outcome == outcomeWin {
			incrs = append(incrs, domain.Increment{Board: domain.BoardWins, PlayerID: r.PlayerID, Delta: 1})
		}
	}

	dup, err := svc.ApplyCumulative(ctx, env.GameID, env.MatchID, incrs)
	if err != nil {
		return false, fmt.Errorf("apply cumulative: %w", err)
	}
	if dup {
		log.Info("cumulativo já aplicado (no-op)", slog.String("match_id", env.MatchID))
	}
	return dup, nil
}

// handlePlacement aplica o resultado por colocação (battle_royale, BR3): highscore
// da sobrevivência (survived_ms) + cada stat; cumulative "wins" (1º lugar),
// "top_n_finishes" (colocação <= limiar) e "<stat>_total". Idempotente por matchID.
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

	var incrs []domain.Increment
	for _, e := range env.Entries {
		// highscore: melhor sobrevivência de uma partida + um board por stat.
		if _, err := svc.Submit(ctx, env.GameID, domain.BoardSurvivedMs, e.PlayerID, e.SurvivedMs); err != nil {
			return false, fmt.Errorf("submit %s survived_ms: %w", e.PlayerID, err)
		}
		for stat, val := range e.Stats {
			if !domain.ValidBoard(stat) {
				continue
			}
			if _, err := svc.Submit(ctx, env.GameID, stat, e.PlayerID, val); err != nil {
				return false, fmt.Errorf("submit %s %s: %w", e.PlayerID, stat, err)
			}
			incrs = append(incrs, domain.Increment{Board: domain.TotalBoard(stat), PlayerID: e.PlayerID, Delta: val})
		}
		// cumulative: vitória (1º lugar) e "terminou no top N" (o 1º conta nos dois).
		if e.Placement == 1 {
			incrs = append(incrs, domain.Increment{Board: domain.BoardWins, PlayerID: e.PlayerID, Delta: 1})
		}
		if e.Placement <= domain.BattleRoyaleTopNThreshold {
			incrs = append(incrs, domain.Increment{Board: domain.BoardTopNFinishes, PlayerID: e.PlayerID, Delta: 1})
		}
	}

	dup, err := svc.ApplyCumulative(ctx, env.GameID, env.MatchID, incrs)
	if err != nil {
		return false, fmt.Errorf("apply cumulative (placement): %w", err)
	}
	if dup {
		log.Info("cumulativo (placement) já aplicado (no-op)", slog.String("match_id", env.MatchID))
	}
	return dup, nil
}

// outcomeWin é o valor do outcome que conta como vitória no ranking (ADR-0023).
const outcomeWin = "win"

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

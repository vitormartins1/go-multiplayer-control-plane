// Package publisher implementa o fan-out do resultado da partida
// (domain.ResultPublisher): o adapter SNS publica o resultado no tópico
// match-results (ADR-0020); Log é um fallback que apenas loga (dev, sem
// MATCH_RESULTS_TOPIC_ARN).
package publisher

import (
	"context"
	"log/slog"

	"github.com/vitormartins1/go-game-backend/services/session-service/internal/domain"
)

var _ domain.ResultPublisher = (*Log)(nil)

// Log é o ResultPublisher de fallback: registra o resultado em vez de publicar
// no tópico (quando MATCH_RESULTS_TOPIC_ARN não está configurado).
type Log struct {
	log *slog.Logger
}

// New cria o publisher de fallback (log).
func New(log *slog.Logger) *Log {
	return &Log{log: log}
}

// Publish loga o resultado da partida (fallback do fan-out real).
func (l *Log) Publish(_ context.Context, gameID, matchID string, results []domain.PlayerResult) error {
	for _, r := range results {
		l.log.Info("result fan-out (log fallback)",
			slog.String("game_id", gameID),
			slog.String("match_id", matchID),
			slog.String("player_id", r.PlayerID),
			slog.Int64("score", r.Score),
			slog.Bool("won", r.Won),
		)
	}
	return nil
}

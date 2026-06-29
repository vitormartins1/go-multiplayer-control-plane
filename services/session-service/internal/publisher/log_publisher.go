// Package publisher implementa o fan-out do resultado da partida. No MVP é um
// stub que apenas loga (domain.ResultPublisher). O fan-out HTTP real para
// leaderboard/achievements exige endpoints server-side + service-to-service
// auth e é a próxima feature (ver spec "Fora do escopo").
package publisher

import (
	"context"
	"log/slog"

	"github.com/c130n/game-backend/services/session-service/internal/domain"
)

var _ domain.ResultPublisher = (*Log)(nil)

// Log é o ResultPublisher stub: registra o resultado em vez de fazer o fan-out.
type Log struct {
	log *slog.Logger
}

// New cria o publisher stub.
func New(log *slog.Logger) *Log {
	return &Log{log: log}
}

// Publish loga o resultado da partida (placeholder do fan-out real).
func (l *Log) Publish(_ context.Context, matchID string, results []domain.PlayerResult) error {
	for _, r := range results {
		l.log.Info("result fan-out (stub)",
			slog.String("match_id", matchID),
			slog.String("player_id", r.PlayerID),
			slog.Int64("score", r.Score),
			slog.Bool("won", r.Won),
		)
	}
	return nil
}

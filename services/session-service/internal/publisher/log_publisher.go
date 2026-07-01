// Package publisher implementa o fan-out do resultado da partida
// (domain.ResultPublisher): o adapter Achievements fala HTTP com o achievements;
// Log é um fallback que apenas loga (dev, sem ACHIEVEMENTS_URL).
package publisher

import (
	"context"
	"log/slog"

	"github.com/vitormartins1/go-game-backend/services/session-service/internal/domain"
)

var _ domain.ResultPublisher = (*Log)(nil)

// Log é o ResultPublisher de fallback: registra o resultado em vez de fazer o
// fan-out (quando ACHIEVEMENTS_URL não está configurado).
type Log struct {
	log *slog.Logger
}

// New cria o publisher de fallback (log).
func New(log *slog.Logger) *Log {
	return &Log{log: log}
}

// Publish loga o resultado da partida (fallback do fan-out real).
func (l *Log) Publish(_ context.Context, gameID string, results []domain.PlayerResult) error {
	for _, r := range results {
		l.log.Info("result fan-out (log fallback)",
			slog.String("game_id", gameID),
			slog.String("player_id", r.PlayerID),
			slog.Int64("score", r.Score),
			slog.Bool("won", r.Won),
		)
	}
	return nil
}

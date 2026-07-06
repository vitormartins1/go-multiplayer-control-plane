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
func (l *Log) Publish(_ context.Context, gameID, matchID string, outcome domain.MatchOutcome) error {
	var players []domain.PlayerOutcome
	if outcome.WinLossDraw != nil {
		players = outcome.WinLossDraw.Players
	}
	for _, p := range players {
		l.log.Info("result fan-out (log fallback)",
			slog.String("game_id", gameID),
			slog.String("match_id", matchID),
			slog.String("kind", outcome.Kind),
			slog.String("player_id", p.PlayerID),
			slog.String("team_id", p.TeamID),
			slog.String("outcome", p.Outcome),
			slog.Int64("score", p.Score),
			slog.Any("stats", p.Stats),
		)
	}
	return nil
}

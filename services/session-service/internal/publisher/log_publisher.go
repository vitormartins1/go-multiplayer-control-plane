// Package publisher implementa o fan-out do resultado da partida
// (domain.ResultPublisher): o adapter SNS publica o resultado no tópico
// match-results (ADR-0020); Log é um fallback que apenas loga (dev, sem
// MATCH_RESULTS_TOPIC_ARN).
package publisher

import (
	"context"
	"log/slog"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/domain"
)

var (
	_ domain.ResultPublisher        = (*Log)(nil)
	_ domain.ParticipationPublisher = (*Log)(nil)
)

// Log é o ResultPublisher de fallback: registra o resultado em vez de publicar
// no tópico (quando MATCH_RESULTS_TOPIC_ARN não está configurado).
type Log struct {
	log *slog.Logger
}

// New cria o publisher de fallback (log).
func New(log *slog.Logger) *Log {
	return &Log{log: log}
}

// Publish loga o resultado da partida (fallback do fan-out real), despachando
// por kind (win_loss_draw / placement / collective).
func (l *Log) Publish(_ context.Context, gameID, matchID string, outcome domain.MatchOutcome) error {
	if outcome.Collective != nil {
		for _, p := range outcome.Collective.Players {
			l.log.Info("collective fan-out (log fallback)",
				slog.String("game_id", gameID),
				slog.String("match_id", matchID),
				slog.String("kind", outcome.Kind),
				slog.String("outcome", outcome.Collective.Outcome),
				slog.String("player_id", p.PlayerID),
				slog.String("team_id", p.TeamID),
				slog.Any("stats", p.Stats),
			)
		}
		return nil
	}
	if outcome.Placement != nil {
		for _, e := range outcome.Placement.Entries {
			l.log.Info("placement fan-out (log fallback)",
				slog.String("game_id", gameID),
				slog.String("match_id", matchID),
				slog.String("kind", outcome.Kind),
				slog.String("player_id", e.PlayerID),
				slog.String("team_id", e.TeamID),
				slog.Int("placement", e.Placement),
				slog.Int64("survived_ms", e.SurvivedMs),
				slog.Any("stats", e.Stats),
			)
		}
		return nil
	}

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

// PublishParticipation loga o evento de participação (fallback do fan-out real).
func (l *Log) PublishParticipation(_ context.Context, gameID, sessionID string, evt domain.ParticipationEvent) error {
	l.log.Info("participation fan-out (log fallback)",
		slog.String("game_id", gameID),
		slog.String("session_id", sessionID),
		slog.String("player_id", evt.PlayerID),
		slog.String("role", evt.Role),
		slog.String("event", evt.Event),
		slog.String("outcome", evt.Outcome),
	)
	return nil
}

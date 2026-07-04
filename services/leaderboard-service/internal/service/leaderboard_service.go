// Package service contém a regra de negócio do leaderboard-service.
// Só depende das interfaces de domain (ADR-0002).
package service

import (
	"context"
	"fmt"

	"github.com/vitormartins1/go-game-backend/services/leaderboard-service/internal/domain"
)

const (
	defaultLimit  = 10
	maxLimit      = 100
	defaultAround = 5
	maxAround     = 50
)

// garante em tempo de compilação que satisfazemos a Port de entrada.
var _ domain.LeaderboardService = (*Service)(nil)

// Service orquestra submissão e leitura do ranking.
type Service struct {
	store domain.LeaderboardStore
}

// New cria o service a partir do store.
func New(store domain.LeaderboardStore) *Service {
	return &Service{store: store}
}

// Submit aplica o highscore e devolve a posição/score do jogador no jogo.
func (s *Service) Submit(ctx context.Context, gameID, playerID string, score int64) (domain.Entry, error) {
	if err := s.store.Submit(ctx, gameID, playerID, score); err != nil {
		return domain.Entry{}, fmt.Errorf("leaderboard service: submit: %w", err)
	}
	rank0, best, err := s.store.Rank(ctx, gameID, playerID)
	if err != nil {
		return domain.Entry{}, fmt.Errorf("leaderboard service: rank: %w", err)
	}
	return domain.Entry{Rank: rank0 + 1, PlayerID: playerID, Score: best}, nil
}

// ForceSubmit substitui o score independente do valor atual (admin override).
func (s *Service) ForceSubmit(ctx context.Context, gameID, playerID string, score int64) (domain.Entry, error) {
	if err := s.store.ForceScore(ctx, gameID, playerID, score); err != nil {
		return domain.Entry{}, fmt.Errorf("leaderboard service: force submit: %w", err)
	}
	rank0, best, err := s.store.Rank(ctx, gameID, playerID)
	if err != nil {
		return domain.Entry{}, fmt.Errorf("leaderboard service: rank after force: %w", err)
	}
	return domain.Entry{Rank: rank0 + 1, PlayerID: playerID, Score: best}, nil
}

func (s *Service) Top(ctx context.Context, gameID string, limit int) ([]domain.Entry, error) {
	entries, err := s.store.Top(ctx, gameID, clampLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("leaderboard service: top: %w", err)
	}
	return entries, nil
}

func (s *Service) Around(ctx context.Context, gameID, playerID string, k int) ([]domain.Entry, error) {
	k = clampAround(k)
	rank0, _, err := s.store.Rank(ctx, gameID, playerID)
	if err != nil {
		return nil, fmt.Errorf("leaderboard service: around: %w", err) // ErrNotRanked propaga
	}
	start := rank0 - int64(k)
	if start < 0 {
		start = 0
	}
	stop := rank0 + int64(k)
	entries, err := s.store.Range(ctx, gameID, start, stop)
	if err != nil {
		return nil, fmt.Errorf("leaderboard service: around range: %w", err)
	}
	return entries, nil
}

func clampLimit(n int) int {
	switch {
	case n < 1:
		return defaultLimit
	case n > maxLimit:
		return maxLimit
	default:
		return n
	}
}

func clampAround(k int) int {
	switch {
	case k < 0:
		return defaultAround
	case k > maxAround:
		return maxAround
	default:
		return k
	}
}

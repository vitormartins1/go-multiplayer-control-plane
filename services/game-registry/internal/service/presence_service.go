package service

import (
	"context"
	"fmt"

	"github.com/c130n/game-backend/services/game-registry/internal/domain"
)

var _ domain.PresenceService = (*presenceService)(nil)

type presenceService struct {
	repo domain.PresenceRepository
}

// NewPresence cria o PresenceService do game-registry.
func NewPresence(repo domain.PresenceRepository) domain.PresenceService {
	return &presenceService{repo: repo}
}

func (s *presenceService) Mark(ctx context.Context, gameID, playerID, handle string) error {
	if err := s.repo.Upsert(ctx, gameID, playerID, handle); err != nil {
		return fmt.Errorf("game-registry service: mark presence: %w", err)
	}
	return nil
}

func (s *presenceService) PlayersOf(ctx context.Context, gameID string) ([]domain.Presence, error) {
	players, err := s.repo.ListByGame(ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("game-registry service: players of: %w", err)
	}
	return players, nil
}

func (s *presenceService) GamesOf(ctx context.Context, playerID string) ([]domain.Game, error) {
	games, err := s.repo.GamesByPlayer(ctx, playerID)
	if err != nil {
		return nil, fmt.Errorf("game-registry service: games of: %w", err)
	}
	return games, nil
}

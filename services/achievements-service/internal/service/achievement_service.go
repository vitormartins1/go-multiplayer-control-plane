// Package service contém a regra de negócio do achievements-service.
// Só depende das interfaces de domain (ADR-0002).
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/c130n/game-backend/services/achievements-service/internal/domain"
)

// garante em tempo de compilação que satisfazemos a Port de entrada.
var _ domain.AchievementService = (*Service)(nil)

// Service implementa as regras de catálogo e desbloqueio de conquistas.
type Service struct {
	repo domain.AchievementRepository
	now  func() time.Time
}

// New cria o service a partir do repositório.
func New(repo domain.AchievementRepository) *Service {
	return &Service{repo: repo, now: time.Now}
}

// ListCatalog devolve o catálogo de conquistas.
func (s *Service) ListCatalog(ctx context.Context) ([]domain.Achievement, error) {
	cat, err := s.repo.ListCatalog(ctx)
	if err != nil {
		return nil, fmt.Errorf("achievements service: list catalog: %w", err)
	}
	return cat, nil
}

// ListForPlayer devolve o catálogo anotado com o estado de desbloqueio do jogador.
func (s *Service) ListForPlayer(ctx context.Context, playerID string) ([]domain.PlayerAchievement, error) {
	cat, err := s.repo.ListCatalog(ctx)
	if err != nil {
		return nil, fmt.Errorf("achievements service: list for player: %w", err)
	}
	unlocks, err := s.repo.ListPlayerUnlocks(ctx, playerID)
	if err != nil {
		return nil, fmt.Errorf("achievements service: list for player: %w", err)
	}

	unlockedAt := make(map[string]time.Time, len(unlocks))
	for _, u := range unlocks {
		unlockedAt[u.AchievementID] = u.UnlockedAt
	}

	out := make([]domain.PlayerAchievement, 0, len(cat))
	for _, a := range cat {
		pa := domain.PlayerAchievement{Achievement: a}
		if at, ok := unlockedAt[a.ID]; ok {
			pa.Unlocked = true
			pa.UnlockedAt = at
		}
		out = append(out, pa)
	}
	return out, nil
}

// RecordEvent avalia o evento contra o catálogo e desbloqueia as conquistas
// cujo critério (event_type) casa, de forma idempotente. Retorna apenas as
// recém-desbloqueadas nesta chamada.
func (s *Service) RecordEvent(ctx context.Context, playerID string, event domain.Event) ([]domain.Achievement, error) {
	if event.Type == "" {
		return nil, domain.ErrInvalidEvent
	}

	matches, err := s.repo.FindByEventType(ctx, event.Type)
	if err != nil {
		return nil, fmt.Errorf("achievements service: record event: %w", err)
	}

	at := s.now().UTC()
	var unlocked []domain.Achievement
	for _, a := range matches {
		created, err := s.repo.Unlock(ctx, playerID, a.ID, at)
		if err != nil {
			return nil, fmt.Errorf("achievements service: unlock %q: %w", a.ID, err)
		}
		if created {
			unlocked = append(unlocked, a)
		}
	}
	return unlocked, nil
}

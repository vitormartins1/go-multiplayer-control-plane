// Package service contém a regra de negócio do achievements-service.
// Só depende das interfaces de domain (ADR-0002). Tudo escopado por gameID
// (multi-tenant, ADR-0017).
package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/vitormartins1/go-game-backend/services/achievements-service/internal/domain"
)

// garante em tempo de compilação que satisfazemos a Port de entrada.
var _ domain.AchievementService = (*Service)(nil)

// slugRE valida o id (slug) de uma conquista: 2–40, minúsculas/dígitos/hífen/
// underscore, sem separador nas pontas. Permite o estilo snake_case do catálogo
// (ex.: "first_win", "ten_wins").
var slugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,38}[a-z0-9]$`)

const maxAchievementNameLen = 100

// Service implementa as regras de catálogo e desbloqueio de conquistas.
type Service struct {
	repo domain.AchievementRepository
	now  func() time.Time
}

// New cria o service a partir do repositório.
func New(repo domain.AchievementRepository) *Service {
	return &Service{repo: repo, now: time.Now}
}

// ListCatalog devolve o catálogo de conquistas do jogo.
func (s *Service) ListCatalog(ctx context.Context, gameID string) ([]domain.Achievement, error) {
	cat, err := s.repo.ListCatalog(ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("achievements service: list catalog: %w", err)
	}
	return cat, nil
}

// ListForPlayer devolve o catálogo do jogo anotado com o estado de desbloqueio
// do jogador nesse jogo.
func (s *Service) ListForPlayer(ctx context.Context, gameID, playerID string) ([]domain.PlayerAchievement, error) {
	cat, err := s.repo.ListCatalog(ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("achievements service: list for player: %w", err)
	}
	progress, err := s.repo.ListPlayerProgress(ctx, gameID, playerID)
	if err != nil {
		return nil, fmt.Errorf("achievements service: list for player: %w", err)
	}

	byID := make(map[string]domain.PlayerProgress, len(progress))
	for _, p := range progress {
		byID[p.AchievementID] = p
	}

	out := make([]domain.PlayerAchievement, 0, len(cat))
	for _, a := range cat {
		pa := domain.PlayerAchievement{Achievement: a}
		if p, ok := byID[a.ID]; ok {
			pa.Progress = min(p.Progress, a.Target) // nunca exibir além da meta
			if !p.UnlockedAt.IsZero() {
				pa.Unlocked = true
				pa.UnlockedAt = p.UnlockedAt
			}
		}
		out = append(out, pa)
	}
	return out, nil
}

// RecordEvent avalia o evento contra o catálogo do jogo e desbloqueia as
// conquistas cujo critério (event_type) casa, de forma idempotente. Retorna
// apenas as recém-desbloqueadas nesta chamada.
func (s *Service) RecordEvent(ctx context.Context, gameID, playerID string, event domain.Event) ([]domain.Achievement, error) {
	if event.Type == "" {
		return nil, domain.ErrInvalidEvent
	}

	matches, err := s.repo.FindByEventType(ctx, gameID, event.Type)
	if err != nil {
		return nil, fmt.Errorf("achievements service: record event: %w", err)
	}

	at := s.now().UTC()
	var unlocked []domain.Achievement
	for _, a := range matches {
		justUnlocked, err := s.repo.RecordProgress(ctx, gameID, playerID, a.ID, a.Target, at)
		if err != nil {
			return nil, fmt.Errorf("achievements service: record progress %q: %w", a.ID, err)
		}
		if justUnlocked {
			unlocked = append(unlocked, a)
		}
	}
	return unlocked, nil
}

// CreateAchievement valida e adiciona uma conquista ao catálogo do jogo (admin).
func (s *Service) CreateAchievement(ctx context.Context, gameID string, a domain.Achievement) (*domain.Achievement, error) {
	if !slugRE.MatchString(a.ID) {
		return nil, domain.ErrInvalidAchievement
	}
	a.Name = strings.TrimSpace(a.Name)
	if a.Name == "" || len(a.Name) > maxAchievementNameLen {
		return nil, domain.ErrInvalidAchievement
	}
	if a.EventType == "" {
		return nil, domain.ErrInvalidAchievement
	}
	if a.Target < 1 {
		a.Target = 1
	}
	a.GameID = gameID

	if err := s.repo.CreateAchievement(ctx, &a); err != nil {
		return nil, fmt.Errorf("achievements service: create achievement: %w", err)
	}
	return &a, nil
}

// UpdateAchievement edita os campos mutáveis da conquista (id/gameID são a
// chave). Valida name/eventType/target como na criação; o slug não é revalidado
// (vem do path, já existente).
func (s *Service) UpdateAchievement(ctx context.Context, gameID, id string, a domain.Achievement) (*domain.Achievement, error) {
	a.Name = strings.TrimSpace(a.Name)
	if a.Name == "" || len(a.Name) > maxAchievementNameLen {
		return nil, domain.ErrInvalidAchievement
	}
	if a.EventType == "" {
		return nil, domain.ErrInvalidAchievement
	}
	if a.Target < 1 {
		a.Target = 1
	}
	a.GameID = gameID
	a.ID = id

	if err := s.repo.UpdateAchievement(ctx, &a); err != nil {
		return nil, fmt.Errorf("achievements service: update achievement: %w", err)
	}
	return &a, nil
}

// DeleteAchievement remove a conquista e o progresso dos jogadores nela.
func (s *Service) DeleteAchievement(ctx context.Context, gameID, id string) error {
	if err := s.repo.DeleteAchievement(ctx, gameID, id); err != nil {
		return fmt.Errorf("achievements service: delete achievement: %w", err)
	}
	return nil
}

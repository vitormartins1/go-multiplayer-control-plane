// Package service contém a regra de negócio do cloud-save-service.
// Só depende das interfaces de domain (ADR-0002).
package service

import (
	"context"
	"fmt"

	"github.com/c130n/game-backend/services/cloud-save-service/internal/domain"
)

// garante em tempo de compilação que satisfazemos a Port de entrada.
var _ domain.CloudSaveService = (*Service)(nil)

// Service orquestra o save/load do estado do jogo.
type Service struct {
	store domain.SaveStore
}

// New cria o service a partir do store de saves.
func New(store domain.SaveStore) *Service {
	return &Service{store: store}
}

func (s *Service) Save(ctx context.Context, playerID string, snapshot domain.Snapshot) error {
	if err := s.store.Put(ctx, playerID, snapshot); err != nil {
		return fmt.Errorf("cloud-save service: save: %w", err)
	}
	return nil
}

func (s *Service) Load(ctx context.Context, playerID string) (domain.Snapshot, error) {
	snap, err := s.store.Get(ctx, playerID)
	if err != nil {
		return domain.Snapshot{}, fmt.Errorf("cloud-save service: load: %w", err)
	}
	return snap, nil
}

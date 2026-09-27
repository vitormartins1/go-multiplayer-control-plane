package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/identity-service/internal/domain"
)

// Limites de paginação do listing admin (feature 003-admin-list-users).
const (
	defaultLimit = 50
	maxLimit     = 200
)

var _ domain.PlayerLister = (*playerLister)(nil)

// playerLister implementa o listing admin de contas (read-only). Concern
// separado da autenticação — só precisa do repositório.
type playerLister struct {
	repo     domain.AccountRepository
	external domain.ExternalIdentityLister // nil = sem login federado: nenhum vínculo
}

// ListerOption configura o PlayerLister.
type ListerOption func(*playerLister)

// WithExternalIdentities liga a leitura dos vínculos de loja (spec 006).
func WithExternalIdentities(r domain.ExternalIdentityLister) ListerOption {
	return func(l *playerLister) { l.external = r }
}

// NewPlayerLister cria o PlayerLister a partir do repositório de contas.
func NewPlayerLister(repo domain.AccountRepository, opts ...ListerOption) domain.PlayerLister {
	l := &playerLister{repo: repo}
	for _, o := range opts {
		o(l)
	}
	return l
}

// ListPlayers aplica as regras de paginação (clamp) e devolve a página + total.
func (l *playerLister) ListPlayers(ctx context.Context, limit, offset int) ([]domain.Account, int, error) {
	limit, offset = clampPaging(limit, offset)

	players, err := l.repo.ListAccounts(ctx, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("identity service: list players: %w", err)
	}
	total, err := l.repo.CountAccounts(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("identity service: list players: count: %w", err)
	}
	return players, total, nil
}

// GetPlayer devolve a conta pelo id (feature 004-admin-player-by-id).
// ErrAccountNotFound passa intacto — o transport o mapeia para 404.
func (l *playerLister) GetPlayer(ctx context.Context, id string) (*domain.Account, error) {
	acc, err := l.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrAccountNotFound) {
			return nil, domain.ErrAccountNotFound
		}
		return nil, fmt.Errorf("identity service: get player: %w", err)
	}
	return acc, nil
}

// ExternalIdentities devolve os vínculos de loja da conta (spec 006). Confere
// a conta antes: conta inexistente é 404, conta sem vínculo é lista vazia.
func (l *playerLister) ExternalIdentities(ctx context.Context, id string) ([]domain.ExternalIdentity, error) {
	if _, err := l.GetPlayer(ctx, id); err != nil {
		return nil, err
	}
	if l.external == nil {
		return []domain.ExternalIdentity{}, nil
	}
	ids, err := l.external.ListExternalByAccount(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("identity service: external identities: %w", err)
	}
	return ids, nil
}

// clampPaging força limit em [1, maxLimit] (default quando <= 0) e offset >= 0.
func clampPaging(limit, offset int) (int, int) {
	switch {
	case limit <= 0:
		limit = defaultLimit
	case limit > maxLimit:
		limit = maxLimit
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

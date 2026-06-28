// Package service contém as regras de negócio do identity-service.
// Depende apenas de interfaces declaradas em domain. Ver ADR-0002.
package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/c130n/game-backend/services/identity-service/internal/domain"
)

var handleRE = regexp.MustCompile(`^[a-zA-Z0-9_]{3,20}$`)

const minPasswordLen = 8

// dummyHash permite ao Authenticate gastar tempo de bcrypt mesmo quando a
// conta não existe, evitando um timing oracle que revelaria handles válidos.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("timing-oracle-guard"), bcrypt.DefaultCost)

type service struct {
	repo   domain.AccountRepository
	tokens domain.TokenIssuer
}

// New cria o AuthService do identity-service.
func New(repo domain.AccountRepository, tokens domain.TokenIssuer) domain.AuthService {
	return &service{repo: repo, tokens: tokens}
}

func (s *service) Register(ctx context.Context, handle, password string) (*domain.Account, error) {
	if !handleRE.MatchString(handle) {
		return nil, domain.ErrInvalidHandle
	}
	if len(password) < minPasswordLen {
		return nil, domain.ErrWeakPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("identity service: register: hash: %w", err)
	}
	acc := &domain.Account{
		ID:           uuid.NewString(),
		Handle:       handle,
		PasswordHash: string(hash),
		CreatedAt:    time.Now().UTC(),
	}
	// Unicidade (case-insensitive) é garantida pelo repositório, que retorna
	// domain.ErrHandleTaken em violação — evita corrida de pré-checagem.
	if err := s.repo.Create(ctx, acc); err != nil {
		return nil, fmt.Errorf("identity service: register: %w", err)
	}
	return acc, nil
}

func (s *service) Authenticate(ctx context.Context, handle, password string) (string, error) {
	acc, err := s.repo.FindByHandle(ctx, handle)
	if errors.Is(err, domain.ErrAccountNotFound) {
		// gasta tempo de bcrypt para não vazar a inexistência do handle
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		return "", domain.ErrInvalidCredentials
	}
	if err != nil {
		return "", fmt.Errorf("identity service: authenticate: %w", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(acc.PasswordHash), []byte(password)) != nil {
		return "", domain.ErrInvalidCredentials
	}
	token, err := s.tokens.Sign(ctx, acc.ID, nil)
	if err != nil {
		return "", fmt.Errorf("identity service: authenticate: sign: %w", err)
	}
	return token, nil
}

func (s *service) LoginAnonymous(ctx context.Context) (string, error) {
	subject := "guest-" + uuid.NewString()
	token, err := s.tokens.Sign(ctx, subject, map[string]any{"anon": true})
	if err != nil {
		return "", fmt.Errorf("identity service: login anonymous: sign: %w", err)
	}
	return token, nil
}

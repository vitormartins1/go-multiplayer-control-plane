// Package domain contém os tipos de negócio puros e as interfaces (Ports)
// do identity-service. Zero dependências externas. Ver ADR-0002.
package domain

import (
	"context"
	"time"
)

// Account é a conta de um jogador registrado.
type Account struct {
	ID           string
	Handle       string
	PasswordHash string // representação irreversível da senha; nunca em claro
	CreatedAt    time.Time
}

// AuthService é a Port de entrada do identity-service (feature 001-auth-core).
// A feature 002-anonymous-login estende esta interface com LoginAnonymous.
type AuthService interface {
	Register(ctx context.Context, handle, password string) (*Account, error)
	Authenticate(ctx context.Context, handle, password string) (token string, err error)
}

// AccountRepository é a Port de saída de persistência (implementada em repository).
type AccountRepository interface {
	Create(ctx context.Context, a *Account) error
	// FindByHandle busca por handle de forma case-insensitive.
	FindByHandle(ctx context.Context, handle string) (*Account, error)
}

// TokenIssuer é a Port de saída de emissão de tokens.
// Sign aceita claims extras (ex: "anon" na 002) para servir múltiplas features
// sem mudar a interface.
type TokenIssuer interface {
	Sign(ctx context.Context, subject string, claims map[string]any) (token string, err error)
	JWKS(ctx context.Context) ([]byte, error)
}

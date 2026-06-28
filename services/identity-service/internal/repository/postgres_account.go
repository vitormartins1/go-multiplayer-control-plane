// Package repository implementa as Ports de saída de domain em PostgreSQL
// (pgx). Ver ADR-0002.
package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/c130n/game-backend/services/identity-service/internal/domain"
)

// garante em tempo de compilação que satisfazemos a Port.
var _ domain.AccountRepository = (*PostgresAccountRepository)(nil)

// PostgresAccountRepository persiste contas em PostgreSQL.
type PostgresAccountRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresAccountRepository cria o repositório a partir de um pool pgx.
func NewPostgresAccountRepository(pool *pgxpool.Pool) *PostgresAccountRepository {
	return &PostgresAccountRepository{pool: pool}
}

func (r *PostgresAccountRepository) Create(ctx context.Context, a *domain.Account) error {
	const q = `INSERT INTO accounts (id, handle, password_hash, created_at) VALUES ($1, $2, $3, $4)`
	_, err := r.pool.Exec(ctx, q, a.ID, a.Handle, a.PasswordHash, a.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return domain.ErrHandleTaken
		}
		return fmt.Errorf("identity repository: create: %w", err)
	}
	return nil
}

func (r *PostgresAccountRepository) FindByHandle(ctx context.Context, handle string) (*domain.Account, error) {
	const q = `SELECT id, handle, password_hash, created_at FROM accounts WHERE lower(handle) = lower($1)`
	var a domain.Account
	err := r.pool.QueryRow(ctx, q, handle).Scan(&a.ID, &a.Handle, &a.PasswordHash, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrAccountNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("identity repository: find by handle: %w", err)
	}
	return &a, nil
}

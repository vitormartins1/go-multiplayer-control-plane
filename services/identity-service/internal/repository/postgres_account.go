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

	"github.com/vitormartins1/go-game-backend/services/identity-service/internal/domain"
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

// ListAccounts devolve contas por created_at desc (desempata por id para
// paginação estável). Seleciona só id/handle/created_at — o password_hash NÃO é
// lido aqui (defesa na fonte: nunca vaza credencial no caminho admin).
func (r *PostgresAccountRepository) ListAccounts(ctx context.Context, limit, offset int) ([]domain.Account, error) {
	const q = `SELECT id, handle, created_at FROM accounts ORDER BY created_at DESC, id LIMIT $1 OFFSET $2`
	rows, err := r.pool.Query(ctx, q, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("identity repository: list accounts: %w", err)
	}
	defer rows.Close()

	var out []domain.Account
	for rows.Next() {
		var a domain.Account
		if err := rows.Scan(&a.ID, &a.Handle, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("identity repository: list accounts: scan: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("identity repository: list accounts: rows: %w", err)
	}
	return out, nil
}

// CountAccounts devolve o total de contas.
func (r *PostgresAccountRepository) CountAccounts(ctx context.Context) (int, error) {
	var n int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM accounts`).Scan(&n); err != nil {
		return 0, fmt.Errorf("identity repository: count accounts: %w", err)
	}
	return n, nil
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

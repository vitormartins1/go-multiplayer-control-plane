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

	"github.com/vitormartins1/go-multiplayer-control-plane/services/identity-service/internal/domain"
)

// garante em tempo de compilação que satisfazemos as Ports.
var (
	_ domain.AccountRepository          = (*PostgresAccountRepository)(nil)
	_ domain.ExternalIdentityRepository = (*PostgresAccountRepository)(nil)
)

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

// FindByID busca a conta pelo id com a mesma projeção do ListAccounts — o
// password_hash NÃO é lido (caminho admin, feature 004-admin-player-by-id).
func (r *PostgresAccountRepository) FindByID(ctx context.Context, id string) (*domain.Account, error) {
	const q = `SELECT id, handle, created_at FROM accounts WHERE id = $1`
	var a domain.Account
	err := r.pool.QueryRow(ctx, q, id).Scan(&a.ID, &a.Handle, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrAccountNotFound
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "22P02" { // invalid_text_representation
			// id que nem é UUID válido: conta que não existe, não erro interno.
			return nil, domain.ErrAccountNotFound
		}
		return nil, fmt.Errorf("identity repository: find by id: %w", err)
	}
	return &a, nil
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

// FindByExternal devolve a conta vinculada à identidade externa (spec 005).
// Mesma projeção do FindByID (sem password_hash).
func (r *PostgresAccountRepository) FindByExternal(ctx context.Context, provider, externalID string) (*domain.Account, error) {
	const q = `SELECT a.id, a.handle, a.created_at
		FROM external_identities e JOIN accounts a ON a.id = e.account_id
		WHERE e.provider = $1 AND e.external_id = $2`
	var a domain.Account
	err := r.pool.QueryRow(ctx, q, provider, externalID).Scan(&a.ID, &a.Handle, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrAccountNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("identity repository: find by external: %w", err)
	}
	return &a, nil
}

// CreateWithExternal cria a conta e o vínculo numa transação (spec 005): ou os
// dois existem, ou nenhum. Violação de unicidade distingue o vínculo (corrida
// com outro login do mesmo jogador) do handle.
func (r *PostgresAccountRepository) CreateWithExternal(ctx context.Context, a *domain.Account, provider, externalID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("identity repository: create external: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op depois do Commit

	if _, err := tx.Exec(ctx, `INSERT INTO accounts (id, handle, password_hash, created_at) VALUES ($1, $2, $3, $4)`,
		a.ID, a.Handle, a.PasswordHash, a.CreatedAt); err != nil {
		if isUniqueViolation(err) {
			return domain.ErrHandleTaken
		}
		return fmt.Errorf("identity repository: create external: account: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO external_identities (provider, external_id, account_id) VALUES ($1, $2, $3)`,
		provider, externalID, a.ID); err != nil {
		if isUniqueViolation(err) {
			return domain.ErrExternalLinked
		}
		return fmt.Errorf("identity repository: create external: link: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("identity repository: create external: commit: %w", err)
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

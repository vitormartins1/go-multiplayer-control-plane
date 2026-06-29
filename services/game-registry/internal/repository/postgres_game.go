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

	"github.com/c130n/game-backend/services/game-registry/internal/domain"
)

var _ domain.GameRepository = (*PostgresGameRepository)(nil)

// PostgresGameRepository persiste jogos em PostgreSQL.
type PostgresGameRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresGameRepository cria o repositório a partir de um pool pgx.
func NewPostgresGameRepository(pool *pgxpool.Pool) *PostgresGameRepository {
	return &PostgresGameRepository{pool: pool}
}

func (r *PostgresGameRepository) Create(ctx context.Context, g *domain.Game) error {
	const q = `INSERT INTO games (id, slug, name, created_at) VALUES ($1, $2, $3, $4)`
	_, err := r.pool.Exec(ctx, q, g.ID, g.Slug, g.Name, g.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return domain.ErrSlugTaken
		}
		return fmt.Errorf("game-registry repository: create: %w", err)
	}
	return nil
}

func (r *PostgresGameRepository) List(ctx context.Context) ([]domain.Game, error) {
	const q = `SELECT id, slug, name, created_at FROM games ORDER BY name ASC`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("game-registry repository: list: %w", err)
	}
	defer rows.Close()

	var out []domain.Game
	for rows.Next() {
		var g domain.Game
		if err := rows.Scan(&g.ID, &g.Slug, &g.Name, &g.CreatedAt); err != nil {
			return nil, fmt.Errorf("game-registry repository: list: scan: %w", err)
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("game-registry repository: list: rows: %w", err)
	}
	return out, nil
}

func (r *PostgresGameRepository) FindByID(ctx context.Context, id string) (*domain.Game, error) {
	const q = `SELECT id, slug, name, created_at FROM games WHERE id = $1`
	var g domain.Game
	err := r.pool.QueryRow(ctx, q, id).Scan(&g.ID, &g.Slug, &g.Name, &g.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrGameNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("game-registry repository: find by id: %w", err)
	}
	return &g, nil
}

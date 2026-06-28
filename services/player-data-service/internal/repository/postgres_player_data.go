// Package repository implementa as Ports de saída de domain em PostgreSQL
// (pgx). Banco próprio do serviço (ADR-0015). Ver ADR-0002.
package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/c130n/game-backend/services/player-data-service/internal/domain"
)

// garante em tempo de compilação que satisfazemos a Port.
var _ domain.PlayerDataRepository = (*PostgresPlayerDataRepository)(nil)

// PostgresPlayerDataRepository persiste dados de jogador em PostgreSQL (JSONB).
type PostgresPlayerDataRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresPlayerDataRepository cria o repositório a partir de um pool pgx.
func NewPostgresPlayerDataRepository(pool *pgxpool.Pool) *PostgresPlayerDataRepository {
	return &PostgresPlayerDataRepository{pool: pool}
}

func (r *PostgresPlayerDataRepository) Get(ctx context.Context, playerID, namespace, key string) ([]byte, error) {
	const q = `SELECT value FROM player_data WHERE player_id = $1 AND namespace = $2 AND key = $3`
	var value []byte
	err := r.pool.QueryRow(ctx, q, playerID, namespace, key).Scan(&value)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("player-data repository: get: %w", err)
	}
	return value, nil
}

func (r *PostgresPlayerDataRepository) GetNamespace(ctx context.Context, playerID, namespace string) (map[string][]byte, error) {
	const q = `SELECT key, value FROM player_data WHERE player_id = $1 AND namespace = $2`
	rows, err := r.pool.Query(ctx, q, playerID, namespace)
	if err != nil {
		return nil, fmt.Errorf("player-data repository: get namespace: %w", err)
	}
	defer rows.Close()

	out := make(map[string][]byte)
	for rows.Next() {
		var k string
		var v []byte
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("player-data repository: scan: %w", err)
		}
		out[k] = v
	}
	return out, rows.Err()
}

// Put faz upsert do valor. O cast $4::jsonb é necessário: passamos o JSON como
// texto e o Postgres o converte para JSONB (lição GO-LEARNINGS #09).
func (r *PostgresPlayerDataRepository) Put(ctx context.Context, playerID, namespace, key string, value []byte) error {
	const q = `INSERT INTO player_data (player_id, namespace, key, value, updated_at)
	           VALUES ($1, $2, $3, $4::jsonb, now())
	           ON CONFLICT (player_id, namespace, key) DO UPDATE
	             SET value = EXCLUDED.value, updated_at = EXCLUDED.updated_at`
	if _, err := r.pool.Exec(ctx, q, playerID, namespace, key, string(value)); err != nil {
		return fmt.Errorf("player-data repository: put: %w", err)
	}
	return nil
}

func (r *PostgresPlayerDataRepository) Delete(ctx context.Context, playerID, namespace, key string) error {
	const q = `DELETE FROM player_data WHERE player_id = $1 AND namespace = $2 AND key = $3`
	if _, err := r.pool.Exec(ctx, q, playerID, namespace, key); err != nil {
		return fmt.Errorf("player-data repository: delete: %w", err)
	}
	return nil
}

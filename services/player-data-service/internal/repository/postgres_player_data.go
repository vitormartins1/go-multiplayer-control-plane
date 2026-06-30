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

func (r *PostgresPlayerDataRepository) Get(ctx context.Context, gameID, playerID, namespace, key string) ([]byte, error) {
	const q = `SELECT value FROM player_data WHERE game_id = $1 AND player_id = $2 AND namespace = $3 AND key = $4`
	var value []byte
	err := r.pool.QueryRow(ctx, q, gameID, playerID, namespace, key).Scan(&value)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("player-data repository: get: %w", err)
	}
	return value, nil
}

func (r *PostgresPlayerDataRepository) GetNamespace(ctx context.Context, gameID, playerID, namespace string) (map[string][]byte, error) {
	const q = `SELECT key, value FROM player_data WHERE game_id = $1 AND player_id = $2 AND namespace = $3`
	rows, err := r.pool.Query(ctx, q, gameID, playerID, namespace)
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

// Put faz upsert do valor. O cast $5::jsonb é necessário: passamos o JSON como
// texto e o Postgres o converte para JSONB (lição GO-LEARNINGS #09).
func (r *PostgresPlayerDataRepository) Put(ctx context.Context, gameID, playerID, namespace, key string, value []byte) error {
	const q = `INSERT INTO player_data (game_id, player_id, namespace, key, value, updated_at)
	           VALUES ($1, $2, $3, $4, $5::jsonb, now())
	           ON CONFLICT (game_id, player_id, namespace, key) DO UPDATE
	             SET value = EXCLUDED.value, updated_at = EXCLUDED.updated_at`
	if _, err := r.pool.Exec(ctx, q, gameID, playerID, namespace, key, string(value)); err != nil {
		return fmt.Errorf("player-data repository: put: %w", err)
	}
	return nil
}

func (r *PostgresPlayerDataRepository) Delete(ctx context.Context, gameID, playerID, namespace, key string) error {
	const q = `DELETE FROM player_data WHERE game_id = $1 AND player_id = $2 AND namespace = $3 AND key = $4`
	if _, err := r.pool.Exec(ctx, q, gameID, playerID, namespace, key); err != nil {
		return fmt.Errorf("player-data repository: delete: %w", err)
	}
	return nil
}

// ListForPlayer devolve todo o dado de um jogador num jogo (admin read).
func (r *PostgresPlayerDataRepository) ListForPlayer(ctx context.Context, gameID, playerID string) ([]domain.Entry, error) {
	const q = `SELECT namespace, key, value FROM player_data
	           WHERE game_id = $1 AND player_id = $2
	           ORDER BY namespace, key`
	rows, err := r.pool.Query(ctx, q, gameID, playerID)
	if err != nil {
		return nil, fmt.Errorf("player-data repository: list for player: %w", err)
	}
	defer rows.Close()

	var out []domain.Entry
	for rows.Next() {
		var e domain.Entry
		if err := rows.Scan(&e.Namespace, &e.Key, &e.Value); err != nil {
			return nil, fmt.Errorf("player-data repository: scan: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

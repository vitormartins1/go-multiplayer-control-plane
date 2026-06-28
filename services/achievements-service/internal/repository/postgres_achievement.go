// Package repository implementa as Ports de saída de domain em PostgreSQL
// (pgx). Banco próprio do serviço (ADR-0015). Ver ADR-0002.
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/c130n/game-backend/services/achievements-service/internal/domain"
)

// garante em tempo de compilação que satisfazemos a Port.
var _ domain.AchievementRepository = (*PostgresAchievementRepository)(nil)

// PostgresAchievementRepository persiste catálogo e desbloqueios em PostgreSQL.
type PostgresAchievementRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresAchievementRepository cria o repositório a partir de um pool pgx.
func NewPostgresAchievementRepository(pool *pgxpool.Pool) *PostgresAchievementRepository {
	return &PostgresAchievementRepository{pool: pool}
}

func (r *PostgresAchievementRepository) ListCatalog(ctx context.Context) ([]domain.Achievement, error) {
	const q = `SELECT id, name, description, event_type FROM achievements ORDER BY id`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("achievements repository: list catalog: %w", err)
	}
	defer rows.Close()
	return scanAchievements(rows)
}

func (r *PostgresAchievementRepository) FindByEventType(ctx context.Context, eventType string) ([]domain.Achievement, error) {
	const q = `SELECT id, name, description, event_type FROM achievements WHERE event_type = $1 ORDER BY id`
	rows, err := r.pool.Query(ctx, q, eventType)
	if err != nil {
		return nil, fmt.Errorf("achievements repository: find by event type: %w", err)
	}
	defer rows.Close()
	return scanAchievements(rows)
}

func (r *PostgresAchievementRepository) ListPlayerUnlocks(ctx context.Context, playerID string) ([]domain.Unlock, error) {
	const q = `SELECT player_id, achievement_id, unlocked_at FROM player_achievements WHERE player_id = $1`
	rows, err := r.pool.Query(ctx, q, playerID)
	if err != nil {
		return nil, fmt.Errorf("achievements repository: list unlocks: %w", err)
	}
	defer rows.Close()

	var out []domain.Unlock
	for rows.Next() {
		var u domain.Unlock
		if err := rows.Scan(&u.PlayerID, &u.AchievementID, &u.UnlockedAt); err != nil {
			return nil, fmt.Errorf("achievements repository: scan unlock: %w", err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// Unlock insere o desbloqueio de forma idempotente: ON CONFLICT DO NOTHING não
// duplica nem altera o unlocked_at original. created indica se foi um
// desbloqueio novo (a linha foi inserida).
func (r *PostgresAchievementRepository) Unlock(ctx context.Context, playerID, achievementID string, at time.Time) (bool, error) {
	const q = `INSERT INTO player_achievements (player_id, achievement_id, unlocked_at)
	           VALUES ($1, $2, $3)
	           ON CONFLICT (player_id, achievement_id) DO NOTHING
	           RETURNING player_id`
	var dummy string
	err := r.pool.QueryRow(ctx, q, playerID, achievementID, at).Scan(&dummy)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil // já existia → nada inserido
	}
	if err != nil {
		return false, fmt.Errorf("achievements repository: unlock: %w", err)
	}
	return true, nil
}

func scanAchievements(rows pgx.Rows) ([]domain.Achievement, error) {
	var out []domain.Achievement
	for rows.Next() {
		var a domain.Achievement
		if err := rows.Scan(&a.ID, &a.Name, &a.Description, &a.EventType); err != nil {
			return nil, fmt.Errorf("achievements repository: scan: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

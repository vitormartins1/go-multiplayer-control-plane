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

// PostgresAchievementRepository persiste catálogo e progresso em PostgreSQL.
type PostgresAchievementRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresAchievementRepository cria o repositório a partir de um pool pgx.
func NewPostgresAchievementRepository(pool *pgxpool.Pool) *PostgresAchievementRepository {
	return &PostgresAchievementRepository{pool: pool}
}

func (r *PostgresAchievementRepository) ListCatalog(ctx context.Context) ([]domain.Achievement, error) {
	const q = `SELECT id, name, description, event_type, target FROM achievements ORDER BY id`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("achievements repository: list catalog: %w", err)
	}
	defer rows.Close()
	return scanAchievements(rows)
}

func (r *PostgresAchievementRepository) FindByEventType(ctx context.Context, eventType string) ([]domain.Achievement, error) {
	const q = `SELECT id, name, description, event_type, target FROM achievements WHERE event_type = $1 ORDER BY id`
	rows, err := r.pool.Query(ctx, q, eventType)
	if err != nil {
		return nil, fmt.Errorf("achievements repository: find by event type: %w", err)
	}
	defer rows.Close()
	return scanAchievements(rows)
}

func (r *PostgresAchievementRepository) ListPlayerProgress(ctx context.Context, playerID string) ([]domain.PlayerProgress, error) {
	const q = `SELECT achievement_id, progress, unlocked_at FROM player_achievements WHERE player_id = $1`
	rows, err := r.pool.Query(ctx, q, playerID)
	if err != nil {
		return nil, fmt.Errorf("achievements repository: list progress: %w", err)
	}
	defer rows.Close()

	var out []domain.PlayerProgress
	for rows.Next() {
		var p domain.PlayerProgress
		var unlockedAt *time.Time // nullable: NULL = em progresso
		if err := rows.Scan(&p.AchievementID, &p.Progress, &unlockedAt); err != nil {
			return nil, fmt.Errorf("achievements repository: scan progress: %w", err)
		}
		if unlockedAt != nil {
			p.UnlockedAt = *unlockedAt
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// RecordProgress incrementa o progresso do jogador numa conquista de forma
// atômica e desbloqueia ao atingir target — tudo em uma instrução. O LEAST
// trava em target; o WHERE evita mexer após o desbloqueio; o COALESCE nunca
// regride o unlocked_at. justUnlocked é true só quando o desbloqueio acontece
// nesta chamada (a linha existia em progresso e atingiu a meta agora, ou um
// target=1 inserido já desbloqueado).
func (r *PostgresAchievementRepository) RecordProgress(ctx context.Context, playerID, achievementID string, target int, at time.Time) (bool, error) {
	const q = `INSERT INTO player_achievements (player_id, achievement_id, progress, unlocked_at)
	           VALUES ($1, $2, 1, CASE WHEN 1 >= $3 THEN $4::timestamptz ELSE NULL END)
	           ON CONFLICT (player_id, achievement_id) DO UPDATE
	             SET progress    = LEAST(player_achievements.progress + 1, $3),
	                 unlocked_at = COALESCE(
	                                 player_achievements.unlocked_at,
	                                 CASE WHEN player_achievements.progress + 1 >= $3 THEN $4::timestamptz END)
	             WHERE player_achievements.unlocked_at IS NULL
	           RETURNING unlocked_at`
	var unlockedAt *time.Time
	err := r.pool.QueryRow(ctx, q, playerID, achievementID, target, at).Scan(&unlockedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil // WHERE barrou: já estava desbloqueada
	}
	if err != nil {
		return false, fmt.Errorf("achievements repository: record progress: %w", err)
	}
	return unlockedAt != nil, nil // desbloqueou nesta chamada?
}

func scanAchievements(rows pgx.Rows) ([]domain.Achievement, error) {
	var out []domain.Achievement
	for rows.Next() {
		var a domain.Achievement
		if err := rows.Scan(&a.ID, &a.Name, &a.Description, &a.EventType, &a.Target); err != nil {
			return nil, fmt.Errorf("achievements repository: scan: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

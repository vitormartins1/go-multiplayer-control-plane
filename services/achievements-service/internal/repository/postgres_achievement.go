// Package repository implementa as Ports de saída de domain em PostgreSQL
// (pgx). Banco próprio do serviço (ADR-0015). Ver ADR-0002.
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vitormartins1/go-game-backend/services/achievements-service/internal/domain"
)

// garante em tempo de compilação que satisfazemos a Port.
var _ domain.AchievementRepository = (*PostgresAchievementRepository)(nil)

// PostgresAchievementRepository persiste catálogo e progresso em PostgreSQL.
// Tudo é escopado por game_id (multi-tenant, ADR-0017).
type PostgresAchievementRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresAchievementRepository cria o repositório a partir de um pool pgx.
func NewPostgresAchievementRepository(pool *pgxpool.Pool) *PostgresAchievementRepository {
	return &PostgresAchievementRepository{pool: pool}
}

func (r *PostgresAchievementRepository) ListCatalog(ctx context.Context, gameID string) ([]domain.Achievement, error) {
	const q = `SELECT game_id, id, name, description, event_type, target
	           FROM achievements WHERE game_id = $1 ORDER BY id`
	rows, err := r.pool.Query(ctx, q, gameID)
	if err != nil {
		return nil, fmt.Errorf("achievements repository: list catalog: %w", err)
	}
	defer rows.Close()
	return scanAchievements(rows)
}

func (r *PostgresAchievementRepository) FindByEventType(ctx context.Context, gameID, eventType string) ([]domain.Achievement, error) {
	const q = `SELECT game_id, id, name, description, event_type, target
	           FROM achievements WHERE game_id = $1 AND event_type = $2 ORDER BY id`
	rows, err := r.pool.Query(ctx, q, gameID, eventType)
	if err != nil {
		return nil, fmt.Errorf("achievements repository: find by event type: %w", err)
	}
	defer rows.Close()
	return scanAchievements(rows)
}

func (r *PostgresAchievementRepository) ListPlayerProgress(ctx context.Context, gameID, playerID string) ([]domain.PlayerProgress, error) {
	const q = `SELECT achievement_id, progress, unlocked_at
	           FROM player_achievements WHERE game_id = $1 AND player_id = $2`
	rows, err := r.pool.Query(ctx, q, gameID, playerID)
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
// nesta chamada. Escopado por game_id.
func (r *PostgresAchievementRepository) RecordProgress(ctx context.Context, gameID, playerID, achievementID string, target int, at time.Time) (bool, error) {
	const q = `INSERT INTO player_achievements (game_id, player_id, achievement_id, progress, unlocked_at)
	           VALUES ($1, $2, $3, 1, CASE WHEN 1 >= $4 THEN $5::timestamptz ELSE NULL END)
	           ON CONFLICT (game_id, player_id, achievement_id) DO UPDATE
	             SET progress    = LEAST(player_achievements.progress + 1, $4),
	                 unlocked_at = COALESCE(
	                                 player_achievements.unlocked_at,
	                                 CASE WHEN player_achievements.progress + 1 >= $4 THEN $5::timestamptz END)
	             WHERE player_achievements.unlocked_at IS NULL
	           RETURNING unlocked_at`
	var unlockedAt *time.Time
	err := r.pool.QueryRow(ctx, q, gameID, playerID, achievementID, target, at).Scan(&unlockedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil // WHERE barrou: já estava desbloqueada
	}
	if err != nil {
		return false, fmt.Errorf("achievements repository: record progress: %w", err)
	}
	return unlockedAt != nil, nil // desbloqueou nesta chamada?
}

// CreateAchievement adiciona uma conquista ao catálogo do jogo (admin).
// Violação de unicidade (game_id, id) vira ErrAchievementExists.
func (r *PostgresAchievementRepository) CreateAchievement(ctx context.Context, a *domain.Achievement) error {
	const q = `INSERT INTO achievements (game_id, id, name, description, event_type, target)
	           VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := r.pool.Exec(ctx, q, a.GameID, a.ID, a.Name, a.Description, a.EventType, a.Target)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return domain.ErrAchievementExists
		}
		return fmt.Errorf("achievements repository: create achievement: %w", err)
	}
	return nil
}

// UpdateAchievement atualiza os campos mutáveis da conquista (game_id, id).
// Zero linhas afetadas -> ErrAchievementNotFound.
func (r *PostgresAchievementRepository) UpdateAchievement(ctx context.Context, a *domain.Achievement) error {
	const q = `UPDATE achievements
	           SET name = $3, description = $4, event_type = $5, target = $6
	           WHERE game_id = $1 AND id = $2`
	tag, err := r.pool.Exec(ctx, q, a.GameID, a.ID, a.Name, a.Description, a.EventType, a.Target)
	if err != nil {
		return fmt.Errorf("achievements repository: update achievement: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrAchievementNotFound
	}
	return nil
}

// DeleteAchievement remove a conquista (game_id, id) e o progresso dos jogadores
// nela numa transação (a FK player_achievements -> achievements é RESTRICT).
// Zero linhas no catálogo -> ErrAchievementNotFound.
func (r *PostgresAchievementRepository) DeleteAchievement(ctx context.Context, gameID, id string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("achievements repository: delete achievement: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op após Commit

	if _, err := tx.Exec(ctx,
		`DELETE FROM player_achievements WHERE game_id = $1 AND achievement_id = $2`, gameID, id); err != nil {
		return fmt.Errorf("achievements repository: delete progress: %w", err)
	}
	tag, err := tx.Exec(ctx, `DELETE FROM achievements WHERE game_id = $1 AND id = $2`, gameID, id)
	if err != nil {
		return fmt.Errorf("achievements repository: delete achievement: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrAchievementNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("achievements repository: delete achievement: commit: %w", err)
	}
	return nil
}

func scanAchievements(rows pgx.Rows) ([]domain.Achievement, error) {
	var out []domain.Achievement
	for rows.Next() {
		var a domain.Achievement
		if err := rows.Scan(&a.GameID, &a.ID, &a.Name, &a.Description, &a.EventType, &a.Target); err != nil {
			return nil, fmt.Errorf("achievements repository: scan: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

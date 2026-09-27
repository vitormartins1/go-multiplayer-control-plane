package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/achievements-service/internal/domain"
)

var _ domain.SteamSyncRepository = (*PostgresSteamSyncRepository)(nil)

// PostgresSteamSyncRepository é a fila do espelhamento na Steam (spec 019)
// sobre player_achievements: pendente = desbloqueada + conquista com
// steam_api_name + estado vazio.
type PostgresSteamSyncRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresSteamSyncRepository cria o repositório.
func NewPostgresSteamSyncRepository(pool *pgxpool.Pool) *PostgresSteamSyncRepository {
	return &PostgresSteamSyncRepository{pool: pool}
}

// ClaimSteamPending seleciona com FOR UPDATE SKIP LOCKED e empurra o
// next_attempt para leaseUntil no mesmo comando: duas réplicas do consumer não
// pegam o mesmo desbloqueio, e um worker que morrer no meio libera o item
// quando o aluguel vencer.
func (r *PostgresSteamSyncRepository) ClaimSteamPending(ctx context.Context, gameID string, limit int, now, leaseUntil time.Time) ([]domain.SteamSyncItem, error) {
	const q = `
		WITH picked AS (
			SELECT pa.game_id, pa.player_id, pa.achievement_id
			  FROM player_achievements pa
			  JOIN achievements a ON a.game_id = pa.game_id AND a.id = pa.achievement_id
			 WHERE pa.game_id = $1
			   AND pa.unlocked_at IS NOT NULL
			   AND pa.steam_sync_state = ''
			   AND a.steam_api_name <> ''
			   AND (pa.steam_next_attempt_at IS NULL OR pa.steam_next_attempt_at <= $2)
			 ORDER BY pa.unlocked_at
			 LIMIT $3
			   FOR UPDATE OF pa SKIP LOCKED
		)
		UPDATE player_achievements pa
		   SET steam_next_attempt_at = $4
		  FROM picked p, achievements a
		 WHERE pa.game_id = p.game_id AND pa.player_id = p.player_id AND pa.achievement_id = p.achievement_id
		   AND a.game_id = pa.game_id AND a.id = pa.achievement_id
		RETURNING pa.game_id, pa.player_id, pa.achievement_id, a.steam_api_name, pa.steam_sync_attempts`
	rows, err := r.pool.Query(ctx, q, gameID, now, limit, leaseUntil)
	if err != nil {
		return nil, fmt.Errorf("steam sync repository: claim: %w", err)
	}
	defer rows.Close()
	var out []domain.SteamSyncItem
	for rows.Next() {
		var it domain.SteamSyncItem
		if err := rows.Scan(&it.GameID, &it.PlayerID, &it.AchievementID, &it.SteamAPIName, &it.Attempts); err != nil {
			return nil, fmt.Errorf("steam sync repository: claim: scan: %w", err)
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("steam sync repository: claim: rows: %w", err)
	}
	return out, nil
}

// MarkSteamDone grava o estado final. synced guarda o instante; skipped/failed
// guardam o motivo em steam_sync_error.
func (r *PostgresSteamSyncRepository) MarkSteamDone(ctx context.Context, it domain.SteamSyncItem, state, reason string, at time.Time) error {
	const q = `
		UPDATE player_achievements
		   SET steam_sync_state = $4,
		       steam_sync_error = $5,
		       steam_sync_attempts = steam_sync_attempts + 1,
		       steam_synced_at = CASE WHEN $4 = 'synced' THEN $6::timestamptz ELSE NULL END,
		       steam_next_attempt_at = NULL
		 WHERE game_id = $1 AND player_id = $2 AND achievement_id = $3`
	if _, err := r.pool.Exec(ctx, q, it.GameID, it.PlayerID, it.AchievementID, state, reason, at); err != nil {
		return fmt.Errorf("steam sync repository: mark done: %w", err)
	}
	return nil
}

// MarkSteamRetry conta a tentativa e reagenda.
func (r *PostgresSteamSyncRepository) MarkSteamRetry(ctx context.Context, it domain.SteamSyncItem, nextAt time.Time, reason string) error {
	const q = `
		UPDATE player_achievements
		   SET steam_sync_attempts = steam_sync_attempts + 1,
		       steam_sync_error = $4,
		       steam_next_attempt_at = $5
		 WHERE game_id = $1 AND player_id = $2 AND achievement_id = $3`
	if _, err := r.pool.Exec(ctx, q, it.GameID, it.PlayerID, it.AchievementID, reason, nextAt); err != nil {
		return fmt.Errorf("steam sync repository: mark retry: %w", err)
	}
	return nil
}

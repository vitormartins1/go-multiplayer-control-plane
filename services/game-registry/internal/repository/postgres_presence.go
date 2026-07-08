package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/game-registry/internal/domain"
)

var _ domain.PresenceRepository = (*PostgresPresenceRepository)(nil)

// PostgresPresenceRepository persiste a presença por jogo (game_players).
type PostgresPresenceRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresPresenceRepository cria o repositório a partir de um pool pgx.
func NewPostgresPresenceRepository(pool *pgxpool.Pool) *PostgresPresenceRepository {
	return &PostgresPresenceRepository{pool: pool}
}

// Upsert grava a presença. ON CONFLICT mantém first_seen (idempotente) e só
// atualiza o handle. O INSERT…SELECT só produz linha se o jogo estiver vivo
// (ADR-0025): jogo inexistente ou deletado -> RowsAffected 0 -> ErrGameNotFound,
// sem SELECT prévio (sem corrida com o delete). A FK segue como rede de
// segurança (23503 -> ErrGameNotFound).
func (r *PostgresPresenceRepository) Upsert(ctx context.Context, gameID, playerID, handle string) error {
	const q = `INSERT INTO game_players (game_id, player_id, handle, first_seen)
	           SELECT $1, $2, $3, $4
	           WHERE EXISTS (SELECT 1 FROM games WHERE id = $1 AND deleted_at IS NULL)
	           ON CONFLICT (game_id, player_id) DO UPDATE SET handle = EXCLUDED.handle`
	tag, err := r.pool.Exec(ctx, q, gameID, playerID, handle, time.Now().UTC())
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign_key_violation
			return domain.ErrGameNotFound
		}
		return fmt.Errorf("game-registry repository: presence upsert: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrGameNotFound
	}
	return nil
}

// GamesByPlayer devolve os jogos em que o player está presente (consulta
// reversa: JOIN game_players × games), por nome. Mesmas colunas/COALESCE do
// repo de games (capa, 003).
func (r *PostgresPresenceRepository) GamesByPlayer(ctx context.Context, playerID string) ([]domain.Game, error) {
	const q = `SELECT g.id, g.slug, g.name, g.created_at,
	                  COALESCE(g.image, ''), g.image_source,
	                  COALESCE(g.generated_image_seed, g.slug), g.generated_image_style
	           FROM games g
	           JOIN game_players gp ON gp.game_id = g.id
	           WHERE gp.player_id = $1 AND g.deleted_at IS NULL
	           ORDER BY g.name ASC`
	rows, err := r.pool.Query(ctx, q, playerID)
	if err != nil {
		return nil, fmt.Errorf("game-registry repository: games by player: %w", err)
	}
	defer rows.Close()

	var out []domain.Game
	for rows.Next() {
		var g domain.Game
		if err := rows.Scan(&g.ID, &g.Slug, &g.Name, &g.CreatedAt,
			&g.Image, &g.ImageSource, &g.GeneratedImageSeed, &g.GeneratedImageStyle); err != nil {
			return nil, fmt.Errorf("game-registry repository: games by player: scan: %w", err)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (r *PostgresPresenceRepository) ListByGame(ctx context.Context, gameID string) ([]domain.Presence, error) {
	// JOIN com games vivo (ADR-0025): jogo deletado responde lista vazia, a
	// mesma forma de jogo sem players.
	const q = `SELECT gp.player_id, gp.handle, gp.first_seen
	           FROM game_players gp
	           JOIN games g ON g.id = gp.game_id AND g.deleted_at IS NULL
	           WHERE gp.game_id = $1 ORDER BY gp.first_seen ASC`
	rows, err := r.pool.Query(ctx, q, gameID)
	if err != nil {
		return nil, fmt.Errorf("game-registry repository: presence list: %w", err)
	}
	defer rows.Close()

	var out []domain.Presence
	for rows.Next() {
		var p domain.Presence
		if err := rows.Scan(&p.PlayerID, &p.Handle, &p.FirstSeen); err != nil {
			return nil, fmt.Errorf("game-registry repository: presence list: scan: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("game-registry repository: presence list: rows: %w", err)
	}
	return out, nil
}

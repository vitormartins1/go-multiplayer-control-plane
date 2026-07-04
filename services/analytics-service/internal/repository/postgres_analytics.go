// Package repository implementa a MatchRepository em PostgreSQL (pgx). Banco
// próprio do serviço (ADR-0015). Ver ADR-0002/0021.
package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vitormartins1/go-game-backend/services/analytics-service/internal/domain"
)

var _ domain.MatchRepository = (*PostgresRepository)(nil)

// querier abstrai *pgxpool.Pool e pgx.Tx para reusar SQL dentro/fora de tx.
type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// PostgresRepository persiste os rollups de analytics.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// New cria o repositório a partir de um pool pgx.
func New(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// ApplyMatch agrega uma partida numa única transação, idempotente por matchID
// (ADR-0021). A marca em processed_matches + os UPSERTs de rollup/daily/seen
// vivem na mesma tx: reprocessar (at-least-once do SQS) é no-op.
func (r *PostgresRepository) ApplyMatch(ctx context.Context, m domain.MatchResult) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("analytics repository: apply match: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op após Commit

	tag, err := tx.Exec(ctx,
		`INSERT INTO processed_matches (match_id, game_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		m.MatchID, m.GameID)
	if err != nil {
		return false, fmt.Errorf("analytics repository: apply match: mark: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return true, tx.Commit(ctx) // já processada: no-op idempotente
	}

	var participations, wins, scoreSum int64
	for _, p := range m.Results {
		participations++
		if p.Won {
			wins++
		}
		scoreSum += p.Score
		if _, err := tx.Exec(ctx,
			`INSERT INTO game_player_seen (game_id, player_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			m.GameID, p.PlayerID); err != nil {
			return false, fmt.Errorf("analytics repository: apply match: player seen: %w", err)
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO game_rollup (game_id, matches_total, participations, wins, score_sum)
		VALUES ($1, 1, $2, $3, $4)
		ON CONFLICT (game_id) DO UPDATE SET
			matches_total  = game_rollup.matches_total + 1,
			participations = game_rollup.participations + $2,
			wins           = game_rollup.wins + $3,
			score_sum      = game_rollup.score_sum + $4`,
		m.GameID, participations, wins, scoreSum); err != nil {
		return false, fmt.Errorf("analytics repository: apply match: rollup: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO game_daily (game_id, day, matches)
		VALUES ($1, (now() AT TIME ZONE 'utc')::date, 1)
		ON CONFLICT (game_id, day) DO UPDATE SET matches = game_daily.matches + 1`,
		m.GameID); err != nil {
		return false, fmt.Errorf("analytics repository: apply match: daily: %w", err)
	}

	return false, tx.Commit(ctx)
}

// Overview devolve os totais cross-game + os jogos ordenados por matches desc.
func (r *PostgresRepository) Overview(ctx context.Context) (domain.Overview, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT r.game_id, r.matches_total, r.participations, r.wins, r.score_sum,
		       (SELECT count(*) FROM game_player_seen s WHERE s.game_id = r.game_id)
		FROM game_rollup r
		ORDER BY r.matches_total DESC, r.game_id`)
	if err != nil {
		return domain.Overview{}, fmt.Errorf("analytics repository: overview: %w", err)
	}
	defer rows.Close()

	var ov domain.Overview
	for rows.Next() {
		var g domain.GameStat
		var participations, scoreSum int64
		if err := rows.Scan(&g.GameID, &g.Matches, &participations, &g.Wins, &scoreSum, &g.UniquePlayers); err != nil {
			return domain.Overview{}, fmt.Errorf("analytics repository: scan overview: %w", err)
		}
		g.Participations = participations
		g.AvgScore = avg(scoreSum, participations)
		ov.Games = append(ov.Games, g)
		ov.Totals.Matches += g.Matches
	}
	if err := rows.Err(); err != nil {
		return domain.Overview{}, fmt.Errorf("analytics repository: overview rows: %w", err)
	}
	ov.Totals.Games = int64(len(ov.Games))

	if err := r.pool.QueryRow(ctx, `SELECT count(DISTINCT player_id) FROM game_player_seen`).
		Scan(&ov.Totals.UniquePlayers); err != nil {
		return domain.Overview{}, fmt.Errorf("analytics repository: overview unique: %w", err)
	}
	return ov, nil
}

// GameAnalytics devolve o resumo do jogo + a série diária dos últimos `days`.
func (r *PostgresRepository) GameAnalytics(ctx context.Context, gameID string, days int) (domain.GameAnalytics, error) {
	var out domain.GameAnalytics
	out.GameID = gameID

	var participations, scoreSum int64
	err := r.pool.QueryRow(ctx, `
		SELECT r.matches_total, r.participations, r.wins, r.score_sum,
		       (SELECT count(*) FROM game_player_seen s WHERE s.game_id = r.game_id)
		FROM game_rollup r WHERE r.game_id = $1`, gameID).
		Scan(&out.Matches, &participations, &out.Wins, &scoreSum, &out.UniquePlayers)
	if err != nil && err != pgx.ErrNoRows {
		return domain.GameAnalytics{}, fmt.Errorf("analytics repository: game analytics: %w", err)
	}
	out.Participations = participations
	out.AvgScore = avg(scoreSum, participations)

	rows, err := r.pool.Query(ctx, `
		SELECT to_char(day, 'YYYY-MM-DD'), matches FROM game_daily
		WHERE game_id = $1 AND day >= (now() AT TIME ZONE 'utc')::date - $2::int
		ORDER BY day`, gameID, days)
	if err != nil {
		return domain.GameAnalytics{}, fmt.Errorf("analytics repository: daily: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var d domain.DailyPoint
		if err := rows.Scan(&d.Day, &d.Matches); err != nil {
			return domain.GameAnalytics{}, fmt.Errorf("analytics repository: scan daily: %w", err)
		}
		out.Daily = append(out.Daily, d)
	}
	return out, rows.Err()
}

func avg(sum, n int64) float64 {
	if n == 0 {
		return 0
	}
	return float64(sum) / float64(n)
}

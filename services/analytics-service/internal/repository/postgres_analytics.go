// Package repository implementa a MatchRepository em PostgreSQL (pgx). Banco
// próprio do serviço (ADR-0015). Ver ADR-0002/0021.
package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/analytics-service/internal/domain"
)

var _ domain.MatchRepository = (*PostgresRepository)(nil)

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

	var participations, wins, losses, draws, scoreSum int64
	for _, p := range m.Results {
		participations++
		switch p.Outcome {
		case domain.OutcomeWin:
			wins++
		case domain.OutcomeLoss:
			losses++
		case domain.OutcomeDraw:
			draws++
		}
		scoreSum += p.Score
		if _, err := tx.Exec(ctx,
			`INSERT INTO game_player_seen (game_id, player_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			m.GameID, p.PlayerID); err != nil {
			return false, fmt.Errorf("analytics repository: apply match: player seen: %w", err)
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO game_rollup (game_id, matches_total, participations, wins, losses, draws, score_sum)
		VALUES ($1, 1, $2, $3, $4, $5, $6)
		ON CONFLICT (game_id) DO UPDATE SET
			matches_total  = game_rollup.matches_total + 1,
			participations = game_rollup.participations + $2,
			wins           = game_rollup.wins + $3,
			losses         = game_rollup.losses + $4,
			draws          = game_rollup.draws + $5,
			score_sum      = game_rollup.score_sum + $6`,
		m.GameID, participations, wins, losses, draws, scoreSum); err != nil {
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

// ApplyParticipation agrega um evento de participação do host_world_dropin
// (spec 004) por (game_id, role, event), idempotente por (session_id, player_id,
// event) na mesma tx (mesma garantia do ApplyMatch). Reprocessar é no-op.
func (r *PostgresRepository) ApplyParticipation(ctx context.Context, evt domain.ParticipationEvent) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("analytics repository: apply participation: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op após Commit

	tag, err := tx.Exec(ctx,
		`INSERT INTO processed_participations (event_id, session_id, player_id, event, game_id)
		 VALUES ($1, $2, $3, $4, $5) ON CONFLICT (event_id) DO NOTHING`,
		effectiveEventID(evt.EventID, evt.SessionID, evt.PlayerID, evt.Event), evt.SessionID, evt.PlayerID, evt.Event, evt.GameID)
	if err != nil {
		return false, fmt.Errorf("analytics repository: apply participation: mark: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return true, tx.Commit(ctx) // já processado: no-op idempotente
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO game_participation_rollup (game_id, role, event, outcome, count)
		VALUES ($1, $2, $3, $4, 1)
		ON CONFLICT (game_id, role, event, outcome) DO UPDATE SET count = game_participation_rollup.count + 1`,
		evt.GameID, evt.Role, evt.Event, evt.Outcome); err != nil {
		return false, fmt.Errorf("analytics repository: apply participation: rollup: %w", err)
	}
	return false, tx.Commit(ctx)
}

// ApplyPlacement agrega a distribuição de colocação de uma partida do
// battle_royale (spec 005) por jogo, idempotente por matchID (processed_matches),
// na mesma tx (mesma garantia do ApplyMatch). Os baldes são exclusivos: winners
// (colocação 1), top_n (2..threshold), rest (>threshold). Reprocessar é no-op.
func (r *PostgresRepository) ApplyPlacement(ctx context.Context, pr domain.PlacementResult) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("analytics repository: apply placement: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op após Commit

	tag, err := tx.Exec(ctx,
		`INSERT INTO processed_matches (match_id, game_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		pr.MatchID, pr.GameID)
	if err != nil {
		return false, fmt.Errorf("analytics repository: apply placement: mark: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return true, tx.Commit(ctx) // já processada: no-op idempotente
	}

	var winners, topN, rest, entries, sumSurvived int64
	for _, e := range pr.Entries {
		entries++
		sumSurvived += e.SurvivedMs
		switch {
		case e.Placement == 1:
			winners++
		case e.Placement <= domain.BattleRoyaleTopNThreshold:
			topN++
		default:
			rest++
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO game_placement_rollup (game_id, winners, top_n, rest, entries, sum_survived_ms)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (game_id) DO UPDATE SET
			winners         = game_placement_rollup.winners + $2,
			top_n           = game_placement_rollup.top_n + $3,
			rest            = game_placement_rollup.rest + $4,
			entries         = game_placement_rollup.entries + $5,
			sum_survived_ms = game_placement_rollup.sum_survived_ms + $6`,
		pr.GameID, winners, topN, rest, entries, sumSurvived); err != nil {
		return false, fmt.Errorf("analytics repository: apply placement: rollup: %w", err)
	}
	return false, tx.Commit(ctx)
}

// ApplyCollective agrega o desfecho da missão de uma partida do coop_mission
// (spec 006) por jogo, idempotente por matchID (processed_matches), na mesma tx
// (mesma garantia do ApplyMatch). Uma missão = um ReportResult: +1 em successes
// ou failures conforme o desfecho da SESSÃO. Reprocessar é no-op.
func (r *PostgresRepository) ApplyCollective(ctx context.Context, cr domain.CollectiveResult) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("analytics repository: apply collective: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op após Commit

	tag, err := tx.Exec(ctx,
		`INSERT INTO processed_matches (match_id, game_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		cr.MatchID, cr.GameID)
	if err != nil {
		return false, fmt.Errorf("analytics repository: apply collective: mark: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return true, tx.Commit(ctx) // já processada: no-op idempotente
	}

	var successDelta, failureDelta int64
	switch cr.Outcome {
	case domain.MissionSuccess:
		successDelta = 1
	case domain.MissionFailure:
		failureDelta = 1
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO game_mission_rollup (game_id, successes, failures)
		VALUES ($1, $2, $3)
		ON CONFLICT (game_id) DO UPDATE SET
			successes = game_mission_rollup.successes + $2,
			failures  = game_mission_rollup.failures + $3`,
		cr.GameID, successDelta, failureDelta); err != nil {
		return false, fmt.Errorf("analytics repository: apply collective: rollup: %w", err)
	}
	return false, tx.Commit(ctx)
}

// Overview devolve os totais cross-game + os jogos ordenados por matches desc.
func (r *PostgresRepository) Overview(ctx context.Context) (domain.Overview, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT r.game_id, r.matches_total, r.participations, r.wins, r.losses, r.draws, r.score_sum,
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
		if err := rows.Scan(&g.GameID, &g.Matches, &participations, &g.Wins, &g.Losses, &g.Draws, &scoreSum, &g.UniquePlayers); err != nil {
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

	// Série "partidas por dia" somada entre todos os jogos (últimos overviewDailyDays).
	drows, err := r.pool.Query(ctx, `
		SELECT to_char(day, 'YYYY-MM-DD'), SUM(matches) FROM game_daily
		WHERE day >= (now() AT TIME ZONE 'utc')::date - $1::int
		GROUP BY day ORDER BY day`, overviewDailyDays)
	if err != nil {
		return domain.Overview{}, fmt.Errorf("analytics repository: overview daily: %w", err)
	}
	defer drows.Close()
	for drows.Next() {
		var d domain.DailyPoint
		if err := drows.Scan(&d.Day, &d.Matches); err != nil {
			return domain.Overview{}, fmt.Errorf("analytics repository: scan overview daily: %w", err)
		}
		ov.Daily = append(ov.Daily, d)
	}
	return ov, drows.Err()
}

// overviewDailyDays é a janela (dias) da série do overview cross-game.
const overviewDailyDays = 30

// GameAnalytics devolve o resumo do jogo + a série diária dos últimos `days`.
func (r *PostgresRepository) GameAnalytics(ctx context.Context, gameID string, days int) (domain.GameAnalytics, error) {
	var out domain.GameAnalytics
	out.GameID = gameID

	var participations, scoreSum int64
	err := r.pool.QueryRow(ctx, `
		SELECT r.matches_total, r.participations, r.wins, r.losses, r.draws, r.score_sum,
		       (SELECT count(*) FROM game_player_seen s WHERE s.game_id = r.game_id)
		FROM game_rollup r WHERE r.game_id = $1`, gameID).
		Scan(&out.Matches, &participations, &out.Wins, &out.Losses, &out.Draws, &scoreSum, &out.UniquePlayers)
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
	if err := rows.Err(); err != nil {
		return domain.GameAnalytics{}, err
	}

	// participações por papel/evento/outcome (host_world_dropin, spec 004/007).
	// Aditivo: queue_arena não tem linhas aqui -> lista vazia.
	prows, err := r.pool.Query(ctx, `
		SELECT role, event, outcome, count FROM game_participation_rollup
		WHERE game_id = $1 ORDER BY role, event, outcome`, gameID)
	if err != nil {
		return domain.GameAnalytics{}, fmt.Errorf("analytics repository: participations: %w", err)
	}
	defer prows.Close()
	for prows.Next() {
		var p domain.ParticipationStat
		if err := prows.Scan(&p.Role, &p.Event, &p.Outcome, &p.Count); err != nil {
			return domain.GameAnalytics{}, fmt.Errorf("analytics repository: scan participation: %w", err)
		}
		out.RoleParticipations = append(out.RoleParticipations, p)
	}
	if err := prows.Err(); err != nil {
		return domain.GameAnalytics{}, err
	}

	// distribuição de colocação (battle_royale, spec 005). Aditivo: sem linha ->
	// Placement nil (jogo não-BR não regride).
	var pd domain.PlacementDistribution
	var sumSurvived int64
	err = r.pool.QueryRow(ctx, `
		SELECT winners, top_n, rest, entries, sum_survived_ms
		FROM game_placement_rollup WHERE game_id = $1`, gameID).
		Scan(&pd.Winners, &pd.TopN, &pd.Rest, &pd.Entries, &sumSurvived)
	switch {
	case err == pgx.ErrNoRows:
		// sem colocação neste jogo: Placement fica nil.
	case err != nil:
		return domain.GameAnalytics{}, fmt.Errorf("analytics repository: placement: %w", err)
	default:
		pd.AvgSurvivedMs = avg(sumSurvived, pd.Entries)
		out.Placement = &pd
	}

	// distribuição de desfecho da missão (coop_mission, spec 006). Aditivo: sem
	// linha -> Mission nil (jogo não-coop não regride).
	var md domain.MissionDistribution
	err = r.pool.QueryRow(ctx, `
		SELECT successes, failures
		FROM game_mission_rollup WHERE game_id = $1`, gameID).
		Scan(&md.Successes, &md.Failures)
	switch {
	case err == pgx.ErrNoRows:
		// sem missões neste jogo: Mission fica nil.
	case err != nil:
		return domain.GameAnalytics{}, fmt.Errorf("analytics repository: mission: %w", err)
	default:
		md.SuccessRate = ratio(md.Successes, md.Successes+md.Failures)
		out.Mission = &md
	}
	return out, nil
}

// ratio devolve num/den (0 se den==0) — taxa de sucesso da missão no read.
func ratio(num, den int64) float64 {
	if den == 0 {
		return 0
	}
	return float64(num) / float64(den)
}

func avg(sum, n int64) float64 {
	if n == 0 {
		return 0
	}
	return float64(sum) / float64(n)
}

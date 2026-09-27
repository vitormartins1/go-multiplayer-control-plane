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
		if err := markPlayerSeen(ctx, tx, m.GameID, m.ModeID, p.PlayerID); err != nil {
			return false, fmt.Errorf("analytics repository: apply match: %w", err)
		}
		// stats brutos por jogador (spec 011), com mode_id (spec 012 — o
		// "" hardcoded era o gap: MatchResult não carregava o modo, mesmo
		// o envelope já publicando desde a spec 010).
		if err := insertPlayerStats(ctx, tx, m.GameID, m.ModeID, m.MatchID, p.PlayerID, p.Stats); err != nil {
			return false, fmt.Errorf("analytics repository: apply match: stats: %w", err)
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO game_rollup (game_id, mode_id, matches_total, participations, wins, losses, draws, score_sum)
		VALUES ($1, $2, 1, $3, $4, $5, $6, $7)
		ON CONFLICT (game_id, mode_id) DO UPDATE SET
			matches_total  = game_rollup.matches_total + 1,
			participations = game_rollup.participations + $3,
			wins           = game_rollup.wins + $4,
			losses         = game_rollup.losses + $5,
			draws          = game_rollup.draws + $6,
			score_sum      = game_rollup.score_sum + $7`,
		m.GameID, m.ModeID, participations, wins, losses, draws, scoreSum); err != nil {
		return false, fmt.Errorf("analytics repository: apply match: rollup: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO game_daily (game_id, mode_id, day, matches)
		VALUES ($1, $2, (now() AT TIME ZONE 'utc')::date, 1)
		ON CONFLICT (game_id, mode_id, day) DO UPDATE SET matches = game_daily.matches + 1`,
		m.GameID, m.ModeID); err != nil {
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
		INSERT INTO game_participation_rollup (game_id, mode_id, role, event, outcome, count)
		VALUES ($1, $2, $3, $4, $5, 1)
		ON CONFLICT (game_id, mode_id, role, event, outcome) DO UPDATE SET count = game_participation_rollup.count + 1`,
		evt.GameID, evt.ModeID, evt.Role, evt.Event, evt.Outcome); err != nil {
		return false, fmt.Errorf("analytics repository: apply participation: rollup: %w", err)
	}

	// série diária por modo (spec 010): host_world_dropin não tem "partida"
	// terminal — a atividade é medida em EVENTOS de participação.
	if _, err := tx.Exec(ctx, `
		INSERT INTO game_participation_daily (game_id, mode_id, day, events)
		VALUES ($1, $2, (now() AT TIME ZONE 'utc')::date, 1)
		ON CONFLICT (game_id, mode_id, day) DO UPDATE SET events = game_participation_daily.events + 1`,
		evt.GameID, evt.ModeID); err != nil {
		return false, fmt.Errorf("analytics repository: apply participation: daily: %w", err)
	}

	// jogador visto (spec 016): conta nos jogadores únicos do overview
	if err := markPlayerSeen(ctx, tx, evt.GameID, evt.ModeID, evt.PlayerID); err != nil {
		return false, fmt.Errorf("analytics repository: apply participation: player seen: %w", err)
	}
	// stats brutos por jogador (spec 011, destravado por session-service/026):
	// match_id = session_id — host_world_dropin não tem matchId.
	if err := insertPlayerStats(ctx, tx, evt.GameID, evt.ModeID, evt.SessionID, evt.PlayerID, evt.Stats); err != nil {
		return false, fmt.Errorf("analytics repository: apply participation: stats: %w", err)
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
		INSERT INTO game_placement_rollup (game_id, mode_id, winners, top_n, rest, entries, sum_survived_ms)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (game_id, mode_id) DO UPDATE SET
			winners         = game_placement_rollup.winners + $3,
			top_n           = game_placement_rollup.top_n + $4,
			rest            = game_placement_rollup.rest + $5,
			entries         = game_placement_rollup.entries + $6,
			sum_survived_ms = game_placement_rollup.sum_survived_ms + $7`,
		pr.GameID, pr.ModeID, winners, topN, rest, entries, sumSurvived); err != nil {
		return false, fmt.Errorf("analytics repository: apply placement: rollup: %w", err)
	}

	// série diária por modo (spec 010).
	if _, err := tx.Exec(ctx, `
		INSERT INTO game_placement_daily (game_id, mode_id, day, matches)
		VALUES ($1, $2, (now() AT TIME ZONE 'utc')::date, 1)
		ON CONFLICT (game_id, mode_id, day) DO UPDATE SET matches = game_placement_daily.matches + 1`,
		pr.GameID, pr.ModeID); err != nil {
		return false, fmt.Errorf("analytics repository: apply placement: daily: %w", err)
	}

	// stats brutos por jogador (spec 011).
	for _, e := range pr.Entries {
		if err := markPlayerSeen(ctx, tx, pr.GameID, pr.ModeID, e.PlayerID); err != nil {
			return false, fmt.Errorf("analytics repository: player seen: %w", err)
		}
		if err := insertPlayerStats(ctx, tx, pr.GameID, pr.ModeID, pr.MatchID, e.PlayerID, e.Stats); err != nil {
			return false, fmt.Errorf("analytics repository: apply placement: stats: %w", err)
		}
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
		INSERT INTO game_mission_rollup (game_id, mode_id, successes, failures)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (game_id, mode_id) DO UPDATE SET
			successes = game_mission_rollup.successes + $3,
			failures  = game_mission_rollup.failures + $4`,
		cr.GameID, cr.ModeID, successDelta, failureDelta); err != nil {
		return false, fmt.Errorf("analytics repository: apply collective: rollup: %w", err)
	}

	// série diária por modo (spec 010).
	if _, err := tx.Exec(ctx, `
		INSERT INTO game_mission_daily (game_id, mode_id, day, matches)
		VALUES ($1, $2, (now() AT TIME ZONE 'utc')::date, 1)
		ON CONFLICT (game_id, mode_id, day) DO UPDATE SET matches = game_mission_daily.matches + 1`,
		cr.GameID, cr.ModeID); err != nil {
		return false, fmt.Errorf("analytics repository: apply collective: daily: %w", err)
	}

	// stats brutos por jogador (spec 011).
	for _, p := range cr.Players {
		if err := markPlayerSeen(ctx, tx, cr.GameID, cr.ModeID, p.PlayerID); err != nil {
			return false, fmt.Errorf("analytics repository: player seen: %w", err)
		}
		if err := insertPlayerStats(ctx, tx, cr.GameID, cr.ModeID, cr.MatchID, p.PlayerID, p.Stats); err != nil {
			return false, fmt.Errorf("analytics repository: apply collective: stats: %w", err)
		}
	}
	return false, tx.Commit(ctx)
}

// markPlayerSeen registra o jogador como visto no jogo/modo (base de
// "jogadores únicos"). Todos os arquétipos marcam — até a spec 016 só o
// queue_arena marcava e os outros jogos apareciam com 0 jogadores.
func markPlayerSeen(ctx context.Context, tx pgx.Tx, gameID, modeID, playerID string) error {
	if _, err := tx.Exec(ctx,
		`INSERT INTO game_player_seen (game_id, mode_id, player_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		gameID, modeID, playerID); err != nil {
		return fmt.Errorf("player seen: %w", err)
	}
	return nil
}

// insertPlayerStats grava as linhas de stats brutos de um jogador (spec 011),
// dentro da tx já aberta pelo Apply* chamador — sem proteção própria contra
// reentrega (a tx inteira já está condicionada ao processed_matches/
// processed_participations do chamador). No-op se stats vier vazio (jogo sem
// scoringStats).
func insertPlayerStats(ctx context.Context, tx pgx.Tx, gameID, modeID, matchID, playerID string, stats map[string]int64) error {
	for key, value := range stats {
		if _, err := tx.Exec(ctx, `
			INSERT INTO game_player_stats (game_id, mode_id, match_id, player_id, stat_key, stat_value)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			gameID, modeID, matchID, playerID, key, value); err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
	}
	return nil
}

// ApplySessionClosed agrega um fechamento de sessão numa única transação,
// idempotente por sessionID (spec 013, session-service/028). A marca em
// processed_sessions + o UPSERT do rollup vivem na mesma tx: reprocessar
// (at-least-once do SQS) é no-op.
func (r *PostgresRepository) ApplySessionClosed(ctx context.Context, evt domain.SessionCloseEvent) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("analytics repository: apply session closed: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op após Commit

	tag, err := tx.Exec(ctx,
		`INSERT INTO processed_sessions (session_id, game_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		evt.SessionID, evt.GameID)
	if err != nil {
		return false, fmt.Errorf("analytics repository: apply session closed: mark: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return true, tx.Commit(ctx) // já processado: no-op idempotente
	}

	durationMs := evt.ClosedAt.Sub(evt.OpenedAt).Milliseconds()
	if _, err := tx.Exec(ctx, `
		INSERT INTO game_session_rollup (game_id, mode_id, archetype, close_reason, count, duration_sum_ms)
		VALUES ($1, $2, $3, $4, 1, $5)
		ON CONFLICT (game_id, mode_id, close_reason) DO UPDATE
		  SET count = game_session_rollup.count + 1,
		      duration_sum_ms = game_session_rollup.duration_sum_ms + EXCLUDED.duration_sum_ms,
		      archetype = EXCLUDED.archetype`,
		evt.GameID, evt.ModeID, evt.Archetype, evt.CloseReason, durationMs); err != nil {
		return false, fmt.Errorf("analytics repository: apply session closed: rollup: %w", err)
	}

	// série diária (spec 015): dia derivado de evt.ClosedAt (não now()) —
	// correto mesmo em reprocessamento tardio da fila.
	if _, err := tx.Exec(ctx, `
		INSERT INTO game_session_daily (game_id, mode_id, day, count)
		VALUES ($1, $2, ($3 AT TIME ZONE 'utc')::date, 1)
		ON CONFLICT (game_id, mode_id, day) DO UPDATE SET count = game_session_daily.count + 1`,
		evt.GameID, evt.ModeID, evt.ClosedAt); err != nil {
		return false, fmt.Errorf("analytics repository: apply session closed: daily: %w", err)
	}
	return false, tx.Commit(ctx)
}

// Overview devolve os totais cross-game + um jogo por linha (spec 016). A base
// é a união dos jogos com sessão fechada (todos os arquétipos) e dos com
// partida de queue_arena — antes só o game_rollup (queue_arena) entrava, e o
// Folclore Souls (host_world_dropin) sumia do Analytics global.
func (r *PostgresRepository) Overview(ctx context.Context) (domain.Overview, error) {
	rows, err := r.pool.Query(ctx, `
		WITH games AS (
			SELECT game_id FROM game_session_rollup
			UNION
			SELECT game_id FROM game_rollup
		), sess AS (
			SELECT game_id, SUM(count) AS sessions, SUM(duration_sum_ms) AS dur,
			       -- abandono = o servidor/host sumiu (abandoned) ou a partida venceu o TTL sem
			       -- resultado. Mundo host_world_dropin que vence o TTL é o fim normal de um
			       -- mundo sem heartbeat (session/032), não abandono (revisão 2026-09-27).
			       COALESCE(SUM(count) FILTER (WHERE close_reason = 'abandoned'
			           OR (close_reason = 'ttl' AND archetype <> 'host_world_dropin')), 0) AS abandoned
			FROM game_session_rollup GROUP BY game_id
		), m AS (
			SELECT game_id, SUM(matches_total) AS matches, SUM(participations) AS parts,
			       SUM(wins) AS wins, SUM(losses) AS losses, SUM(draws) AS draws, SUM(score_sum) AS score
			FROM game_rollup GROUP BY game_id
		)
		SELECT g.game_id,
		       COALESCE(sess.sessions, 0), COALESCE(sess.dur, 0), COALESCE(sess.abandoned, 0),
		       COALESCE(m.matches, 0), COALESCE(m.parts, 0), COALESCE(m.wins, 0), COALESCE(m.losses, 0),
		       COALESCE(m.draws, 0), COALESCE(m.score, 0),
		       (SELECT count(DISTINCT s.player_id) FROM game_player_seen s WHERE s.game_id = g.game_id)
		FROM games g
		LEFT JOIN sess ON sess.game_id = g.game_id
		LEFT JOIN m ON m.game_id = g.game_id
		ORDER BY COALESCE(sess.sessions, 0) DESC, COALESCE(m.matches, 0) DESC, g.game_id`)
	if err != nil {
		return domain.Overview{}, fmt.Errorf("analytics repository: overview: %w", err)
	}
	defer rows.Close()

	var ov domain.Overview
	for rows.Next() {
		var g domain.OverviewGame
		var durSum, scoreSum int64
		if err := rows.Scan(&g.GameID, &g.Sessions, &durSum, &g.Abandoned,
			&g.Matches, &g.Participations, &g.Wins, &g.Losses, &g.Draws, &scoreSum, &g.UniquePlayers); err != nil {
			return domain.Overview{}, fmt.Errorf("analytics repository: scan overview: %w", err)
		}
		g.AvgScore = avg(scoreSum, g.Participations)
		if g.Sessions > 0 {
			g.AvgSessionMs = durSum / g.Sessions
		}
		ov.Games = append(ov.Games, g)
		ov.Totals.Sessions += g.Sessions
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

	// Série "sessões fechadas por dia" somada entre todos os jogos (últimos
	// overviewDailyDays) — a medida que existe para todo arquétipo.
	drows, err := r.pool.Query(ctx, `
		SELECT to_char(day, 'YYYY-MM-DD'), SUM(count) FROM game_session_daily
		WHERE day > (now() AT TIME ZONE 'utc')::date - $1::int -- N dias = hoje + N-1 anteriores
		GROUP BY day ORDER BY day`, overviewDailyDays)
	if err != nil {
		return domain.Overview{}, fmt.Errorf("analytics repository: overview daily: %w", err)
	}
	defer drows.Close()
	for drows.Next() {
		var d domain.SessionsPoint
		if err := drows.Scan(&d.Day, &d.Sessions); err != nil {
			return domain.Overview{}, fmt.Errorf("analytics repository: scan overview daily: %w", err)
		}
		ov.Daily = append(ov.Daily, d)
	}
	return ov, drows.Err()
}

// overviewDailyDays é a janela (dias) da série do overview cross-game.
const overviewDailyDays = 30

// GameAnalytics devolve o resumo do jogo + a série diária dos últimos `days`.
func (r *PostgresRepository) GameAnalytics(ctx context.Context, gameID string, days int, modeID string) (domain.GameAnalytics, error) {
	var out domain.GameAnalytics
	out.GameID = gameID
	out.ModeID = modeID

	// mode_id (spec 014): agregação, não mais QueryRow direto — pode haver
	// N linhas em game_rollup (uma por modo). modeID="" soma todas.
	var participations, scoreSum int64
	err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(r.matches_total),0), COALESCE(SUM(r.participations),0),
		       COALESCE(SUM(r.wins),0), COALESCE(SUM(r.losses),0), COALESCE(SUM(r.draws),0),
		       COALESCE(SUM(r.score_sum),0),
		       (SELECT count(DISTINCT s.player_id) FROM game_player_seen s
		          WHERE s.game_id = $1 AND ($2 = '' OR s.mode_id = $2))
		FROM game_rollup r WHERE r.game_id = $1 AND ($2 = '' OR r.mode_id = $2)`, gameID, modeID).
		Scan(&out.Matches, &participations, &out.Wins, &out.Losses, &out.Draws, &scoreSum, &out.UniquePlayers)
	if err != nil {
		return domain.GameAnalytics{}, fmt.Errorf("analytics repository: game analytics: %w", err)
	}
	out.Participations = participations
	out.AvgScore = avg(scoreSum, participations)

	// game_daily agora tem o mesmo shape das outras 3 séries diárias (spec
	// 014) — reusa o helper comum em vez de uma query bespoke.
	out.Daily, err = r.dailySeries(ctx, "game_daily", "matches", gameID, days, modeID)
	if err != nil {
		return domain.GameAnalytics{}, fmt.Errorf("analytics repository: daily: %w", err)
	}

	// séries diárias dos 3 arquétipos que não tinham (spec 010) — Matches
	// representa "events" em ParticipationDaily (host_world_dropin não tem
	// partida terminal, ver plan.md da 010), reaproveitando o mesmo campo.
	out.PlacementDaily, err = r.dailySeries(ctx, "game_placement_daily", "matches", gameID, days, modeID)
	if err != nil {
		return domain.GameAnalytics{}, fmt.Errorf("analytics repository: placement daily: %w", err)
	}
	out.MissionDaily, err = r.dailySeries(ctx, "game_mission_daily", "matches", gameID, days, modeID)
	if err != nil {
		return domain.GameAnalytics{}, fmt.Errorf("analytics repository: mission daily: %w", err)
	}
	out.ParticipationDaily, err = r.dailySeries(ctx, "game_participation_daily", "events", gameID, days, modeID)
	if err != nil {
		return domain.GameAnalytics{}, fmt.Errorf("analytics repository: participation daily: %w", err)
	}

	// participações por papel/evento/outcome (host_world_dropin, spec 004/007).
	// Aditivo: queue_arena não tem linhas aqui -> lista vazia. modeID (spec
	// 012): "" = sem filtro (comportamento atual).
	prows, err := r.pool.Query(ctx, `
		SELECT mode_id, role, event, outcome, count FROM game_participation_rollup
		WHERE game_id = $1 AND ($2 = '' OR mode_id = $2)
		ORDER BY mode_id, role, event, outcome`, gameID, modeID)
	if err != nil {
		return domain.GameAnalytics{}, fmt.Errorf("analytics repository: participations: %w", err)
	}
	defer prows.Close()
	for prows.Next() {
		var p domain.ParticipationStat
		if err := prows.Scan(&p.ModeID, &p.Role, &p.Event, &p.Outcome, &p.Count); err != nil {
			return domain.GameAnalytics{}, fmt.Errorf("analytics repository: scan participation: %w", err)
		}
		out.RoleParticipations = append(out.RoleParticipations, p)
	}
	if err := prows.Err(); err != nil {
		return domain.GameAnalytics{}, err
	}

	// distribuição de colocação (battle_royale, spec 005), agregada por modo
	// (spec 014). rowCount distingue "sem colocação neste jogo" (Placement
	// fica nil) de "zero" de verdade — agregação sempre devolve uma linha.
	var pd domain.PlacementDistribution
	var sumSurvived, placementRows int64
	err = r.pool.QueryRow(ctx, `
		SELECT COUNT(*), COALESCE(SUM(winners),0), COALESCE(SUM(top_n),0),
		       COALESCE(SUM(rest),0), COALESCE(SUM(entries),0), COALESCE(SUM(sum_survived_ms),0)
		FROM game_placement_rollup WHERE game_id = $1 AND ($2 = '' OR mode_id = $2)`, gameID, modeID).
		Scan(&placementRows, &pd.Winners, &pd.TopN, &pd.Rest, &pd.Entries, &sumSurvived)
	if err != nil {
		return domain.GameAnalytics{}, fmt.Errorf("analytics repository: placement: %w", err)
	}
	if placementRows > 0 {
		pd.AvgSurvivedMs = avg(sumSurvived, pd.Entries)
		out.Placement = &pd
	}

	// distribuição de desfecho da missão (coop_mission, spec 006), agregada
	// por modo (spec 014). Mesmo tratamento de rowCount do placement acima.
	var md domain.MissionDistribution
	var missionRows int64
	err = r.pool.QueryRow(ctx, `
		SELECT COUNT(*), COALESCE(SUM(successes),0), COALESCE(SUM(failures),0)
		FROM game_mission_rollup WHERE game_id = $1 AND ($2 = '' OR mode_id = $2)`, gameID, modeID).
		Scan(&missionRows, &md.Successes, &md.Failures)
	if err != nil {
		return domain.GameAnalytics{}, fmt.Errorf("analytics repository: mission: %w", err)
	}
	if missionRows > 0 {
		md.SuccessRate = ratio(md.Successes, md.Successes+md.Failures)
		out.Mission = &md
	}

	// resumo agregado de stats brutos (spec 011): soma/contagem/média entre
	// TODOS os jogadores e partidas do jogo (ou só o modo filtrado, spec
	// 012). Vazio = jogo sem scoringStats (ou sem dado nesse modo).
	srows, err := r.pool.Query(ctx, `
		SELECT stat_key, SUM(stat_value), COUNT(*)
		FROM game_player_stats WHERE game_id = $1 AND ($2 = '' OR mode_id = $2)
		GROUP BY stat_key ORDER BY stat_key`, gameID, modeID)
	if err != nil {
		return domain.GameAnalytics{}, fmt.Errorf("analytics repository: stats: %w", err)
	}
	defer srows.Close()
	for srows.Next() {
		var s domain.StatSummary
		if err := srows.Scan(&s.Key, &s.Sum, &s.Count); err != nil {
			return domain.GameAnalytics{}, fmt.Errorf("analytics repository: scan stats: %w", err)
		}
		s.Avg = avg(s.Sum, s.Count)
		out.Stats = append(out.Stats, s)
	}
	if err := srows.Err(); err != nil {
		return domain.GameAnalytics{}, err
	}

	// distribuição por motivo de fechamento (spec 013) — já nasce com
	// mode_id, sem a lacuna do item D que afeta os campos acima.
	hrows, err := r.pool.Query(ctx, `
		SELECT mode_id, archetype, close_reason, count, duration_sum_ms
		FROM game_session_rollup WHERE game_id = $1 AND ($2 = '' OR mode_id = $2)
		ORDER BY mode_id, close_reason`, gameID, modeID)
	if err != nil {
		return domain.GameAnalytics{}, fmt.Errorf("analytics repository: session health: %w", err)
	}
	defer hrows.Close()
	for hrows.Next() {
		var h domain.SessionCloseStat
		if err := hrows.Scan(&h.ModeID, &h.Archetype, &h.CloseReason, &h.Count, &h.DurationSumMs); err != nil {
			return domain.GameAnalytics{}, fmt.Errorf("analytics repository: scan session health: %w", err)
		}
		h.AvgDurationMs = avg(h.DurationSumMs, h.Count)
		out.SessionHealth = append(out.SessionHealth, h)
	}
	if err := hrows.Err(); err != nil {
		return domain.GameAnalytics{}, err
	}

	// série "sessões fechadas por dia" (spec 015) — mesmo helper das
	// outras 3 séries diárias, mesmo shape de tabela.
	out.SessionCloseDaily, err = r.dailySeries(ctx, "game_session_daily", "count", gameID, days, modeID)
	if err != nil {
		return domain.GameAnalytics{}, fmt.Errorf("analytics repository: session close daily: %w", err)
	}
	return out, nil
}

// dailySeries lê uma série diária somando entre modos, ou só o modo
// filtrado (spec 012): table e valueCol são identificadores FIXOS do
// código (nunca entrada do usuário) — mesmo padrão de leitura de
// game_daily, generalizado pras 3 tabelas novas (spec 010).
func (r *PostgresRepository) dailySeries(ctx context.Context, table, valueCol, gameID string, days int, modeID string) ([]domain.DailyPoint, error) {
	q := fmt.Sprintf(`
		SELECT to_char(day, 'YYYY-MM-DD'), SUM(%s) FROM %s
		WHERE game_id = $1 AND day > (now() AT TIME ZONE 'utc')::date - $2::int -- N dias, não N+1
		  AND ($3 = '' OR mode_id = $3)
		GROUP BY day ORDER BY day`, valueCol, table)
	rows, err := r.pool.Query(ctx, q, gameID, days, modeID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", table, err)
	}
	defer rows.Close()
	var out []domain.DailyPoint
	for rows.Next() {
		var d domain.DailyPoint
		if err := rows.Scan(&d.Day, &d.Matches); err != nil {
			return nil, fmt.Errorf("%s: scan: %w", table, err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
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

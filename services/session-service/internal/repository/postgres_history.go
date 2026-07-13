// Package repository implementa as Ports de saída de domain em PostgreSQL
// (pgx). Banco próprio do serviço (ADR-0015). Ver ADR-0002.
package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/domain"
)

var _ domain.HistoryStore = (*PostgresHistoryRepository)(nil)

// PostgresHistoryRepository persiste o histórico de sessões fechadas (spec
// 025), write-once: um INSERT por sessão em closeSession, nunca UPDATE/DELETE.
type PostgresHistoryRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresHistoryRepository cria o repositório a partir de um pool pgx.
func NewPostgresHistoryRepository(pool *pgxpool.Pool) *PostgresHistoryRepository {
	return &PostgresHistoryRepository{pool: pool}
}

// SaveClosed grava a sessão fechada e seus participantes numa transação
// (schema D4 do plan: closed_sessions + closed_session_players).
func (r *PostgresHistoryRepository) SaveClosed(ctx context.Context, sess *domain.Session) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("session history repository: save closed: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op após Commit

	var archetype, profileHash string
	if sess.Profile != nil {
		archetype = sess.Profile.Archetype
		profileHash = sess.Profile.Hash
	}

	// result/team_by_player (spec 027): nil quando a sessão não tem resultado
	// (fechamento sem ReportResult) ou times (FFA/host_world) — vira NULL, não
	// literal JSON "null" (senão json.Unmarshal na leitura teria que distinguir
	// os dois casos à toa).
	var resultParam, teamsParam any
	if sess.Result != nil {
		b, err := json.Marshal(sess.Result)
		if err != nil {
			return fmt.Errorf("session history repository: save closed: marshal result: %w", err)
		}
		resultParam = string(b)
	}
	if len(sess.TeamByPlayer) > 0 {
		b, err := json.Marshal(sess.TeamByPlayer)
		if err != nil {
			return fmt.Errorf("session history repository: save closed: marshal team_by_player: %w", err)
		}
		teamsParam = string(b)
	}

	const insertSession = `
		INSERT INTO closed_sessions
			(session_id, match_id, game_id, mode_id, archetype, origin,
			 host_player_id, close_reason, profile_hash, opened_at, closed_at,
			 result, team_by_player)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::jsonb, $13::jsonb)`
	if _, err := tx.Exec(ctx, insertSession,
		sess.ID, sess.MatchID, sess.GameID, sess.ModeID, archetype, string(sess.Origin),
		sess.HostPlayerID, string(sess.CloseReason), profileHash, sess.OpenedAt, sess.ClosedAt,
		resultParam, teamsParam,
	); err != nil {
		return fmt.Errorf("session history repository: save closed: insert session: %w", err)
	}

	const insertPlayer = `
		INSERT INTO closed_session_players (session_id, player_id, role, joined_at, last_event, last_outcome)
		VALUES ($1, $2, $3, $4, $5, $6)`
	for _, p := range sess.Participants {
		if _, err := tx.Exec(ctx, insertPlayer, sess.ID, p.PlayerID, p.Role, p.JoinedAt, p.LastEvent, p.LastOutcome); err != nil {
			return fmt.Errorf("session history repository: save closed: insert player: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("session history repository: save closed: commit: %w", err)
	}
	return nil
}

const closedSessionCols = `session_id, match_id, game_id, mode_id, archetype, origin, host_player_id, close_reason, profile_hash, opened_at, closed_at, result, team_by_player`

const closedSessionColsAliased = `cs.session_id, cs.match_id, cs.game_id, cs.mode_id, cs.archetype, cs.origin, cs.host_player_id, cs.close_reason, cs.profile_hash, cs.opened_at, cs.closed_at, cs.result, cs.team_by_player`

// ClosedByGame lista as sessões fechadas do jogo, mais recente primeiro,
// paginadas por closed_at (before zero = sem cursor).
func (r *PostgresHistoryRepository) ClosedByGame(ctx context.Context, gameID string, limit int, before time.Time) ([]domain.Session, error) {
	const q = `
		SELECT ` + closedSessionCols + `
		FROM closed_sessions
		WHERE game_id = $1 AND ($2::timestamptz IS NULL OR closed_at < $2)
		ORDER BY closed_at DESC, session_id DESC
		LIMIT $3`
	rows, err := r.pool.Query(ctx, q, gameID, beforeParam(before), limit)
	if err != nil {
		return nil, fmt.Errorf("session history repository: closed by game: %w", err)
	}
	sessions, err := scanClosedSessions(rows)
	if err != nil {
		return nil, fmt.Errorf("session history repository: closed by game: %w", err)
	}
	return r.attachParticipants(ctx, sessions)
}

// ClosedByPlayer lista as sessões fechadas do jogo em que o player participou,
// mesma paginação de ClosedByGame.
func (r *PostgresHistoryRepository) ClosedByPlayer(ctx context.Context, gameID, playerID string, limit int, before time.Time) ([]domain.Session, error) {
	const q = `
		SELECT ` + closedSessionColsAliased + `
		FROM closed_sessions cs
		JOIN closed_session_players csp ON csp.session_id = cs.session_id
		WHERE cs.game_id = $1 AND csp.player_id = $2 AND ($3::timestamptz IS NULL OR cs.closed_at < $3)
		ORDER BY cs.closed_at DESC, cs.session_id DESC
		LIMIT $4`
	rows, err := r.pool.Query(ctx, q, gameID, playerID, beforeParam(before), limit)
	if err != nil {
		return nil, fmt.Errorf("session history repository: closed by player: %w", err)
	}
	sessions, err := scanClosedSessions(rows)
	if err != nil {
		return nil, fmt.Errorf("session history repository: closed by player: %w", err)
	}
	return r.attachParticipants(ctx, sessions)
}

// attachParticipants busca os participantes das sessões da página numa
// segunda query (evita duplicar linhas de sessão por participante no join) e
// os agrupa por session_id.
func (r *PostgresHistoryRepository) attachParticipants(ctx context.Context, sessions []domain.Session) ([]domain.Session, error) {
	if len(sessions) == 0 {
		return sessions, nil
	}
	ids := make([]string, len(sessions))
	for i, s := range sessions {
		ids[i] = s.ID
	}

	const q = `
		SELECT session_id, player_id, role, joined_at, last_event, last_outcome
		FROM closed_session_players
		WHERE session_id = ANY($1)
		ORDER BY joined_at`
	rows, err := r.pool.Query(ctx, q, ids)
	if err != nil {
		return nil, fmt.Errorf("participants: %w", err)
	}
	defer rows.Close()

	byID := make(map[string][]domain.Participant)
	for rows.Next() {
		var sessionID string
		var p domain.Participant
		if err := rows.Scan(&sessionID, &p.PlayerID, &p.Role, &p.JoinedAt, &p.LastEvent, &p.LastOutcome); err != nil {
			return nil, fmt.Errorf("participants: scan: %w", err)
		}
		byID[sessionID] = append(byID[sessionID], p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("participants: rows: %w", err)
	}

	for i := range sessions {
		sessions[i].Participants = byID[sessions[i].ID]
	}
	return sessions, nil
}

func scanClosedSessions(rows pgx.Rows) ([]domain.Session, error) {
	defer rows.Close()
	var out []domain.Session
	for rows.Next() {
		var s domain.Session
		var archetype, profileHash, origin, closeReason string
		var resultRaw, teamsRaw []byte
		if err := rows.Scan(
			&s.ID, &s.MatchID, &s.GameID, &s.ModeID, &archetype, &origin,
			&s.HostPlayerID, &closeReason, &profileHash, &s.OpenedAt, &s.ClosedAt,
			&resultRaw, &teamsRaw,
		); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		s.Origin = domain.SessionOrigin(origin)
		s.CloseReason = domain.CloseReason(closeReason)
		s.State = domain.SessionClosed
		if archetype != "" || profileHash != "" {
			s.Profile = &domain.ProfileSnapshot{Archetype: archetype, Hash: profileHash}
		}
		if resultRaw != nil {
			var result domain.MatchOutcome
			if err := json.Unmarshal(resultRaw, &result); err != nil {
				return nil, fmt.Errorf("scan: unmarshal result: %w", err)
			}
			s.Result = &result
		}
		if teamsRaw != nil {
			if err := json.Unmarshal(teamsRaw, &s.TeamByPlayer); err != nil {
				return nil, fmt.Errorf("scan: unmarshal team_by_player: %w", err)
			}
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	return out, nil
}

// beforeParam converte o cursor de paginação para o parâmetro NULL-able que a
// query espera (before zero = sem cursor, primeira página).
func beforeParam(before time.Time) *time.Time {
	if before.IsZero() {
		return nil
	}
	return &before
}

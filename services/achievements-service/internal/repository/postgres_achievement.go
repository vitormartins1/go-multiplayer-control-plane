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

	"github.com/vitormartins1/go-multiplayer-control-plane/services/achievements-service/internal/domain"
)

// garante em tempo de compilação que satisfazemos a Port.
var _ domain.AchievementRepository = (*PostgresAchievementRepository)(nil)

// querier abstrai *pgxpool.Pool e pgx.Tx (ambos expõem Query/QueryRow/Exec) para
// reusar as mesmas queries dentro ou fora de transação — usado pelo fan-out
// idempotente (ApplyMatchResult) sem duplicar SQL.
type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// PostgresAchievementRepository persiste catálogo e progresso em PostgreSQL.
// Tudo é escopado por game_id (multi-tenant, ADR-0017).
type PostgresAchievementRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresAchievementRepository cria o repositório a partir de um pool pgx.
func NewPostgresAchievementRepository(pool *pgxpool.Pool) *PostgresAchievementRepository {
	return &PostgresAchievementRepository{pool: pool}
}

// achievementCols é a projeção comum do catálogo (ordem casada por scanAchievements).
const achievementCols = `game_id, id, name, description, event_type, stat_key, stat_mode, target`

func (r *PostgresAchievementRepository) ListCatalog(ctx context.Context, gameID string) ([]domain.Achievement, error) {
	const q = `SELECT ` + achievementCols + `
	           FROM achievements WHERE game_id = $1 ORDER BY id`
	rows, err := r.pool.Query(ctx, q, gameID)
	if err != nil {
		return nil, fmt.Errorf("achievements repository: list catalog: %w", err)
	}
	defer rows.Close()
	return scanAchievements(rows)
}

func (r *PostgresAchievementRepository) FindByEventType(ctx context.Context, gameID, eventType string) ([]domain.Achievement, error) {
	return findByEventType(ctx, r.pool, gameID, eventType)
}

// findByEventType roda a busca num querier (pool ou tx). Só critérios por evento
// (event_type <> ” — as conquistas por stat têm event_type vazio).
func findByEventType(ctx context.Context, q querier, gameID, eventType string) ([]domain.Achievement, error) {
	const sql = `SELECT ` + achievementCols + `
	             FROM achievements WHERE game_id = $1 AND event_type = $2 ORDER BY id`
	rows, err := q.Query(ctx, sql, gameID, eventType)
	if err != nil {
		return nil, fmt.Errorf("achievements repository: find by event type: %w", err)
	}
	defer rows.Close()
	return scanAchievements(rows)
}

func (r *PostgresAchievementRepository) FindByStatKey(ctx context.Context, gameID, statKey string) ([]domain.Achievement, error) {
	return findByStatKey(ctx, r.pool, gameID, statKey)
}

// findByStatKey roda a busca das conquistas por stat cujo stat_key casa.
func findByStatKey(ctx context.Context, q querier, gameID, statKey string) ([]domain.Achievement, error) {
	const sql = `SELECT ` + achievementCols + `
	             FROM achievements WHERE game_id = $1 AND stat_key = $2 ORDER BY id`
	rows, err := q.Query(ctx, sql, gameID, statKey)
	if err != nil {
		return nil, fmt.Errorf("achievements repository: find by stat key: %w", err)
	}
	defer rows.Close()
	return scanAchievements(rows)
}

// findByEventTypes busca, numa única query, as conquistas por evento cujo
// event_type está no conjunto informado — usada por ApplyMatchResult/
// ApplyParticipationEvent pra evitar o N+1 clássico (achado no /code-review):
// antes, uma partida com P jogadores × E event types cada rodava P×E queries
// separadas dentro da mesma transação. Devolve agrupado por event_type; a
// ordem por id dentro de cada grupo vem de graça (ORDER BY id é global, e um
// subconjunto de uma sequência ordenada continua ordenado).
func findByEventTypes(ctx context.Context, q querier, gameID string, eventTypes []string) (map[string][]domain.Achievement, error) {
	out := make(map[string][]domain.Achievement)
	if len(eventTypes) == 0 {
		return out, nil
	}
	const sql = `SELECT ` + achievementCols + `
	             FROM achievements WHERE game_id = $1 AND event_type = ANY($2) ORDER BY id`
	rows, err := q.Query(ctx, sql, gameID, eventTypes)
	if err != nil {
		return nil, fmt.Errorf("achievements repository: find by event types: %w", err)
	}
	defer rows.Close()
	list, err := scanAchievements(rows)
	if err != nil {
		return nil, err
	}
	for _, a := range list {
		out[a.EventType] = append(out[a.EventType], a)
	}
	return out, nil
}

// findByStatKeys é o equivalente de findByEventTypes para o critério por stat.
func findByStatKeys(ctx context.Context, q querier, gameID string, statKeys []string) (map[string][]domain.Achievement, error) {
	out := make(map[string][]domain.Achievement)
	if len(statKeys) == 0 {
		return out, nil
	}
	const sql = `SELECT ` + achievementCols + `
	             FROM achievements WHERE game_id = $1 AND stat_key = ANY($2) ORDER BY id`
	rows, err := q.Query(ctx, sql, gameID, statKeys)
	if err != nil {
		return nil, fmt.Errorf("achievements repository: find by stat keys: %w", err)
	}
	defer rows.Close()
	list, err := scanAchievements(rows)
	if err != nil {
		return nil, err
	}
	for _, a := range list {
		out[a.StatKey] = append(out[a.StatKey], a)
	}
	return out, nil
}

// distinctEventTypes/distinctStatKeys agregam as chaves de todos os jogadores
// da partida numa única lista sem duplicatas, pro batch de findByEventTypes/
// findByStatKeys rodar uma vez por transação em vez de uma vez por jogador.
func distinctEventTypes(players []domain.PlayerMatchInput) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, pe := range players {
		for _, et := range pe.EventTypes {
			if _, ok := seen[et]; !ok {
				seen[et] = struct{}{}
				out = append(out, et)
			}
		}
	}
	return out
}

func distinctStatKeys(players []domain.PlayerMatchInput) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, pe := range players {
		for statKey := range pe.Stats {
			if _, ok := seen[statKey]; !ok {
				seen[statKey] = struct{}{}
				out = append(out, statKey)
			}
		}
	}
	return out
}

// ProgressForAchievement devolve o progresso de todos os jogadores na conquista.
func (r *PostgresAchievementRepository) ProgressForAchievement(ctx context.Context, gameID, achievementID string) ([]domain.AchievementProgressEntry, error) {
	const q = `SELECT player_id, progress, unlocked_at
	           FROM player_achievements
	           WHERE game_id = $1 AND achievement_id = $2
	           ORDER BY progress DESC, player_id ASC`
	rows, err := r.pool.Query(ctx, q, gameID, achievementID)
	if err != nil {
		return nil, fmt.Errorf("achievements repository: progress for achievement: %w", err)
	}
	defer rows.Close()

	var out []domain.AchievementProgressEntry
	for rows.Next() {
		var e domain.AchievementProgressEntry
		var unlockedAt *time.Time
		if err := rows.Scan(&e.PlayerID, &e.Progress, &unlockedAt); err != nil {
			return nil, fmt.Errorf("achievements repository: progress for achievement scan: %w", err)
		}
		if unlockedAt != nil {
			e.Unlocked = true
			e.UnlockedAt = *unlockedAt
		}
		out = append(out, e)
	}
	return out, rows.Err()
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
	return recordProgress(ctx, r.pool, gameID, playerID, achievementID, target, at)
}

// recordProgress roda o incremento atômico num querier (pool ou tx).
func recordProgress(ctx context.Context, qr querier, gameID, playerID, achievementID string, target int, at time.Time) (bool, error) {
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
	err := qr.QueryRow(ctx, q, gameID, playerID, achievementID, target, at).Scan(&unlockedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil // WHERE barrou: já estava desbloqueada
	}
	if err != nil {
		return false, fmt.Errorf("achievements repository: record progress: %w", err)
	}
	return unlockedAt != nil, nil // desbloqueou nesta chamada?
}

// ApplyMatchResult aplica os eventos e stats da partida numa única transação,
// idempotente por matchID (ADR-0020). A marca em processed_matches + o progresso
// vivem na MESMA tx: reprocessar (entrega at-least-once do SQS) é no-op e um crash
// no meio não deixa a partida "processada" sem efeito. Como a tx roda uma única
// vez por matchID, os incrementos cumulativos de stat são seguros (não há dupla
// contagem, diferente de um ZINCRBY solto).
func (r *PostgresAchievementRepository) ApplyMatchResult(ctx context.Context, gameID, matchID string, players []domain.PlayerMatchInput, at time.Time) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("achievements repository: apply match: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op após Commit

	tag, err := tx.Exec(ctx,
		`INSERT INTO processed_matches (match_id, game_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		matchID, gameID)
	if err != nil {
		return false, fmt.Errorf("achievements repository: apply match: mark: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return true, tx.Commit(ctx) // já processada: no-op idempotente
	}

	// Busca o catálogo casado uma única vez por transação (achado no
	// /code-review: antes, findByEventType/findByStatKey rodavam uma query por
	// jogador × event type/stat, um N+1 clássico numa partida com muitos
	// jogadores).
	eventMatches, err := findByEventTypes(ctx, tx, gameID, distinctEventTypes(players))
	if err != nil {
		return false, err
	}
	statMatches, err := findByStatKeys(ctx, tx, gameID, distinctStatKeys(players))
	if err != nil {
		return false, err
	}

	for _, pe := range players {
		// critério por evento: +1 por ocorrência (como sempre).
		for _, et := range pe.EventTypes {
			for _, a := range eventMatches[et] {
				if _, err := recordProgress(ctx, tx, gameID, pe.PlayerID, a.ID, a.Target, at); err != nil {
					return false, err
				}
			}
		}
		// critério por stat: avança pela métrica (cumulative/per_match).
		for statKey, value := range pe.Stats {
			for _, a := range statMatches[statKey] {
				if _, err := recordStatProgress(ctx, tx, gameID, pe.PlayerID, a.ID, a.Target, value, a.StatMode, at); err != nil {
					return false, err
				}
			}
		}
	}
	return false, tx.Commit(ctx)
}

// ApplyParticipationEvent aplica os event types de um evento de participação
// (host_world_dropin, spec 013/009/012) numa única tx, idempotente por
// event_id (follow-up #4 — session_id/player_id/event preservados na linha
// só como referência/debug, não fazem mais parte da chave). A marca em
// processed_participations + o progresso vivem na MESMA tx (mesma garantia
// do ApplyMatchResult): reprocessar é no-op e um crash no meio não deixa o
// evento "processado" sem efeito. eventTypes vazio (evento sem conquista, ex.:
// LEFT) → só marca processado.
func (r *PostgresAchievementRepository) ApplyParticipationEvent(ctx context.Context, gameID, sessionID, playerID, event, eventID string, eventTypes []string, stats map[string]int64, at time.Time) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("achievements repository: apply participation: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op após Commit

	tag, err := tx.Exec(ctx,
		`INSERT INTO processed_participations (event_id, session_id, player_id, event, game_id)
		 VALUES ($1, $2, $3, $4, $5) ON CONFLICT (event_id) DO NOTHING`,
		effectiveEventID(eventID, sessionID, playerID, event), sessionID, playerID, event, gameID)
	if err != nil {
		return false, fmt.Errorf("achievements repository: apply participation: mark: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return true, tx.Commit(ctx) // já processado: no-op idempotente
	}

	// Mesmo batch de ApplyMatchResult (achado no /code-review): uma query por
	// transação em vez de uma por event type/stat.
	statKeys := make([]string, 0, len(stats))
	for statKey := range stats {
		statKeys = append(statKeys, statKey)
	}
	eventMatches, err := findByEventTypes(ctx, tx, gameID, eventTypes)
	if err != nil {
		return false, err
	}
	statMatches, err := findByStatKeys(ctx, tx, gameID, statKeys)
	if err != nil {
		return false, err
	}

	// critério por evento: +1 por ocorrência para cada event type traduzido.
	for _, et := range eventTypes {
		for _, a := range eventMatches[et] {
			if _, err := recordProgress(ctx, tx, gameID, playerID, a.ID, a.Target, at); err != nil {
				return false, err
			}
		}
	}
	// critério por stat (spec 018): mesmo loop de ApplyMatchResult — fecha a
	// assimetria do host_world_dropin com os outros 3 arquétipos.
	for statKey, value := range stats {
		for _, a := range statMatches[statKey] {
			if _, err := recordStatProgress(ctx, tx, gameID, playerID, a.ID, a.Target, value, a.StatMode, at); err != nil {
				return false, err
			}
		}
	}
	return false, tx.Commit(ctx)
}

// recordStatProgress avança o progresso de uma conquista por stat de forma
// atômica (ADR-0023 A1). cumulative soma o valor da partida ao acumulado;
// per_match guarda o melhor valor de partida. Em ambos o progress é capado em
// target, o desbloqueio é registrado uma única vez (COALESCE) e o WHERE evita
// mexer após o desbloqueio. delta é o valor do stat nesta partida, esperado
// >= 0 — mas vem de fora (dedicated server via envelope SQS, achado no
// /code-review), então um delta negativo malformado é clampado a 0 aqui: sem
// isso, o modo cumulative decrementaria (ou zeraria via LEAST) progresso já
// legítimo de partidas anteriores, e o INSERT inicial (novo jogador) gravaria
// progress negativo em qualquer modo.
func recordStatProgress(ctx context.Context, qr querier, gameID, playerID, achievementID string, target int, delta int64, mode string, at time.Time) (bool, error) {
	if delta < 0 {
		delta = 0
	}
	// A expressão de progresso difere pelo modo; o resto da instrução é igual.
	// cumulative: progress + delta ; per_match: max(progress, delta).
	// $6 (delta) precisa de cast explícito: sem ele o Postgres infere text (o
	// mesmo $6 aparece em vários contextos). progress é INT -> casta o resultado
	// (já capado em $4).
	progressExpr := "LEAST(player_achievements.progress + $6::bigint, $4)::int"
	unlockPred := "player_achievements.progress + $6::bigint >= $4"
	if mode == domain.StatModePerMatch {
		progressExpr = "LEAST(GREATEST(player_achievements.progress::bigint, $6::bigint), $4)::int"
		unlockPred = "$6::bigint >= $4"
	}
	q := `INSERT INTO player_achievements (game_id, player_id, achievement_id, progress, unlocked_at)
	      VALUES ($1, $2, $3, LEAST($6::bigint, $4)::int, CASE WHEN $6::bigint >= $4 THEN $5::timestamptz ELSE NULL END)
	      ON CONFLICT (game_id, player_id, achievement_id) DO UPDATE
	        SET progress    = ` + progressExpr + `,
	            unlocked_at = COALESCE(player_achievements.unlocked_at,
	                                   CASE WHEN ` + unlockPred + ` THEN $5::timestamptz END)
	        WHERE player_achievements.unlocked_at IS NULL
	      RETURNING unlocked_at`
	var unlockedAt *time.Time
	err := qr.QueryRow(ctx, q, gameID, playerID, achievementID, target, at, delta).Scan(&unlockedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil // WHERE barrou: já estava desbloqueada
	}
	if err != nil {
		return false, fmt.Errorf("achievements repository: record stat progress: %w", err)
	}
	return unlockedAt != nil, nil
}

// CreateAchievement adiciona uma conquista ao catálogo do jogo (admin).
// Violação de unicidade (game_id, id) vira ErrAchievementExists.
func (r *PostgresAchievementRepository) CreateAchievement(ctx context.Context, a *domain.Achievement) error {
	const q = `INSERT INTO achievements (game_id, id, name, description, event_type, stat_key, stat_mode, target)
	           VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err := r.pool.Exec(ctx, q, a.GameID, a.ID, a.Name, a.Description, a.EventType, a.StatKey, a.StatMode, a.Target)
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
// O critério (event_type/stat_key/stat_mode) define o que o progresso
// acumulado em player_achievements está medindo — se o update troca o
// critério (achado no /code-review: stat↔event ou stat_key/event_type
// diferentes), o progresso antigo passa a medir a coisa errada sob a
// semântica nova (ex.: 10 kills acumulados reaparecendo como "quase lá" num
// critério novo de vitórias). SELECT ... FOR UPDATE + reset dentro da mesma
// transação evita a corrida (update concorrente lendo o critério antigo
// depois do commit). Linha inexistente -> ErrAchievementNotFound.
func (r *PostgresAchievementRepository) UpdateAchievement(ctx context.Context, a *domain.Achievement) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("achievements repository: update achievement: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op após Commit

	var oldEventType, oldStatKey, oldStatMode string
	err = tx.QueryRow(ctx,
		`SELECT event_type, stat_key, stat_mode FROM achievements WHERE game_id = $1 AND id = $2 FOR UPDATE`,
		a.GameID, a.ID,
	).Scan(&oldEventType, &oldStatKey, &oldStatMode)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrAchievementNotFound
	}
	if err != nil {
		return fmt.Errorf("achievements repository: update achievement: select: %w", err)
	}

	const q = `UPDATE achievements
	           SET name = $3, description = $4, event_type = $5, stat_key = $6, stat_mode = $7, target = $8
	           WHERE game_id = $1 AND id = $2`
	if _, err := tx.Exec(ctx, q, a.GameID, a.ID, a.Name, a.Description, a.EventType, a.StatKey, a.StatMode, a.Target); err != nil {
		return fmt.Errorf("achievements repository: update achievement: %w", err)
	}

	if oldEventType != a.EventType || oldStatKey != a.StatKey || oldStatMode != a.StatMode {
		if _, err := tx.Exec(ctx,
			`DELETE FROM player_achievements WHERE game_id = $1 AND achievement_id = $2`,
			a.GameID, a.ID); err != nil {
			return fmt.Errorf("achievements repository: update achievement: reset progress: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("achievements repository: update achievement: commit: %w", err)
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
		if err := rows.Scan(&a.GameID, &a.ID, &a.Name, &a.Description, &a.EventType, &a.StatKey, &a.StatMode, &a.Target); err != nil {
			return nil, fmt.Errorf("achievements repository: scan: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

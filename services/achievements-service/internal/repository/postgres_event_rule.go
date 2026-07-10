package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/achievements-service/internal/domain"
)

// garante em tempo de compilação que satisfazemos a Port.
var _ domain.EventRuleRepository = (*PostgresEventRuleRepository)(nil)

// PostgresEventRuleRepository persiste as regras de mapeamento
// evento->achievement-event-type (migration 008) em PostgreSQL.
type PostgresEventRuleRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresEventRuleRepository cria o repositório a partir de um pool pgx.
func NewPostgresEventRuleRepository(pool *pgxpool.Pool) *PostgresEventRuleRepository {
	return &PostgresEventRuleRepository{pool: pool}
}

const eventRuleCols = `id, game_id, archetype, match, event_types`

// RulesForArchetype exclui as regras globais suprimidas por gameID (spec
// 017, V2b) — o caminho de avaliação (EvaluateRules) nunca chega a ver uma
// regra suprimida deste jogo; a exclusão é aqui, não no domain.
func (r *PostgresEventRuleRepository) RulesForArchetype(ctx context.Context, gameID, archetype string) ([]domain.EventRule, error) {
	const q = `SELECT ` + eventRuleCols + `
	           FROM event_rules
	           WHERE archetype = $1 AND (game_id IS NULL OR game_id = $2)
	             AND NOT EXISTS (
	               SELECT 1 FROM event_rule_suppressions s
	               WHERE s.game_id = $2 AND s.rule_id = event_rules.id
	             )
	           ORDER BY game_id NULLS FIRST, id`
	rows, err := r.pool.Query(ctx, q, archetype, gameID)
	if err != nil {
		return nil, fmt.Errorf("event rule repository: rules for archetype: %w", err)
	}
	defer rows.Close()
	return scanEventRules(rows)
}

// ListEffective computa Suppressed via LEFT JOIN — ao contrário de
// RulesForArchetype, aqui as globais suprimidas por gameID DEVEM aparecer
// (o admin precisa vê-las pra restaurar), só marcadas.
func (r *PostgresEventRuleRepository) ListEffective(ctx context.Context, gameID string) ([]domain.EventRule, error) {
	const q = `SELECT e.id, e.game_id, e.archetype, e.match, e.event_types, (s.rule_id IS NOT NULL) AS suppressed
	           FROM event_rules e
	           LEFT JOIN event_rule_suppressions s ON s.game_id = $1 AND s.rule_id = e.id
	           WHERE e.game_id IS NULL OR e.game_id = $1
	           ORDER BY e.archetype, e.game_id NULLS FIRST, e.id`
	rows, err := r.pool.Query(ctx, q, gameID)
	if err != nil {
		return nil, fmt.Errorf("event rule repository: list effective: %w", err)
	}
	defer rows.Close()
	return scanEventRulesWithSuppressed(rows)
}

// CreateRule persiste uma regra sempre escopada a um jogo (r.GameID não-nil —
// o service garante isso antes de chamar).
func (r *PostgresEventRuleRepository) CreateRule(ctx context.Context, rule *domain.EventRule) error {
	matchJSON, err := json.Marshal(rule.Match)
	if err != nil {
		return fmt.Errorf("event rule repository: marshal match: %w", err)
	}
	const q = `INSERT INTO event_rules (game_id, archetype, match, event_types)
	           VALUES ($1, $2, $3, $4) RETURNING id`
	if err := r.pool.QueryRow(ctx, q, rule.GameID, rule.Archetype, matchJSON, rule.EventTypes).Scan(&rule.ID); err != nil {
		return fmt.Errorf("event rule repository: create rule: %w", err)
	}
	return nil
}

// UpdateRule substitui match/event_types da regra (rule.ID, rule.GameID) —
// nunca uma global (WHERE game_id = $ não casa NULL, mesma proteção do
// delete). RETURNING preenche o archetype persistido (não editável) pro DTO
// da resposta sair completo sem um SELECT extra.
func (r *PostgresEventRuleRepository) UpdateRule(ctx context.Context, rule *domain.EventRule) error {
	matchJSON, err := json.Marshal(rule.Match)
	if err != nil {
		return fmt.Errorf("event rule repository: marshal match: %w", err)
	}
	const q = `UPDATE event_rules SET match = $1, event_types = $2
	           WHERE id = $3 AND game_id = $4 RETURNING archetype`
	err = r.pool.QueryRow(ctx, q, matchJSON, rule.EventTypes, rule.ID, rule.GameID).Scan(&rule.Archetype)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrEventRuleNotFound
	}
	if err != nil {
		return fmt.Errorf("event rule repository: update rule: %w", err)
	}
	return nil
}

// DeleteRule remove a regra (gameID, ruleID) — nunca uma global, já que o
// WHERE game_id = $2 nunca casa uma linha com game_id NULL. Zero linhas
// afetadas (não existe OU é global) -> ErrEventRuleNotFound (mesma resposta,
// não vaza a distinção).
func (r *PostgresEventRuleRepository) DeleteRule(ctx context.Context, gameID, ruleID string) error {
	const q = `DELETE FROM event_rules WHERE id = $1 AND game_id = $2`
	tag, err := r.pool.Exec(ctx, q, ruleID, gameID)
	if err != nil {
		return fmt.Errorf("event rule repository: delete rule: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrEventRuleNotFound
	}
	return nil
}

// Suppress grava a supressão (gameID, ruleID) — idempotente (ON CONFLICT DO
// NOTHING). Duas idas ao banco: a primeira confirma que a regra existe E é
// global (SELECT EXISTS). Sem essa checagem, um INSERT sozinho com ON
// CONFLICT DO NOTHING RETURNING não distinguiria "conflito porque já estava
// suprimida" (sucesso) de "SELECT vazio porque a regra não é global" (erro)
// — os dois casos devolvem zero linhas.
func (r *PostgresEventRuleRepository) Suppress(ctx context.Context, gameID, ruleID string) error {
	var exists bool
	const checkQ = `SELECT EXISTS(SELECT 1 FROM event_rules WHERE id = $1 AND game_id IS NULL)`
	if err := r.pool.QueryRow(ctx, checkQ, ruleID).Scan(&exists); err != nil {
		return fmt.Errorf("event rule repository: suppress check: %w", err)
	}
	if !exists {
		return domain.ErrEventRuleNotFound
	}
	const insertQ = `INSERT INTO event_rule_suppressions (game_id, rule_id) VALUES ($1, $2)
	                  ON CONFLICT (game_id, rule_id) DO NOTHING`
	if _, err := r.pool.Exec(ctx, insertQ, gameID, ruleID); err != nil {
		return fmt.Errorf("event rule repository: suppress insert: %w", err)
	}
	return nil
}

// Unsuppress remove a supressão (gameID, ruleID) — mesmo padrão de
// RowsAffected==0 -> ErrEventRuleNotFound do DeleteRule.
func (r *PostgresEventRuleRepository) Unsuppress(ctx context.Context, gameID, ruleID string) error {
	const q = `DELETE FROM event_rule_suppressions WHERE game_id = $1 AND rule_id = $2`
	tag, err := r.pool.Exec(ctx, q, gameID, ruleID)
	if err != nil {
		return fmt.Errorf("event rule repository: unsuppress: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrEventRuleNotFound
	}
	return nil
}

func scanEventRules(rows pgx.Rows) ([]domain.EventRule, error) {
	var out []domain.EventRule
	for rows.Next() {
		var e domain.EventRule
		var gameID *string
		var matchJSON []byte
		if err := rows.Scan(&e.ID, &gameID, &e.Archetype, &matchJSON, &e.EventTypes); err != nil {
			return nil, fmt.Errorf("event rule repository: scan: %w", err)
		}
		e.GameID = gameID
		if err := unmarshalMatch(&e, matchJSON); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// scanEventRulesWithSuppressed é a variante usada por ListEffective — mesmas
// colunas + o booleano computado pelo LEFT JOIN.
func scanEventRulesWithSuppressed(rows pgx.Rows) ([]domain.EventRule, error) {
	var out []domain.EventRule
	for rows.Next() {
		var e domain.EventRule
		var gameID *string
		var matchJSON []byte
		if err := rows.Scan(&e.ID, &gameID, &e.Archetype, &matchJSON, &e.EventTypes, &e.Suppressed); err != nil {
			return nil, fmt.Errorf("event rule repository: scan: %w", err)
		}
		e.GameID = gameID
		if err := unmarshalMatch(&e, matchJSON); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// unmarshalMatch decodifica o JSONB de match e loga (sem falhar) condições
// malformadas — decode leniente (spec 014): nunca casam, mas nunca
// silenciosamente sem rastro. O warning mora aqui (o domain é puro, sem I/O).
func unmarshalMatch(e *domain.EventRule, matchJSON []byte) error {
	if err := json.Unmarshal(matchJSON, &e.Match); err != nil {
		return fmt.Errorf("event rule repository: unmarshal match: %w", err)
	}
	for k, c := range e.Match {
		if !c.Valid() {
			slog.Warn("event rule com condição inválida (nunca casa)",
				slog.String("rule_id", e.ID), slog.String("key", k), slog.String("op", c.Op))
		}
	}
	return nil
}

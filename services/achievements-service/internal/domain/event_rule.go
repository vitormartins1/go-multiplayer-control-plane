package domain

import (
	"context"
	"strconv"
)

// Arquétipos reconhecidos pelo avaliador de regras (espelham os arquétipos do
// game-registry, ADR-0023). Redeclarados aqui pelo mesmo motivo de
// participation.go/placement.go/collective.go: o achievements não importa o
// game-registry, só consome o vocabulário.
const (
	ArchetypeQueueArena      = "queue_arena"
	ArchetypeHostWorldDropIn = "host_world_dropin"
	ArchetypeBattleRoyale    = "battle_royale"
	ArchetypeCoopMission     = "coop_mission"
)

// EventRule é uma regra de mapeamento fact-match -> event types (substitui as
// funções hardcoded ParticipationEventTypes/PlacementEventTypes/
// CollectiveEventTypes e a lógica inline de win_loss_draw). GameID nil = regra
// global do arquétipo (aplica a todo jogo que usa esse arquétipo); GameID
// setado = override/adição específica de um jogo.
type EventRule struct {
	ID         string
	GameID     *string // nil = global
	Archetype  string
	Match      map[string]string
	EventTypes []string
}

// Facts é o mapa normalizado de um envelope recebido, independente do kind —
// a mesma forma que EvaluateRules casa contra o Match de cada regra. Todos os
// valores são strings (evita ambiguidade float64/int na comparação com o que
// vem decodificado do JSONB).
type Facts map[string]string

// BattleRoyaleTopNThreshold é o limiar do "top N" — parâmetro numérico do
// arquétipo, não vocabulário de evento; permanece hardcoded na normalização
// (fora do escopo da generalização: adotar configurável só por evidência
// futura, mesmo espírito de outras decisões deste repo).
const BattleRoyaleTopNThreshold = 10

// matchSubset diz se m é subconjunto exato de facts (toda chave de m existe
// em facts com o mesmo valor). m vazio sempre bate (ex.: "played").
func matchSubset(m map[string]string, facts Facts) bool {
	for k, v := range m {
		if facts[k] != v {
			return false
		}
	}
	return true
}

// EvaluateRules casa TODAS as regras aplicáveis (Archetype bate E (GameID nil
// OU == gameID do evento)) cujo Match é subconjunto exato de facts, unindo
// (sem duplicar) os EventTypes das que baterem. Função pura e testável — a
// política do sink, agora data-driven em vez de hardcoded por arquétipo.
func EvaluateRules(rules []EventRule, gameID string, facts Facts) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, r := range rules {
		if r.GameID != nil && *r.GameID != gameID {
			continue
		}
		if !matchSubset(r.Match, facts) {
			continue
		}
		for _, et := range r.EventTypes {
			if _, dup := seen[et]; !dup {
				seen[et] = struct{}{}
				out = append(out, et)
			}
		}
	}
	return out
}

// --- Normalizadoras finas: só forma estrutural do envelope, sem regra de
// negócio (a regra vira dado, em EventRule.Match). ---

// ParticipationFacts normaliza um evento de participação (host_world_dropin).
func ParticipationFacts(role, event, outcome string) Facts {
	return Facts{"role": role, "event": event, "outcome": outcome}
}

// PlacementFacts normaliza uma colocação (battle_royale). O tier "top_n" é
// derivado aqui (não é vocabulário, é o limiar numérico do arquétipo).
func PlacementFacts(placement int) Facts {
	f := Facts{"placement": strconv.Itoa(placement)}
	if placement <= BattleRoyaleTopNThreshold {
		f["tier"] = "top_n"
	}
	return f
}

// CollectiveFacts normaliza o desfecho coletivo (coop_mission).
func CollectiveFacts(outcome string) Facts {
	return Facts{"outcome": outcome}
}

// WinLossDrawFacts normaliza o outcome de partida 1x1 (queue_arena).
func WinLossDrawFacts(outcome string) Facts {
	return Facts{"outcome": outcome}
}

// EventRuleService é a Port de entrada para a administração das regras
// (admin console). Escopado por gameID: ListEffective devolve as globais do
// arquétipo (scope="global") + as específicas do jogo (scope="game").
type EventRuleService interface {
	ListEffectiveRules(ctx context.Context, gameID string) ([]EventRule, error)
	// CreateEventRule cria uma regra escopada ao jogo (GameID sempre setado
	// pelo service, nunca aceito do payload). ErrInvalidEventRule em entrada
	// inválida (archetype desconhecido, match ou eventTypes vazios).
	CreateEventRule(ctx context.Context, gameID string, r EventRule) (*EventRule, error)
	// DeleteEventRule remove uma regra do jogo. ErrEventRuleNotFound se não
	// existir OU se for uma regra global (não editável por este endpoint —
	// mesma resposta pras duas situações, não vaza a distinção).
	DeleteEventRule(ctx context.Context, gameID, id string) error
}

// EventRuleRepository é a Port de saída de persistência das regras.
type EventRuleRepository interface {
	// RulesForArchetype devolve as regras globais (GameID nil) + as do
	// gameID, para o arquétipo — usado na avaliação (EvaluateRules).
	RulesForArchetype(ctx context.Context, gameID, archetype string) ([]EventRule, error)
	// ListEffective devolve globais + overrides do jogo, de TODOS os
	// arquétipos — usado pelo admin (painel).
	ListEffective(ctx context.Context, gameID string) ([]EventRule, error)
	// CreateRule persiste uma regra sempre com GameID não-nil (escopada).
	CreateRule(ctx context.Context, r *EventRule) error
	// DeleteRule remove a regra (gameID, ruleID). ErrEventRuleNotFound se não
	// existir ou se for global (WHERE game_id = $2 nunca casa NULL).
	DeleteRule(ctx context.Context, gameID, ruleID string) error
}

package domain

import (
	"context"
	"encoding/json"
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

// Operadores do match (spec 014, V2a do ADR-0028). OpEq é o default — a forma
// string do JSON ("chave": "valor") decodifica pra ele.
const (
	OpEq  = "eq"
	OpNe  = "ne"
	OpLt  = "lt"
	OpLte = "lte"
	OpGt  = "gt"
	OpGte = "gte"
	OpIn  = "in"
)

// MatchCond é UMA condição do match: operador + valor. No JSON aceita duas
// formas — string pura (igualdade, formato do ADR-0027, 100% retrocompatível)
// ou objeto {"op": "lte", "value": 10}. O decode é LENIENTE (op desconhecido
// ou value de tipo errado viram Value nil — condição que nunca casa, nunca
// erro): linha antiga/futura no banco não pode derrubar a avaliação de um
// envelope. A validação ESTRITA acontece na criação (validateEventRule).
type MatchCond struct {
	Op    string
	Value any // string (eq/ne) | float64 (lt/lte/gt/gte) | []string (in)
}

// Eq é o atalho pra condição de igualdade (a forma clássica do ADR-0027).
func Eq(v string) MatchCond { return MatchCond{Op: OpEq, Value: v} }

// EqMatch monta um match só de igualdades — açúcar pra testes e seeds no
// formato antigo (mapa de strings).
func EqMatch(pairs map[string]string) map[string]MatchCond {
	m := make(map[string]MatchCond, len(pairs))
	for k, v := range pairs {
		m[k] = Eq(v)
	}
	return m
}

func (c *MatchCond) UnmarshalJSON(b []byte) error {
	// forma string: igualdade exata (formato original).
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*c = Eq(s)
		return nil
	}
	// forma objeto: {op, value} — decode leniente por tipo do op.
	var raw struct {
		Op    string          `json:"op"`
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	c.Op = raw.Op
	c.Value = nil
	switch raw.Op {
	case OpEq, OpNe:
		var v string
		if json.Unmarshal(raw.Value, &v) == nil {
			c.Value = v
		}
	case OpLt, OpLte, OpGt, OpGte:
		var v float64
		if json.Unmarshal(raw.Value, &v) == nil {
			c.Value = v
		}
	case OpIn:
		var v []string
		if json.Unmarshal(raw.Value, &v) == nil && len(v) > 0 {
			c.Value = v
		}
	}
	return nil
}

func (c MatchCond) MarshalJSON() ([]byte, error) {
	// eq serializa como string pura — mantém o JSONB compacto e idêntico ao
	// formato do ADR-0027 pras regras de igualdade.
	if c.Op == OpEq {
		if s, ok := c.Value.(string); ok {
			return json.Marshal(s)
		}
	}
	return json.Marshal(struct {
		Op    string `json:"op"`
		Value any    `json:"value"`
	}{c.Op, c.Value})
}

// Valid diz se a condição é bem formada (op conhecido + value do tipo certo)
// — usada na validação estrita de criação.
func (c MatchCond) Valid() bool {
	switch c.Op {
	case OpEq, OpNe:
		_, ok := c.Value.(string)
		return ok
	case OpLt, OpLte, OpGt, OpGte:
		_, ok := c.Value.(float64)
		return ok
	case OpIn:
		v, ok := c.Value.([]string)
		return ok && len(v) > 0
	default:
		return false
	}
}

// EventRule é uma regra de mapeamento condições -> event types (ADR-0027 +
// V2a). GameID nil = regra global do arquétipo (aplica a todo jogo que usa
// esse arquétipo); GameID setado = override/adição específica de um jogo.
type EventRule struct {
	ID         string
	GameID     *string // nil = global
	Archetype  string
	Match      map[string]MatchCond
	EventTypes []string
	// Suppressed diz se esta regra global foi desligada para o jogo da
	// consulta (spec 017, V2b do ADR-0028) — só populado por
	// EventRuleRepository.ListEffective (leitura do admin); regras devolvidas
	// por RulesForArchetype (caminho de avaliação) já vêm sem as suprimidas,
	// então o campo fica no zero-value (false) ali. Sempre false pra regras
	// de jogo (o conceito não se aplica a overrides, que são removíveis).
	Suppressed bool
}

// Facts é o mapa normalizado de um envelope recebido, independente do kind —
// a mesma forma que EvaluateRules casa contra o Match de cada regra. Todos os
// valores são strings (evita ambiguidade float64/int na comparação com o que
// vem decodificado do JSONB); ops numéricos parseiam o fact na hora.
type Facts map[string]string

// FactMode é a chave do fact de modo de jogo (spec 013, F1 do ADR-0028) —
// cross-kind: todo envelope carrega modeId (ADR-0024, aditivo).
const FactMode = "mode"

// WithMode devolve os facts com o fact `mode` somado — decorator cross-kind
// (o modo vem do envelope, não da forma de resultado de cada arquétipo, por
// isso não mora nas normalizadoras). modeID vazio (envelope antigo) não gera
// a chave: regra com `mode=` simplesmente não casa, nunca casa com vazio.
func (f Facts) WithMode(modeID string) Facts {
	if modeID != "" {
		f[FactMode] = modeID
	}
	return f
}

// FactStatPrefix prefixa os facts vindos das stats do jogador (spec 015) —
// evita colisão com facts estruturais (um jogo com stat chamada "outcome"
// não conflita) e deixa a origem explícita no admin: stat.kills, stat.deaths.
const FactStatPrefix = "stat."

// WithStats devolve os facts com as stats do JOGADOR somadas como facts
// numéricos (stat.<chave>) — mesmo padrão de decorator do WithMode. Os
// comparadores do V2a (spec 014) fazem a comparação numérica; stat ausente
// no envelope = fact ausente = regra com stat.<x> não casa.
func (f Facts) WithStats(stats map[string]int64) Facts {
	for k, v := range stats {
		f[FactStatPrefix+k] = strconv.FormatInt(v, 10)
	}
	return f
}

// condMatches avalia UMA condição contra o valor do fact. Fact ausente nunca
// casa (nem no ne — desigualdade exige a chave presente); fact não-numérico
// nunca casa op numérico; op desconhecido/value nil nunca casa (defensivo —
// o warning fica na camada que carrega do banco, o domain é puro).
func condMatches(c MatchCond, factVal string, present bool) bool {
	if !present {
		return false
	}
	switch c.Op {
	case OpEq:
		v, ok := c.Value.(string)
		return ok && factVal == v
	case OpNe:
		v, ok := c.Value.(string)
		return ok && factVal != v
	case OpLt, OpLte, OpGt, OpGte:
		want, ok := c.Value.(float64)
		if !ok {
			return false
		}
		got, err := strconv.ParseFloat(factVal, 64)
		if err != nil {
			return false
		}
		switch c.Op {
		case OpLt:
			return got < want
		case OpLte:
			return got <= want
		case OpGt:
			return got > want
		default:
			return got >= want
		}
	case OpIn:
		list, ok := c.Value.([]string)
		if !ok {
			return false
		}
		for _, v := range list {
			if factVal == v {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// matchSubset diz se TODAS as condições de m casam contra facts. m vazio
// sempre bate (ex.: "played").
func matchSubset(m map[string]MatchCond, facts Facts) bool {
	for k, c := range m {
		v, present := facts[k]
		if !condMatches(c, v, present) {
			return false
		}
	}
	return true
}

// EvaluateRules casa TODAS as regras aplicáveis (Archetype bate E (GameID nil
// OU == gameID do evento)) cujo Match casa inteiro contra facts, unindo (sem
// duplicar) os EventTypes das que baterem. Função pura e testável — a
// política do sink, data-driven (ADR-0027 + V2a).
func EvaluateRules(rules []EventRule, gameID string, facts Facts) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, r := range rules {
		if r.Suppressed {
			// Defensivo: RulesForArchetype já exclui as globais suprimidas
			// deste jogo antes de chegar aqui — esta guarda só protege contra
			// um uso futuro indevido (ex.: passar o resultado de
			// ListEffective direto pro avaliador).
			continue
		}
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

// PlacementFacts normaliza uma colocação (battle_royale). O limiar do top-N
// deixou de ser derivado aqui (spec 014): virou a regra editável
// `placement lte N` na migration 009 — parâmetro por dado, não por código.
func PlacementFacts(placement int) Facts {
	return Facts{"placement": strconv.Itoa(placement)}
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
	// inválida (archetype desconhecido, condição malformada, eventTypes
	// vazios).
	CreateEventRule(ctx context.Context, gameID string, r EventRule) (*EventRule, error)
	// UpdateEventRule substitui match e eventTypes de uma regra DO JOGO,
	// preservando id e archetype (spec 016 — arquétipo não é editável).
	// ErrInvalidEventRule em conteúdo inválido; ErrEventRuleNotFound se a
	// regra não existir, for de outro jogo OU for global (mesma proteção do
	// delete).
	UpdateEventRule(ctx context.Context, gameID, id string, r EventRule) (*EventRule, error)
	// DeleteEventRule remove uma regra do jogo. ErrEventRuleNotFound se não
	// existir OU se for uma regra global (não editável por este endpoint —
	// mesma resposta pras duas situações, não vaza a distinção).
	DeleteEventRule(ctx context.Context, gameID, id string) error
	// SuppressEventRule desliga uma regra GLOBAL para o jogo gameID (spec
	// 017, V2b) — a linha da regra não é tocada; só deixa de contribuir
	// event types nas avaliações DESTE jogo (outros jogos do arquétipo
	// continuam recebendo normalmente). Idempotente (suprimir uma regra já
	// suprimida é um no-op). ErrEventRuleNotFound se a regra não existir ou
	// não for global (override de jogo não é suprimível — é removível via
	// DeleteEventRule).
	SuppressEventRule(ctx context.Context, gameID, id string) error
	// UnsuppressEventRule restaura uma regra global suprimida (volta a valer
	// para gameID). ErrEventRuleNotFound se não havia supressão ativa para o
	// par (gameID, id) — mesma semântica de "não existe" do DeleteEventRule.
	UnsuppressEventRule(ctx context.Context, gameID, id string) error
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
	// UpdateRule substitui match/event_types da regra (r.ID, r.GameID) e
	// preenche r.Archetype com o valor persistido. ErrEventRuleNotFound se
	// não existir ou for global (WHERE game_id = $ nunca casa NULL).
	UpdateRule(ctx context.Context, r *EventRule) error
	// DeleteRule remove a regra (gameID, ruleID). ErrEventRuleNotFound se não
	// existir ou se for global (WHERE game_id = $2 nunca casa NULL).
	DeleteRule(ctx context.Context, gameID, ruleID string) error
	// Suppress grava a supressão (gameID, ruleID) — idempotente (ON CONFLICT
	// DO NOTHING). ErrEventRuleNotFound se ruleID não existir ou não for
	// global.
	Suppress(ctx context.Context, gameID, ruleID string) error
	// Unsuppress remove a supressão (gameID, ruleID). ErrEventRuleNotFound se
	// não havia nenhuma.
	Unsuppress(ctx context.Context, gameID, ruleID string) error
}

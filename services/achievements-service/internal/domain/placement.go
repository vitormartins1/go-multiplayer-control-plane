package domain

// Event types de gameplay do battle_royale (BR3, ADR-0023). Devem casar com o
// event_type cadastrado no catálogo (data-driven) — o operador cria conquistas
// com esses gatilhos (ex.: "Vitória Battle Royale" com eventType battle_royale_win).
const (
	EventBattleRoyalePlayed = "battle_royale_played" // participou de uma partida BR
	EventBattleRoyaleWin    = "battle_royale_win"    // 1º lugar (colocação 1)
	EventBattleRoyaleTopN   = "battle_royale_top_n"  // terminou no top N
)

// StatSurvivedMs é a chave sob a qual a sobrevivência (survivedMs, campo de topo
// do placement) é exposta como stat, para os critérios por-métrica (spec 008)
// mirarem a sobrevivência no battle_royale sem código novo.
const StatSurvivedMs = "survived_ms"

// PlacementEntry é a colocação de um participante recebida do envelope
// kind=placement (session/015). Só os campos que o sink consome.
type PlacementEntry struct {
	PlayerID   string
	Placement  int
	SurvivedMs int64
	Stats      map[string]int64
}

// O mapeamento colocação -> event types (played/win/top_n) virou dado
// (migration 008, tabela event_rules, EvaluateRules + PlacementFacts em
// event_rule.go) — estas consts só nomeiam o vocabulário de saída.

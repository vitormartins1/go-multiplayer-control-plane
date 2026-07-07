package domain

// Event types de gameplay do battle_royale (BR3, ADR-0023). Devem casar com o
// event_type cadastrado no catálogo (data-driven) — o operador cria conquistas
// com esses gatilhos (ex.: "Vitória Battle Royale" com eventType battle_royale_win).
const (
	EventBattleRoyalePlayed = "battle_royale_played" // participou de uma partida BR
	EventBattleRoyaleWin    = "battle_royale_win"    // 1º lugar (colocação 1)
	EventBattleRoyaleTopN   = "battle_royale_top_n"  // terminou no top N
)

// BattleRoyaleTopNThreshold é o limiar do "top N" — política do sink, não vem do
// perfil (mantém o BR0 enxuto). Colocação <= 10 conta como top_n.
const BattleRoyaleTopNThreshold = 10

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

// PlacementEventTypes traduz uma colocação nos event types de gameplay que o
// catálogo casa (BR3). Função pura (política do sink), espelha
// ParticipationEventTypes. Todo participante gera "played"; o 1º lugar soma
// "win"; colocação <= limiar soma "top_n" (o 1º lugar acumula os três).
func PlacementEventTypes(placement int) []string {
	events := []string{EventBattleRoyalePlayed}
	if placement == 1 {
		events = append(events, EventBattleRoyaleWin)
	}
	if placement <= BattleRoyaleTopNThreshold {
		events = append(events, EventBattleRoyaleTopN)
	}
	return events
}

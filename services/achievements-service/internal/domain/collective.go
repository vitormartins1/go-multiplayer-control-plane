package domain

// Event types de gameplay do coop_mission (C3, ADR-0023). Devem casar com o
// event_type cadastrado no catálogo (data-driven) — o operador cria conquistas
// com esses gatilhos (ex.: "Missão cumprida" com eventType coop_mission_success).
const (
	EventCoopMissionPlayed  = "coop_mission_played"  // participou de uma missão
	EventCoopMissionSuccess = "coop_mission_success" // a missão foi cumprida (desfecho coletivo)
	EventCoopMissionFailed  = "coop_mission_failed"  // a missão falhou (wipe/objetivo perdido)
)

// Vocabulário do desfecho coletivo (espelha o session, ADR-0023/C2). O outcome
// é da SESSÃO inteira, igual para todos os jogadores.
const (
	MissionSuccess = "MISSION_SUCCESS"
	MissionFailure = "MISSION_FAILURE"
)

// CollectivePlayer é a participação de um jogador recebida do envelope
// kind=collective (session/016). Só os campos que o sink consome — o desfecho é
// coletivo (não vem por jogador); os stats modulam critérios por-métrica.
type CollectivePlayer struct {
	PlayerID string
	Stats    map[string]int64
}

// O mapeamento outcome coletivo -> event types (played/success/failed) virou
// dado (migration 008, tabela event_rules, EvaluateRules + CollectiveFacts em
// event_rule.go) — estas consts só nomeiam o vocabulário de saída.

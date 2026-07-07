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

// CollectiveEventTypes traduz o desfecho COLETIVO da sessão nos event types de
// gameplay que o catálogo casa (C3). Função pura (política do sink), espelha
// PlacementEventTypes. Todo jogador gera "played"; o desfecho da sessão soma
// "success" ou "failed" — igual para todos (não é por jogador). Desfecho fora do
// vocabulário: só "played" (rede de segurança; o session já validou).
func CollectiveEventTypes(outcome string) []string {
	events := []string{EventCoopMissionPlayed}
	switch outcome {
	case MissionSuccess:
		events = append(events, EventCoopMissionSuccess)
	case MissionFailure:
		events = append(events, EventCoopMissionFailed)
	}
	return events
}

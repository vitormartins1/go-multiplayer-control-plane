package domain

// Event types de gameplay derivados dos eventos de participação do
// host_world_dropin (spec 013/009). Devem casar com o event_type cadastrado no
// catálogo (data-driven) — o operador cria conquistas com esses gatilhos.
const (
	EventInvasionWon    = "invasion_won"     // venceu como invasor (matou o host)
	EventCoopBossHelped = "coop_boss_helped" // ajudou a matar o boss como co-op
	EventWorldJoined    = "world_joined"     // entrou num mundo (qualquer papel)
)

// Vocabulário dos eventos de participação recebidos no envelope (espelha a
// session/013; o achievements não os importa, então redeclara o que consome).
const (
	roleCoop    = "coop"
	roleInvader = "invader"

	participationJoined       = "JOINED"
	participationBossDefeated = "BOSS_DEFEATED"
	participationHostKilled   = "HOST_KILLED"

	outcomeSuccess = "SUCCESS"
)

// ParticipationEventTypes traduz um evento de participação (papel × evento ×
// outcome) nos event types de gameplay que o catálogo casa (spec 009). Função
// pura e testável — a política do sink. Evento sem conquista associada → nil.
func ParticipationEventTypes(role, event, outcome string) []string {
	switch {
	case role == roleInvader && event == participationHostKilled && outcome == outcomeSuccess:
		return []string{EventInvasionWon}
	case role == roleCoop && event == participationBossDefeated && outcome == outcomeSuccess:
		return []string{EventCoopBossHelped}
	case event == participationJoined:
		return []string{EventWorldJoined}
	default:
		return nil
	}
}

package repository

// effectiveEventID resolve a chave de idempotência de um evento de
// participação (follow-up #4, spec 007): usa o eventID vindo do data plane
// quando presente; sintetiza um fallback determinístico no formato da chave
// antiga (session_id:player_id:event) quando vazio — data plane anterior à
// spec 004 do Unity, que ainda não manda o campo. Duplicada deliberadamente
// de achievements-service/internal/repository (mesmo motivo do hash de senha
// em session/020 e matchmaking/011: serviços independentes, função trivial).
func effectiveEventID(eventID, sessionID, playerID, event string) string {
	if eventID != "" {
		return eventID
	}
	return sessionID + ":" + playerID + ":" + event
}

package repository

// effectiveEventID resolve a chave de idempotência de um evento de
// participação (follow-up #4, spec 012): usa o eventID vindo do data plane
// quando presente; sintetiza um fallback determinístico no formato da chave
// antiga (session_id:player_id:event) quando vazio — data plane anterior à
// spec 004 do Unity, que ainda não manda o campo. O fallback preserva o
// comportamento de dedupe pré-migration (uma vez por sessão/jogador/evento)
// sem exigir um branch condicional espalhado pelos call sites.
func effectiveEventID(eventID, sessionID, playerID, event string) string {
	if eventID != "" {
		return eventID
	}
	return sessionID + ":" + playerID + ":" + event
}

package domain

// KnownServices são os serviços Go do backend, na ordem de exibição. O ID casa
// com o `job` do Prometheus (para derivar o status de `up{job=ID}`).
var KnownServices = []ServiceNode{
	{ID: "identity-service", Label: "Identity"},
	{ID: "achievements-service", Label: "Achievements"},
	{ID: "player-data-service", Label: "Player Data"},
	{ID: "cloud-save-service", Label: "Cloud Save"},
	{ID: "leaderboard-service", Label: "Leaderboard"},
	{ID: "matchmaking-service", Label: "Matchmaking"},
	{ID: "session-service", Label: "Session"},
	{ID: "game-registry", Label: "Game Registry"},
	{ID: "admin-bff", Label: "Admin BFF"},
}

// IsKnownService informa se o id corresponde a um serviço conhecido (allowlist
// usada para validar o {id} do path antes de montar PromQL — evita 404 tardio
// e label arbitrário na query).
func IsKnownService(id string) bool {
	for _, n := range KnownServices {
		if n.ID == id {
			return true
		}
	}
	return false
}

// KnownEdges são as dependências conhecidas entre serviços (estáticas no MVP —
// ver ADR-0016 e os env do compose: IDENTITY_JWKS_URL / SESSION_GRPC_ADDR).
var KnownEdges = []Edge{
	{From: "achievements-service", To: "identity-service", Label: "valida JWT"},
	{From: "player-data-service", To: "identity-service", Label: "valida JWT"},
	{From: "cloud-save-service", To: "identity-service", Label: "valida JWT"},
	{From: "leaderboard-service", To: "identity-service", Label: "valida JWT"},
	{From: "matchmaking-service", To: "identity-service", Label: "valida JWT"},
	{From: "matchmaking-service", To: "session-service", Label: "provisiona sessão"},
	{From: "admin-bff", To: "achievements-service", Label: "lê achievements"},
}

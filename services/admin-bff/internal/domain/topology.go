package domain

// KnownServices são os serviços Go do backend, na ordem de exibição. O ID casa
// com o `job` do Prometheus (para derivar o status de `up{job=ID}`).
var KnownServices = []ServiceNode{
	{ID: "identity-service", Label: "Identity"},
	{ID: "achievements-service", Label: "Achievements"},
	{ID: "player-data-service", Label: "Player Data"},
	{ID: "cloud-save-service", Label: "Cloud Save"},
	{ID: "leaderboard-service", Label: "Leaderboard"},
	{ID: "analytics-service", Label: "Analytics"},
	{ID: "matchmaking-service", Label: "Matchmaking"},
	{ID: "session-service", Label: "Session"},
	{ID: "game-registry", Label: "Game Registry"},
	{ID: "admin-bff", Label: "Admin BFF"},
}

// KnownEdges são as dependências conhecidas entre serviços (estáticas —
// ADR-0016; conferidas contra os env do compose na revisão 2026-09, quando o
// analytics-service e as dependências do game-registry faltavam aqui).
var KnownEdges = []Edge{
	{From: "achievements-service", To: "identity-service", Label: "valida JWT"},
	{From: "player-data-service", To: "identity-service", Label: "valida JWT"},
	{From: "cloud-save-service", To: "identity-service", Label: "valida JWT"},
	{From: "leaderboard-service", To: "identity-service", Label: "valida JWT"},
	{From: "matchmaking-service", To: "identity-service", Label: "valida JWT"},
	{From: "matchmaking-service", To: "session-service", Label: "provisiona sessão / injeta no mundo"},
	{From: "matchmaking-service", To: "game-registry", Label: "lê política do modo"},
	{From: "session-service", To: "identity-service", Label: "valida JWT (ADR-0030)"},
	{From: "session-service", To: "game-registry", Label: "lê perfil do modo e fleet"},
	{From: "identity-service", To: "game-registry", Label: "marca presença (ADR-0019)"},
	{From: "admin-bff", To: "achievements-service", Label: "lê achievements"},
	{From: "admin-bff", To: "session-service", Label: "lê sessões"},
	{From: "admin-bff", To: "game-registry", Label: "lê jogos e modos"},
	{From: "admin-bff", To: "analytics-service", Label: "lê agregados"},
	{From: "admin-bff", To: "identity-service", Label: "lê jogadores"},
	{From: "admin-bff", To: "leaderboard-service", Label: "lê rankings"},
	{From: "admin-bff", To: "player-data-service", Label: "lê dados do jogador"},
	{From: "admin-bff", To: "cloud-save-service", Label: "lê saves"},
}

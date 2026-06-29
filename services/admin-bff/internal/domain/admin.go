// Package domain contém os tipos de negócio puros e as interfaces (Ports) do
// admin-bff. Zero dependências externas (ADR-0002). O admin-bff é um agregador
// read-only: não persiste nada; lê o Prometheus e as APIs dos serviços (ADR-0016).
package domain

import (
	"context"
	"time"
)

// Sample é um ponto de uma query instantânea ao Prometheus (labels + valor).
type Sample struct {
	Labels map[string]string
	Value  float64
}

// ServiceNode é um serviço no mapa de topologia. Up nil = desconhecido (a fonte
// de saúde, o Prometheus, está indisponível).
type ServiceNode struct {
	ID    string `json:"id"` // = job do Prometheus (ex.: "session-service")
	Label string `json:"label"`
	Up    *bool  `json:"up"`
}

// Edge é uma dependência conhecida entre dois serviços (aresta da topologia).
type Edge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label"`
}

// Topology é o mapa de serviços + dependências. SourceAvailable=false sinaliza
// que o Prometheus não respondeu (os nós vêm com Up nil).
type Topology struct {
	Nodes           []ServiceNode `json:"nodes"`
	Edges           []Edge        `json:"edges"`
	SourceAvailable bool          `json:"sourceAvailable"`
}

// LiveMetrics são os contadores operacionais de produto. Ponteiros: nil = a
// métrica não existe/ainda não foi exposta (não confundir com zero).
type LiveMetrics struct {
	QueueSize      *float64 `json:"queueSize"`
	ActiveSessions *float64 `json:"activeSessions"`
	MatchesTotal   *float64 `json:"matchesTotal"`
	MatchesFailed  *float64 `json:"matchesFailed"`
}

// PlayerAchievement é uma conquista do catálogo anotada com o estado do jogador.
type PlayerAchievement struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Target      int       `json:"target"`
	Progress    int       `json:"progress"`
	Unlocked    bool      `json:"unlocked"`
	UnlockedAt  time.Time `json:"unlockedAt,omitempty"`
}

// Player é uma conta de jogador na visão admin (User base). Nunca carrega
// credenciais — vem da API admin do identity (id/handle/createdAt).
type Player struct {
	ID        string    `json:"id"`
	Handle    string    `json:"handle"`
	CreatedAt time.Time `json:"createdAt"`
}

// Game é um jogo registrado (fonte: game-registry — ADR-0017).
type Game struct {
	ID        string    `json:"id"`
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

// Achievement é uma entrada do catálogo de conquistas de um jogo. EventType é
// usado só na criação (o catálogo lido não o expõe) — daí o omitempty.
type Achievement struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	EventType   string `json:"eventType,omitempty"`
	Target      int    `json:"target"`
}

// PlayerPage é uma página da lista de usuários + o total global.
type PlayerPage struct {
	Total   int      `json:"total"`
	Players []Player `json:"players"`
}

// MetricsSource é a Port de saída para o Prometheus (HTTP API, instant query).
type MetricsSource interface {
	// Query roda uma instant query (PromQL). Devolve ErrMetricsUnavailable se o
	// Prometheus estiver inacessível.
	Query(ctx context.Context, promQL string) ([]Sample, error)
}

// PlayerDirectory é a Port de saída para a lista de usuários (API admin do
// identity-service). Repassa a paginação; a política é do identity.
type PlayerDirectory interface {
	// ListPlayers lê a página de usuários. Devolve ErrIdentityUnavailable se o
	// identity estiver inacessível.
	ListPlayers(ctx context.Context, limit, offset int) (PlayerPage, error)
}

// GameRegistry é a Port de saída para o game-registry (gestão de jogos).
type GameRegistry interface {
	ListGames(ctx context.Context) ([]Game, error)
	// CreateGame registra um jogo. Erro de validação do upstream vira
	// UpstreamError (status repassado); indisponibilidade -> ErrGameRegistryUnavailable.
	CreateGame(ctx context.Context, slug, name string) (*Game, error)
}

// AchievementsReader é a Port de saída para a API admin do achievements-service,
// agora por jogo (multi-tenant, ADR-0017).
type AchievementsReader interface {
	// CatalogForGame lê o catálogo de conquistas do jogo.
	CatalogForGame(ctx context.Context, gameID string) ([]Achievement, error)
	// CreateAchievement cadastra uma conquista no jogo. Validação -> UpstreamError.
	CreateAchievement(ctx context.Context, gameID string, a Achievement) error
	// PlayerAchievements lê os achievements de um jogador no jogo.
	PlayerAchievements(ctx context.Context, gameID, playerID string) ([]PlayerAchievement, error)
}

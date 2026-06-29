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

// ServiceMetrics são as métricas de um serviço para a página de domínio: saúde,
// RED (Rate/Errors/Duration) e runtime. Ponteiros: nil = a métrica não existe
// para o serviço (ex.: RED é nil nos serviços gRPC, sem http_requests_total).
type ServiceMetrics struct {
	ID          string   `json:"id"`
	Up          *bool    `json:"up"`
	RequestRate *float64 `json:"requestRate"` // req/s
	ErrorRate   *float64 `json:"errorRate"`   // fração 0..1 (5xx / total)
	LatencyP95  *float64 `json:"latencyP95"`  // segundos
	Goroutines  *float64 `json:"goroutines"`
	MemoryBytes *float64 `json:"memoryBytes"`
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

// MetricsSource é a Port de saída para o Prometheus (HTTP API, instant query).
type MetricsSource interface {
	// Query roda uma instant query (PromQL). Devolve ErrMetricsUnavailable se o
	// Prometheus estiver inacessível.
	Query(ctx context.Context, promQL string) ([]Sample, error)
}

// AchievementsReader é a Port de saída para a API admin do achievements-service.
type AchievementsReader interface {
	// ForPlayer lê os achievements de um jogador. Devolve ErrAchievementsUnavailable
	// se o serviço estiver inacessível.
	ForPlayer(ctx context.Context, playerID string) ([]PlayerAchievement, error)
}

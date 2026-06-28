// Package domain contém os tipos de negócio puros e as interfaces (Ports)
// do achievements-service. Zero dependências externas. Ver ADR-0002.
package domain

import (
	"context"
	"time"
)

// Achievement é uma definição do catálogo de conquistas.
type Achievement struct {
	ID          string // slug estável, ex. "first_win"
	Name        string
	Description string
	EventType   string // tipo de evento que dispara o desbloqueio (critério MVP)
}

// Unlock é o desbloqueio de uma conquista por um jogador.
type Unlock struct {
	PlayerID      string
	AchievementID string
	UnlockedAt    time.Time
}

// PlayerAchievement é uma conquista do catálogo anotada com o estado do jogador.
type PlayerAchievement struct {
	Achievement
	Unlocked   bool
	UnlockedAt time.Time // zero quando ainda bloqueada
}

// Event é um evento de gameplay recebido para avaliação. O PlayerID nunca vem
// do corpo da requisição — é resolvido a partir do JWT (sub) no transport.
type Event struct {
	Type string
}

// AchievementService é a Port de entrada do achievements-service.
type AchievementService interface {
	ListCatalog(ctx context.Context) ([]Achievement, error)
	ListForPlayer(ctx context.Context, playerID string) ([]PlayerAchievement, error)
	// RecordEvent avalia o evento e desbloqueia as conquistas cujo critério
	// casa, de forma idempotente; retorna apenas as recém-desbloqueadas.
	RecordEvent(ctx context.Context, playerID string, event Event) (unlocked []Achievement, err error)
}

// AchievementRepository é a Port de saída de persistência (implementada em repository).
type AchievementRepository interface {
	ListCatalog(ctx context.Context) ([]Achievement, error)
	// FindByEventType retorna as conquistas disparadas por um tipo de evento.
	FindByEventType(ctx context.Context, eventType string) ([]Achievement, error)
	ListPlayerUnlocks(ctx context.Context, playerID string) ([]Unlock, error)
	// Unlock cria o desbloqueio se ainda não existir; created indica se foi
	// um desbloqueio novo (idempotência garantida no store).
	Unlock(ctx context.Context, playerID, achievementID string, at time.Time) (created bool, err error)
}

// TokenVerifier é a Port de saída que valida o JWT do identity e devolve o
// playerID (sub). Implementada em internal/auth via JWKS. O domínio não
// conhece HTTP nem o formato do token.
type TokenVerifier interface {
	Verify(ctx context.Context, token string) (playerID string, err error)
}

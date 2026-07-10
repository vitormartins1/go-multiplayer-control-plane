// Package domain contém os tipos de negócio puros e as interfaces (Ports)
// do achievements-service. Zero dependências externas. Ver ADR-0002.
package domain

import (
	"context"
	"time"
)

// Achievement é uma definição do catálogo de conquistas. O catálogo é por jogo
// (multi-tenant, ADR-0017): o ID (slug) é único POR GameID.
type Achievement struct {
	GameID      string // jogo dono da conquista (ADR-0017)
	ID          string // slug estável, ex. "first_win" — único por jogo
	Name        string
	Description string
	EventType   string // critério por evento: dispara o progresso (vazio se por stat)
	// Critério por métrica (ADR-0023, A1). StatKey != "" torna a conquista
	// "por stat": Target vira o threshold e StatMode define a agregação. Nesse
	// caso EventType é vazio (critérios mutuamente exclusivos).
	StatKey  string // ex.: "kills"; vazio = critério por evento
	StatMode string // "cumulative" | "per_match" (só com StatKey)
	Target   int    // evento: nº de ocorrências; stat: threshold (>= 1)
}

// Modos de agregação de um critério por stat (ADR-0023 A1).
const (
	StatModeCumulative = "cumulative" // soma o stat entre partidas
	StatModePerMatch   = "per_match"  // melhor stat de uma única partida
)

// PlayerProgress é o progresso de um jogador numa conquista.
type PlayerProgress struct {
	AchievementID string
	Progress      int
	UnlockedAt    time.Time // zero = em progresso (ainda não desbloqueada)
}

// AchievementProgressEntry é uma linha do relatório de progresso de uma
// conquista: quantos jogadores a desbloquearam, quantos estão em progresso.
type AchievementProgressEntry struct {
	PlayerID   string
	Progress   int
	Unlocked   bool
	UnlockedAt time.Time // zero quando ainda em progresso
}

// PlayerAchievement é uma conquista do catálogo anotada com o estado do jogador.
type PlayerAchievement struct {
	Achievement
	Progress   int
	Unlocked   bool
	UnlockedAt time.Time // zero quando ainda bloqueada
}

// Event é um evento de gameplay recebido para avaliação. O PlayerID nunca vem
// do corpo da requisição — é resolvido a partir do JWT (sub) no transport.
type Event struct {
	Type string
}

// MatchResult é o resultado de um jogador numa partida, entrada do fan-out
// durável (consumidor SQS, ADR-0020). O service traduz em eventos de gameplay.
type MatchResult struct {
	PlayerID string
	Won      bool             // mantido para o fallback (envelope v1 / Outcome ausente)
	Outcome  string           // "win" | "loss" | "draw" | "" (ausente) — ADR-0023
	Stats    map[string]int64 // métricas da partida (kills, assists, ...); ADR-0023
}

// PlayerMatchInput é o resultado de um jogador já traduzido pelo service: os
// eventos de gameplay (ex.: match_played, match_won) e os stats crus da partida.
// O repo aplica ambos numa transação idempotente por matchID (eventos por
// contagem; stats por métrica — ADR-0023 A1).
type PlayerMatchInput struct {
	PlayerID   string
	EventTypes []string
	Stats      map[string]int64
}

// AchievementService é a Port de entrada do achievements-service. Tudo é
// escopado por gameID (multi-tenant, ADR-0017).
type AchievementService interface {
	ListCatalog(ctx context.Context, gameID string) ([]Achievement, error)
	ListForPlayer(ctx context.Context, gameID, playerID string) ([]PlayerAchievement, error)
	// ProgressForAchievement devolve o progresso de todos os jogadores na
	// conquista {achievementID} do jogo {gameID} (admin, painel).
	ProgressForAchievement(ctx context.Context, gameID, achievementID string) ([]AchievementProgressEntry, error)
	// RecordEvent avalia o evento, incrementa o progresso das conquistas cujo
	// critério casa e desbloqueia as que atingem a meta; retorna apenas as
	// recém-desbloqueadas. Escopo: gameID.
	RecordEvent(ctx context.Context, gameID, playerID string, event Event) (unlocked []Achievement, err error)
	// CreateAchievement adiciona uma conquista ao catálogo do jogo (admin).
	// ErrInvalidAchievement em entrada inválida; ErrAchievementExists se o id
	// já existir nesse jogo.
	CreateAchievement(ctx context.Context, gameID string, a Achievement) (*Achievement, error)
	// UpdateAchievement edita name/description/eventType/target da conquista
	// (id e gameID são a chave, não mudam). ErrInvalidAchievement em entrada
	// inválida; ErrAchievementNotFound se não existir.
	UpdateAchievement(ctx context.Context, gameID, id string, a Achievement) (*Achievement, error)
	// DeleteAchievement remove a conquista e o progresso dos jogadores nela.
	// ErrAchievementNotFound se não existir.
	DeleteAchievement(ctx context.Context, gameID, id string) error
}

// AchievementRepository é a Port de saída de persistência (implementada em
// repository). Tudo é escopado por gameID.
type AchievementRepository interface {
	ListCatalog(ctx context.Context, gameID string) ([]Achievement, error)
	// FindByEventType retorna as conquistas do jogo disparadas por um event_type.
	FindByEventType(ctx context.Context, gameID, eventType string) ([]Achievement, error)
	// FindByStatKey retorna as conquistas por stat do jogo cujo stat_key casa
	// (critério por métrica, ADR-0023 A1).
	FindByStatKey(ctx context.Context, gameID, statKey string) ([]Achievement, error)
	ListPlayerProgress(ctx context.Context, gameID, playerID string) ([]PlayerProgress, error)
	// ProgressForAchievement devolve o progresso de todos os jogadores na
	// conquista {achievementID} do jogo, ordenado por progresso desc.
	ProgressForAchievement(ctx context.Context, gameID, achievementID string) ([]AchievementProgressEntry, error)
	// RecordProgress incrementa o progresso do jogador na conquista de forma
	// atômica e desbloqueia ao atingir target; é no-op se já desbloqueada.
	// justUnlocked indica se foi desbloqueada nesta chamada.
	RecordProgress(ctx context.Context, gameID, playerID, achievementID string, target int, at time.Time) (justUnlocked bool, err error)
	// ApplyMatchResult aplica os eventos e stats da partida numa única transação
	// e de forma idempotente por matchID (SQS entrega at-least-once, ADR-0020).
	// dup=true se a partida já havia sido processada (no-op).
	ApplyMatchResult(ctx context.Context, gameID, matchID string, players []PlayerMatchInput, at time.Time) (dup bool, err error)
	// ApplyParticipationEvent aplica os event types de um evento de participação
	// (host_world_dropin, spec 013/009/012) numa única transação, idempotente por
	// eventID (follow-up #4; vazio → fallback determinístico por
	// sessionID/playerID/event, ver effectiveEventID). eventTypes vazio → marca
	// processado sem progresso. stats (spec 018) avança achievements por
	// métrica na MESMA transação — nil/vazio é no-op. dup=true se o evento já
	// havia sido processado.
	ApplyParticipationEvent(ctx context.Context, gameID, sessionID, playerID, event, eventID string, eventTypes []string, stats map[string]int64, at time.Time) (dup bool, err error)
	// CreateAchievement persiste uma conquista no catálogo do jogo;
	// ErrAchievementExists em violação de unicidade (game_id, id).
	CreateAchievement(ctx context.Context, a *Achievement) error
	// UpdateAchievement atualiza name/description/event_type/target da conquista
	// (game_id, id); ErrAchievementNotFound se não existir.
	UpdateAchievement(ctx context.Context, a *Achievement) error
	// DeleteAchievement remove a conquista (game_id, id) e o progresso dos
	// jogadores nela (transação); ErrAchievementNotFound se não existir.
	DeleteAchievement(ctx context.Context, gameID, id string) error
}

// TokenVerifier é a Port de saída que valida o JWT do identity e devolve o
// playerID (sub). Implementada em internal/auth via JWKS. O domínio não
// conhece HTTP nem o formato do token.
type TokenVerifier interface {
	Verify(ctx context.Context, token string) (playerID string, err error)
}

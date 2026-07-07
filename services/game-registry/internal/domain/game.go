// Package domain contém os tipos de negócio puros e as interfaces (Ports) do
// game-registry. Zero dependências externas. Ver ADR-0002.
//
// O game-registry é a fonte canônica de jogos (ADR-0017): fornece o game_id
// (UUID) que os demais domínios usam como dimensão de multi-tenant.
package domain

import (
	"context"
	"time"
)

// Game é um jogo registrado no backend. O id (UUID) é a chave canônica usada
// pelos outros serviços; o slug é a chave humana estável.
//
// Capa (feature 003-generated-cover): sem upload no MVP, a imagem é gerada no
// cliente a partir de GeneratedImageSeed (default = slug) no estilo
// GeneratedImageStyle. Image (URL/data-uri) e ImageSource ("generated" | futuro
// "upload") deixam o espaço para upload futuro.
type Game struct {
	ID                  string
	Slug                string
	Name                string
	CreatedAt           time.Time
	Image               string // vazio = sem upload (reservado)
	ImageSource         string // "generated" no MVP
	GeneratedImageSeed  string // seed determinística (default = slug)
	GeneratedImageStyle string // estilo do gerador (default = "shapes")
	// MultiplayerProfile é o perfil de multiplayer do jogo (ADR-0023), nil se
	// ainda não configurado. Carregado por FindByID; List não o carrega.
	MultiplayerProfile *MultiplayerProfile
}

// Valores de vocabulário do perfil suportados (ADR-0023). O conjunto de
// arquétipos/topologias é fechado e expansível por evidência (sem DSL).
const (
	ArchetypeQueueArena      = "queue_arena"
	ArchetypeHostWorldDropIn = "host_world_dropin"
	TopologyDedicated        = "dedicated"
)

// MultiplayerProfile declara a *forma* do multiplayer de um jogo (ADR-0023): o
// arquétipo, a topologia e os parâmetros do arquétipo. É lido por matchmaking/
// session para se configurarem por jogo.
//
// Os parâmetros são polimórficos por arquétipo (tagged union por ponteiro):
// exatamente o ponteiro correspondente ao Archetype é não-nil. Novos arquétipos
// adicionam um campo-ponteiro próprio sem alterar os existentes.
type MultiplayerProfile struct {
	Archetype       string // "queue_arena" | "host_world_dropin"
	Topology        string // "dedicated" (único suportado; "p2p" = slot futuro)
	QueueArena      *QueueArenaParams
	HostWorldDropIn *HostWorldDropInParams
}

// QueueArenaParams são os parâmetros do arquétipo "queue_arena": sessão
// simétrica de N jogadores (FFA ou por times) que termina com um desfecho
// e, opcionalmente, stats. Ver spec 006 para as regras de validação.
type QueueArenaParams struct {
	Structure       string   // "ffa" | "teams"
	PlayerCount     int      // se ffa: total de jogadores (>= 2)
	TeamCount       int      // se teams: nº de times (>= 2)
	TeamSize        int      // se teams: jogadores por time (>= 1)
	WinCondition    string   // "first_to_score" | "highest_at_time" | "objective" | "elimination"
	ScoreTarget     int      // se first_to_score: pontuação-alvo (> 0)
	DurationSeconds int      // se highest_at_time: duração (> 0)
	MatchStructure  string   // "single" | "best_of_rounds"
	RoundsToWin     int      // se best_of_rounds: rounds para vencer (> 0)
	AllowDraw       bool     // empate permitido no desfecho
	ScoringStats    []string // métricas que o resultado carregará (pode ser vazia)
}

// HostWorldDropInParams são os parâmetros do arquétipo "host_world_dropin":
// mundo persistente de um host onde phantoms entram/saem com papéis assimétricos
// (co-op aliado e/ou invasor adversário). Inspirado nos Soulslike (ADR-0023,
// ficha 01). Ver profile_service.go para as regras de validação.
type HostWorldDropInParams struct {
	// EnabledRoles lista os papéis externos habilitados. Pelo menos um valor.
	// Valores: "coop" (aliado) e/ou "invader" (adversário).
	EnabledRoles []string
	// CoopSlots é o máximo de co-ops simultâneos. Obrigatório (>= 1) se "coop"
	// estiver em EnabledRoles; 0 caso contrário.
	CoopSlots int
	// InvaderSlots é o máximo de invasores simultâneos. Obrigatório (>= 1) se
	// "invader" estiver em EnabledRoles; 0 caso contrário.
	InvaderSlots int
	// LevelRangeEnabled: filtro de faixa de nível no matchmaking.
	LevelRangeEnabled bool
	// WeaponTierEnabled: filtro adicional de tier de arma/equipamento.
	WeaponTierEnabled bool
	// PasswordEnabled: senha de sessão que ignora as faixas e permite jogar
	// com qualquer nível.
	PasswordEnabled bool
	// InvasionEnabled: invasores podem entrar sem convite (mundo aparece como
	// alvo elegível). Se falso, só co-op (aliados convidados pelo host).
	InvasionEnabled bool
}

// GameService é a Port de entrada do game-registry.
type GameService interface {
	// Register valida e cria um jogo. ErrInvalidSlug/ErrInvalidName em entrada
	// inválida; ErrSlugTaken se o slug já existir.
	Register(ctx context.Context, slug, name string) (*Game, error)
	// List devolve os jogos por nome (asc).
	List(ctx context.Context) ([]Game, error)
	// Get devolve um jogo por id; ErrGameNotFound se ausente.
	Get(ctx context.Context, id string) (*Game, error)
	// Rename altera só o nome do jogo (slug/id são estáveis). ErrInvalidName em
	// entrada inválida; ErrGameNotFound se não existir.
	Rename(ctx context.Context, id, name string) (*Game, error)
	// SetMultiplayerProfile valida e faz upsert do perfil de multiplayer do jogo
	// (ADR-0023). ErrUnsupportedArchetype/ErrUnsupportedTopology/
	// ErrInvalidProfileParams em entrada inválida; ErrGameNotFound se o jogo não
	// existir. Devolve o perfil persistido.
	SetMultiplayerProfile(ctx context.Context, gameID string, p MultiplayerProfile) (*MultiplayerProfile, error)
	// GetMultiplayerProfile devolve o perfil do jogo; ErrGameNotFound se o jogo
	// não existir; ErrProfileNotFound se existir sem perfil configurado.
	GetMultiplayerProfile(ctx context.Context, gameID string) (*MultiplayerProfile, error)
}

// GameRepository é a Port de saída de persistência (implementada em repository).
type GameRepository interface {
	// Create persiste o jogo; ErrSlugTaken em violação de unicidade do slug.
	Create(ctx context.Context, g *Game) error
	// List devolve os jogos por nome (asc).
	List(ctx context.Context) ([]Game, error)
	// FindByID busca por id; ErrGameNotFound se ausente.
	FindByID(ctx context.Context, id string) (*Game, error)
	// UpdateName atualiza só o nome do jogo; ErrGameNotFound se não existir.
	UpdateName(ctx context.Context, id, name string) error
	// UpsertMultiplayerProfile persiste (cria ou substitui) o perfil de
	// multiplayer do jogo; ErrGameNotFound se o jogo não existir. A validação do
	// perfil é responsabilidade do service (o repositório só persiste).
	UpsertMultiplayerProfile(ctx context.Context, gameID string, p MultiplayerProfile) error
}

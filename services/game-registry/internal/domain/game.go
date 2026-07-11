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
	// MultiplayerProfile é o perfil do modo "default" (ADR-0023; compat interna
	// pós-ADR-0024), nil se ainda não configurado. Carregado por FindByID; List
	// não o carrega.
	MultiplayerProfile *MultiplayerProfile
	// MultiplayerModes é a coleção de modos do jogo (ADR-0024): "default"
	// primeiro, demais por ModeID asc. Carregada por FindByID; List não a carrega.
	MultiplayerModes []MultiplayerMode
	// MultiplayerModesVersion é o contador de concorrência otimista da coleção
	// de modos (achado no /code-review — revisão do "last-write-wins aceito"
	// da spec 009/plan.md): SetModes só persiste se a versão ainda bater com a
	// lida por este FindByID; senão devolve ErrModeConflict em vez de
	// sobrescrever silenciosamente uma edição concorrente.
	MultiplayerModesVersion int64
	// Fleet é o nome da Fleet Agones (label `agones.dev/fleet`) que aloca os game
	// servers deste jogo (ADR-0026). Por JOGO, não por modo: todos os modos do
	// mesmo jogo compartilham o mesmo binário/Fleet (o server lê `params_json` no
	// boot para se configurar por modo, ADR-0024) — só arquétipo/jogo diferente
	// precisa de imagem/Fleet diferente. Vazio = sem Agones real habilitado ainda
	// (o session-service recusa alocar via Agones nesse caso).
	Fleet string
	// Description é um texto curto livre sobre o jogo (spec 013, revisão de
	// layout do admin-ui — Identidade tinha pouco conteúdo). Opcional, até 500
	// caracteres; vazio é um estado válido (mesma convenção do Fleet).
	Description string
}

// MultiplayerMode é um perfil de multiplayer NOMEADO (ADR-0024): um jogo tem
// 1—N modos, cada um com um id estável (vai para ticket/token/envelope no M1)
// e o corpo do ADR-0023 intacto. Renomear a exibição é DisplayName; o ModeID é
// imutável na prática.
type MultiplayerMode struct {
	ModeID      string // slug estável: ^[a-z][a-z0-9-]{0,31}$; "default" = modo do alias legado
	DisplayName string // opcional; exibição no console
	Profile     MultiplayerProfile
}

// Valores de vocabulário do perfil suportados (ADR-0023). O conjunto de
// arquétipos/topologias é fechado e expansível por evidência (sem DSL).
const (
	ArchetypeQueueArena      = "queue_arena"
	ArchetypeHostWorldDropIn = "host_world_dropin"
	ArchetypeBattleRoyale    = "battle_royale"
	ArchetypeCoopMission     = "coop_mission"
	TopologyDedicated        = "dedicated"

	// DefaultModeID é o modo servido pelo alias legado /multiplayer-profile e o
	// fallback dos consumidores até o M1 (ADR-0024). Não pode ser removido.
	DefaultModeID = "default"

	// Modos de respawn do battle_royale (ADR-0023, ficha 03). Declarados no
	// perfil e lidos pelo data plane, que decide quando a colocação é fixada;
	// o control plane não implementa lógica de respawn.
	RespawnOneLife        = "one_life"
	RespawnTeammateRevive = "teammate_revive"
	RespawnArena1v1       = "arena_1v1"
	RespawnRebootTimer    = "reboot_timer"
)

// MultiplayerProfile declara a *forma* do multiplayer de um jogo (ADR-0023): o
// arquétipo, a topologia e os parâmetros do arquétipo. É lido por matchmaking/
// session para se configurarem por jogo.
//
// Os parâmetros são polimórficos por arquétipo (tagged union por ponteiro):
// exatamente o ponteiro correspondente ao Archetype é não-nil. Novos arquétipos
// adicionam um campo-ponteiro próprio sem alterar os existentes.
type MultiplayerProfile struct {
	Archetype       string // "queue_arena" | "host_world_dropin" | "battle_royale" | "coop_mission"
	Topology        string // "dedicated" (único suportado; "p2p" = slot futuro)
	QueueArena      *QueueArenaParams
	HostWorldDropIn *HostWorldDropInParams
	BattleRoyale    *BattleRoyaleParams
	CoopMission     *CoopMissionParams
	// ExtraParams são chaves de `params` fora do vocabulário tipado do
	// arquétipo (chave → valor JSON cru), preservadas sem validação
	// (follow-up #14 do arquétipo host_world_dropin — decodificar num struct
	// tipado por arquétipo descartava em silêncio qualquer chave que o
	// registry ainda não conhecesse). Repassadas ao data plane via
	// `params_json`/manifest tal como vieram; nil/vazio = nenhuma.
	ExtraParams map[string]string
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
	// ScoringStats são métricas extras que o resultado carregará (ex.: kills,
	// deaths); pode ser vazia. Adicionado depois dos outros 3 arquétipos —
	// achado usando o motor de event rules a fundo (o participation_event só
	// ganhou stats na spec achievements/018; até então não fazia sentido
	// declarar aqui).
	ScoringStats []string
}

// BattleRoyaleParams são os parâmetros do arquétipo "battle_royale": lobby
// grande, eliminação progressiva e resultado por colocação ordinal (ADR-0023,
// ficha 03). Ver profile_service.go para as regras de validação. A colocação e
// o tempo de sobrevivência são intrínsecos ao resultado (fatia BR2), não campos
// aqui; ScoringStats cobre só métricas extras opcionais (ex.: kills).
type BattleRoyaleParams struct {
	// LobbySize é o alvo de jogadores no lobby (>= 2 e >= 2*TeamSize).
	LobbySize int
	// TeamSize é o nº de jogadores por squad (>= 1; 1 = solo/FFA, 2 = duo, 4 = squad).
	TeamSize int
	// RespawnMode é o modo de respawn declarado (Respawn*); comportamento é do
	// data plane, não do control plane.
	RespawnMode string
	// ScoringStats são métricas extras que o resultado carregará além da
	// colocação/sobrevivência (ex.: kills); pode ser vazia.
	ScoringStats []string
}

// CoopMissionParams são os parâmetros do arquétipo "coop_mission": time pequeno
// contra o ambiente/IA que termina por objetivo cumprido ou wipe, com resultado
// COLETIVO — MISSION_SUCCESS/FAILURE igual para todos (ADR-0023, ficha 04). Ver
// profile_service.go para as regras de validação. O desfecho coletivo é
// intrínseco ao outcome (fatia C2, session), não campo aqui; Missions/Difficulties/
// BotfillEnabled são params declarados que o data plane lê (o control plane não
// implementa seleção de missão nem bot-fill); ScoringStats cobre métricas por
// jogador que modulam recompensa, não o desfecho.
type CoopMissionParams struct {
	// TeamSize é o nº de jogadores no time co-op (>= 2; ficha usa ~4, bots
	// preenchem vagas humanas).
	TeamSize int
	// Missions é o catálogo de missões que o host pode escolher (identificadores
	// estáveis, sem duplicata); pode ser vazia (catálogo mora no data plane).
	Missions []string
	// Difficulties é o catálogo de dificuldades (Hazard Level etc.; identificadores
	// estáveis, sem duplicata); pode ser vazia.
	Difficulties []string
	// BotfillEnabled: se vagas humanas são preenchidas por bots (comportamento do
	// data plane; aqui só declarado/armazenado).
	BotfillEnabled bool
	// ScoringStats são métricas extras que o resultado carregará (ex.: kills,
	// revives) e que modulam recompensa, não o desfecho; pode ser vazia.
	ScoringStats []string
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
	// Delete marca o jogo como removido (soft delete, ADR-0025): a partir daí o
	// jogo responde como inexistente em toda a API e o slug permanece reservado.
	// ErrGameNotFound se ausente ou já deletado.
	Delete(ctx context.Context, id string) error
	// SetFleet atualiza a Fleet Agones do jogo (ADR-0026). Vazio limpa o campo;
	// não-vazio precisa casar o formato de slug (ErrInvalidFleet caso contrário).
	// ErrGameNotFound se não existir.
	SetFleet(ctx context.Context, id, fleet string) (*Game, error)
	// SetDescription atualiza a descrição do jogo (spec 013). Vazio limpa o
	// campo; acima de 500 caracteres devolve ErrInvalidDescription.
	// ErrGameNotFound se não existir.
	SetDescription(ctx context.Context, id, description string) (*Game, error)
	// SetMultiplayerProfile valida e faz upsert do perfil de multiplayer do jogo
	// (ADR-0023). ErrUnsupportedArchetype/ErrUnsupportedTopology/
	// ErrInvalidProfileParams em entrada inválida; ErrGameNotFound se o jogo não
	// existir. Devolve o perfil persistido.
	SetMultiplayerProfile(ctx context.Context, gameID string, p MultiplayerProfile) (*MultiplayerProfile, error)
	// GetMultiplayerProfile devolve o perfil do jogo; ErrGameNotFound se o jogo
	// não existir; ErrProfileNotFound se existir sem perfil configurado.
	GetMultiplayerProfile(ctx context.Context, gameID string) (*MultiplayerProfile, error)

	// Modos nomeados (ADR-0024, spec 009). SetMode valida e faz upsert do modo
	// (ErrInvalidModeID/ErrTooManyModes/erros de perfil das specs 006-008);
	// DeleteMode protege o "default" (ErrDefaultModeProtected); GetMode/DeleteMode
	// devolvem ErrModeNotFound se o modo não existir. Todos: ErrGameNotFound se o
	// jogo não existir.
	SetMode(ctx context.Context, gameID, modeID, displayName string, p MultiplayerProfile) (*MultiplayerMode, error)
	// GetMode também devolve a Fleet do jogo dono do modo (ADR-0026, spec 012) —
	// já carregada no mesmo FindByID, sem query extra; consumida pelo
	// session-service (registryclient.ModeProfile) numa única chamada.
	GetMode(ctx context.Context, gameID, modeID string) (*MultiplayerMode, string, error)
	ListModes(ctx context.Context, gameID string) ([]MultiplayerMode, error)
	DeleteMode(ctx context.Context, gameID, modeID string) error
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
	// UpdateFleet atualiza a Fleet Agones do jogo (ADR-0026, spec 012);
	// ErrGameNotFound se não existir. Vazio limpa o campo.
	UpdateFleet(ctx context.Context, id, fleet string) error
	// UpdateDescription atualiza a descrição do jogo (spec 013); ErrGameNotFound
	// se não existir. Vazio limpa o campo.
	UpdateDescription(ctx context.Context, id, description string) error
	// SoftDelete marca deleted_at (ADR-0025); ErrGameNotFound se ausente ou já
	// deletado. Jogo deletado nunca sai deste repositório: List/FindByID/
	// UpdateName/SetModes o tratam como inexistente — o domínio não representa
	// o estado "deletado".
	SoftDelete(ctx context.Context, id string) error
	// SetModes substitui a coleção de modos do jogo (ADR-0024); ErrGameNotFound
	// se o jogo não existir. A validação é responsabilidade do service (o
	// repositório só persiste). O service faz read-modify-write via FindByID;
	// expectedVersion (lido do mesmo FindByID) protege essa janela por
	// concorrência otimista — ErrModeConflict se outra escrita mudou os modos
	// nesse meio tempo, em vez de sobrescrever silenciosamente (revisão do
	// "last-write-wins aceito" original da spec 009/plan.md — achado no
	// /code-review).
	SetModes(ctx context.Context, gameID string, modes []MultiplayerMode, expectedVersion int64) error
}

// Package domain contém os tipos de negócio puros e as interfaces (Ports) do
// session-service. Zero dependências externas. Ver ADR-0002.
package domain

import (
	"context"
	"time"
)

// SessionState é o estado do ciclo de vida da sessão (spec 004).
type SessionState string

// CloseReason é o motivo do fechamento da sessão (spec 004).
type CloseReason string

const (
	SessionOpen   SessionState = "open"
	SessionClosed SessionState = "closed"

	// CloseResult: fechada por ReportResult (fim de partida reportado).
	CloseResult CloseReason = "result"
	// CloseTTL: fechada pelo sweeper (aberta há mais que SESSION_TTL).
	CloseTTL CloseReason = "ttl"
	// CloseAdmin: encerrada por um operador no console (spec 007).
	CloseAdmin CloseReason = "admin"
	// CloseAbandoned: fechada pela reconciliação — o game server sumiu da fleet
	// sem ReportResult (spec 008, ADR-0022).
	CloseAbandoned CloseReason = "abandoned"
	// CloseHostLeft: mundo host_world encerrado limpo porque o host saiu
	// (spec 011, ADR-0023). Distinto de abandoned (crash) — encerramento voluntário.
	CloseHostLeft CloseReason = "host_left"
	// CloseHostKilled: mundo host_world encerrado porque o host foi morto (evento
	// HOST_KILLED, spec 013). Distinto de host_left (saída voluntária).
	CloseHostKilled CloseReason = "host_killed"
)

// SessionOrigin distingue como a sessão nasceu (ADR-0023, spec 011).
type SessionOrigin string

const (
	// OriginMatch: sessão criada a partir de um match (queue_arena, specs 001–010).
	OriginMatch SessionOrigin = "match"
	// OriginHostWorld: mundo persistente aberto por um host, sem match prévio
	// (host_world_dropin).
	OriginHostWorld SessionOrigin = "host_world"
)

// Papéis do participante num mundo host_world_dropin (ADR-0023, ficha 01). O
// host é o dono do mundo (não ocupa slot); coop/invader são phantoms com slots.
const (
	RoleHost    = "host"
	RoleCoop    = "coop"
	RoleInvader = "invader"
)

// RoleSlot é a capacidade e a ocupação de um papel num mundo. Occupants guarda
// os playerIDs presentes; len(Occupants) < Capacity indica slot livre.
type RoleSlot struct {
	Capacity  int
	Occupants []string
}

// Vocabulário dos eventos de participação (spec 013, ficha 01 §5). O server do
// mundo reporta cada evento ao longo da vida da sessão host_world_dropin.
const (
	EventJoined       = "JOINED"        // phantom entrou (confirmação; o slot já foi ocupado no JoinWorld)
	EventBossDefeated = "BOSS_DEFEATED" // boss da área morto (co-op ajudou)
	EventHostKilled   = "HOST_KILLED"   // o host morreu — encerra o mundo
	EventDied         = "DIED"          // phantom morto — libera o slot
	EventLeft         = "LEFT"          // phantom saiu — libera o slot

	OutcomeSuccess = "SUCCESS"
	OutcomeFail    = "FAIL"
	OutcomeNeutral = "NEUTRAL"
)

// ParticipationEvent é um desfecho por participante reportado pelo server do
// mundo (spec 013). Role vem do claim role do connect token (H0). O session
// valida só o vocabulário (Event/Outcome) — a interpretação de "quem venceu" é
// dos sinks (achievements/analytics).
type ParticipationEvent struct {
	PlayerID string
	Role     string
	Event    string // um de Event*
	Outcome  string // um de Outcome*
	// EventID identifica a ocorrência (follow-up #4, spec 021) — gerado pelo
	// data plane, ANTES do loop de retry do client (nunca aqui): repassado tal
	// como chega, sem gerar/validar. Vazio = data plane antigo (fallback dos
	// sinks). Só existe pros sinks (achievements/analytics) contarem certo
	// ocorrências repetidas do mesmo evento na mesma sessão (ex.: boss que
	// respawna); o session não usa isto pra nada (seus efeitos já são
	// idempotentes por natureza).
	EventID string
}

// TeamAssignment é um lado da partida NvN: um rótulo estável no match e seus
// jogadores (ADR-0023, arquétipo queue_arena). Em FFA a composição é um único
// assignment com TeamID vazio e todos os jogadores.
type TeamAssignment struct {
	TeamID  string
	Players []string
}

// Session é uma partida em andamento: o match alocado a um game server, ou o
// mundo persistente de um host (ADR-0023, spec 011). Origin distingue os dois.
type Session struct {
	ID      string
	MatchID string // vazio em host_world (não há match)
	GameID  string // jogo da partida (multi-tenant, ADR-0017)
	// ModeID e Profile são o modo e o SNAPSHOT do perfil congelados na criação
	// (ADR-0024, spec 017): mudar o perfil no registry não afeta a sessão em voo.
	// O snapshot alimenta o manifest (GetSessionConfig) e a validação
	// kind×arquétipo do ReportResult.
	ModeID        string
	Profile       *ProfileSnapshot
	Players       []string
	TeamByPlayer  map[string]string // playerID -> teamID; nil/vazio em FFA (ADR-0023)
	ServerID      string
	ServerAddress string

	// host_world_dropin (spec 011): Origin=host_world; HostPlayerID é o dono do
	// mundo; RoleSlots é a capacidade/ocupação por papel (coop/invader). Em
	// OriginMatch estes ficam zero/nil.
	Origin       SessionOrigin
	HostPlayerID string
	RoleSlots    map[string]*RoleSlot

	// Atributos de matchmaking do host (spec 012): a faixa do mundo para o
	// matchmaking casar um invasor (soul level / weapon tier). Zero em
	// queue_arena e em mundos abertos sem atributos.
	HostSoulLevel int
	WeaponTier    int

	// PasswordHash é o hash da senha do mundo (follow-up #1, spec 020): permite
	// jogar com amigos ignorando a faixa. Vazio = mundo sem senha. A senha em
	// texto puro nunca é persistida (só passa por WorldAttributes.Password na
	// chamada de OpenWorld, já hasheada antes de chegar aqui).
	PasswordHash string

	State       SessionState
	OpenedAt    time.Time
	ClosedAt    time.Time // zero enquanto OPEN
	CloseReason CloseReason

	// Result é o outcome gravado no fechamento por ReportResult (spec 018):
	// exatamente o que foi publicado no fan-out, com team_id já carimbado.
	// Nil em sessões abertas e nos fechamentos sem resultado (ttl/admin/
	// abandoned/host_left/host_killed).
	Result *MatchOutcome
}

// WorldAttributes são os atributos de matchmaking que o host informa ao abrir o
// mundo (spec 012): a faixa que o matchmaking usa para casar um invasor. Um
// struct (não ints posicionais) para o OpenWorld não crescer a assinatura a cada
// evolução (senha/região virão). Zeros = mundo sem faixa (não-casável).
type WorldAttributes struct {
	SoulLevel  int
	WeaponTier int
	// Password é a senha em texto puro informada pelo host (follow-up #1, spec
	// 020); vazia = sem senha. Só usada para computar PasswordHash no OpenWorld
	// — não sobrevive além da chamada.
	Password string
}

// InvadableWorld é a fotografia de um mundo aberto com vaga de invasor (spec
// 012): o que o matchmaking (006) precisa para filtrar por faixa e injetar. Fonte
// de dados pura — sem política de elegibilidade (isso mora no matchmaking).
type InvadableWorld struct {
	SessionID        string
	HostSoulLevel    int
	WeaponTier       int
	FreeInvaderSlots int
	// PasswordHash espelha Session.PasswordHash (spec 020) — o matchmaking
	// compara contra a senha que o invasor informou (matchmaking/011).
	PasswordHash string
}

// Server é um game server do pool (mock no MVP; Agones no deploy — ADR-0011).
type Server struct {
	ID      string
	Address string
}

// ConnectGrant é o connect token (JWT EdDSA) de um jogador para o game server.
type ConnectGrant struct {
	PlayerID string
	Token    string
}

// Arquétipos do ADR-0023 (vocabulário do game-registry) — usados na validação
// kind×arquétipo (spec 017) e na derivação de slots do host_world.
const (
	ArchetypeQueueArena      = "queue_arena"
	ArchetypeHostWorldDropIn = "host_world_dropin"
	ArchetypeBattleRoyale    = "battle_royale"
	ArchetypeCoopMission     = "coop_mission"

	// DefaultModeID é o modo assumido quando a entrada não declara um
	// (ADR-0024). A normalização acontece no transport.
	DefaultModeID = "default"
)

// Vocabulário do outcome (ADR-0023). win_loss_draw (queue_arena), placement
// (battle_royale, BR2) e collective (coop_mission, C2) são os kinds existentes.
const (
	OutcomeWinLossDraw = "win_loss_draw"
	OutcomePlacement   = "placement"  // battle_royale (ADR-0023/BR2)
	OutcomeCollective  = "collective" // coop_mission (ADR-0023/C2)

	ResultWin  = "win"
	ResultLoss = "loss"
	ResultDraw = "draw"

	// Desfecho coletivo do coop_mission (C2): da SESSÃO inteira, não por jogador.
	MissionSuccess = "MISSION_SUCCESS"
	MissionFailure = "MISSION_FAILURE"
)

// MatchOutcome é o resultado da partida, polimórfico por arquétipo (ADR-0023),
// modelado como tagged union por ponteiro: exatamente o ponteiro do Kind
// declarado é não-nil. WinLossDraw (queue_arena), Placement (battle_royale) e
// Collective (coop_mission); novos arquétipos adicionam um campo-ponteiro próprio.
type MatchOutcome struct {
	Kind        string // "win_loss_draw" | "placement" | "collective"
	WinLossDraw *WinLossDrawResult
	Placement   *PlacementResult
	Collective  *CollectiveResult
}

// WinLossDrawResult é o outcome do queue_arena: vitória/derrota/empate por
// jogador (e por time, via TeamID), com stats.
type WinLossDrawResult struct {
	Players []PlayerOutcome
}

// PlayerOutcome é o resultado de um jogador na partida (reportado pelo server).
type PlayerOutcome struct {
	PlayerID string
	TeamID   string // vazio em FFA
	Outcome  string // "win" | "loss" | "draw"
	Score    int64
	Stats    map[string]int64 // métricas declaradas (kills, assists, ...); pode ser nil
}

// PlacementResult é o outcome ordinal do battle_royale (ADR-0023/BR2): uma
// colocação por participante. Resultado terminal (a eliminação progressiva é
// gameplay do data plane).
type PlacementResult struct {
	Entries []PlacementEntry
}

// PlacementEntry é a colocação de um participante no battle_royale. Squads
// (squad-K, BR1) compartilham a colocação do time; o TeamID é carimbado pelo
// session a partir da formação (não confiado no que o server ecoa).
type PlacementEntry struct {
	PlayerID   string
	TeamID     string           // squad; vazio em solo/FFA
	Placement  int              // colocação ordinal (1 = vencedor)
	SurvivedMs int64            // tempo de sobrevivência (ms)
	Stats      map[string]int64 // métricas extras opcionais (ex.: kills); pode ser nil
}

// CollectiveResult é o outcome do coop_mission (ADR-0023/C2): um desfecho
// COLETIVO da sessão inteira (MISSION_SUCCESS/FAILURE igual para todos),
// terminal. Outcome vive no nível da sessão (não por jogador) — a diferença
// estrutural do arquétipo; os stats individuais modulam recompensa, não o
// desfecho.
type CollectiveResult struct {
	Outcome string // "MISSION_SUCCESS" | "MISSION_FAILURE" (da sessão)
	Players []CollectivePlayer
}

// CollectivePlayer é a participação de um jogador na missão: só o time e as
// métricas (sem outcome/won/score — o desfecho é coletivo). O TeamID é carimbado
// pelo session a partir da formação (não confiado no que o server ecoa).
type CollectivePlayer struct {
	PlayerID string
	TeamID   string           // time co-op (team-1)
	Stats    map[string]int64 // métricas individuais opcionais; pode ser nil
}

// SessionService é a Port de entrada: cria sessões a partir de matches e
// recebe o resultado da partida.
type SessionService interface {
	// Create aloca um game server, emite um connect token por jogador (claims
	// team_id/mode_id), congela o SNAPSHOT do perfil do modo (ADR-0024) e
	// registra a sessão. FFA = um único TeamAssignment com TeamID vazio.
	// ErrProfileUnavailable se o perfil do modo não puder ser lido (nada é
	// alocado); ErrNoServer se o pool esgotar; ErrInvalidTeams se a composição
	// for inconsistente.
	Create(ctx context.Context, matchID, gameID, modeID string, teams []TeamAssignment) (*Session, []ConnectGrant, error)
	// ReportResult valida o outcome (estrutural + kind×arquétipo do snapshot,
	// spec 017), dispara o fan-out, libera o server e encerra a sessão.
	// ErrUnsupportedOutcome/ErrInvalidOutcome em outcome inválido;
	// ErrOutcomeMismatch se o kind não corresponder ao arquétipo do modo;
	// ErrSessionNotFound se não houver sessão para o match.
	ReportResult(ctx context.Context, matchID string, outcome MatchOutcome) error

	// GetSessionConfig devolve o session manifest do snapshot (spec 017).
	// refID = matchID (queue-form) ou sessionID (host_world).
	// ErrSessionNotFound se a sessão não existir ou estiver fechada.
	GetSessionConfig(ctx context.Context, refID string) (*SessionManifest, error)

	// OpenWorld abre um mundo persistente para o host (host_world_dropin): lê o
	// perfil do jogo, aloca um game server, deriva os slots por papel e emite o
	// connect token do host (papel host). attrs carrega a faixa do host (soul
	// level / weapon tier) para o matchmaking casar depois (spec 012); zeros =
	// mundo sem faixa. ErrUnsupportedArchetype se o jogo não for host_world_dropin;
	// ErrNoServer se o pool esgotar. Spec 011/012, ADR-0023.
	OpenWorld(ctx context.Context, hostPlayerID, gameID, modeID string, attrs WorldAttributes) (*Session, ConnectGrant, error)
	// JoinWorld injeta um phantom no mundo com um papel (coop/invader), ocupando
	// um slot, e emite o connect token com o claim role. Devolve o endereço do
	// game server junto do grant (spec 012): o phantom injetado pelo matchmaking
	// precisa dele para conectar. ErrWorldNotFound, ErrRoleNotEnabled, ErrRoleFull,
	// ErrAlreadyJoined.
	JoinWorld(ctx context.Context, sessionID, playerID, role string) (ConnectGrant, string, error)
	// ListInvadableWorlds devolve os mundos abertos do jogo (Origin=host_world)
	// com slot de invasor livre, cada um com seus atributos e vagas (spec 012).
	// Fonte de dados pura: sem filtro de faixa/senha (política é do matchmaking).
	// Lista vazia (não erro) quando nada é elegível.
	ListInvadableWorlds(ctx context.Context, gameID string) ([]InvadableWorld, error)
	// LeaveWorld libera o slot do jogador; o mundo segue aberto. No-op idempotente
	// se o jogador não estiver no mundo. Se o jogador for o host, encerra o mundo.
	LeaveWorld(ctx context.Context, sessionID, playerID string) error
	// CloseWorld encerra o mundo (CloseHostLeft), libera o server e encerra a
	// participação de todos. ErrWorldNotFound se não houver mundo aberto.
	CloseWorld(ctx context.Context, sessionID string) error

	// ReportParticipationEvent recebe um evento por participante do server do
	// mundo (spec 013): publica no fan-out (kind=participation_event) e aplica o
	// efeito no slot (DIED/LEFT liberam; HOST_KILLED encerra o mundo). Publica
	// ANTES de mutar (ADR-0020). ErrInvalidParticipation (vocabulário),
	// ErrWorldNotFound (mundo inexistente/fechado).
	ReportParticipationEvent(ctx context.Context, sessionID string, evt ParticipationEvent) error
}

// ProfileSnapshot é o perfil de um MODO congelado na criação da sessão
// (ADR-0024, spec 017). ParamsJSON é o corpo cru dos params do arquétipo — vira
// o manifest que o data plane lê (só as chaves que conhece); Hash correlaciona
// control e data plane em log. Os campos de host_world (EnabledRoles/slots) são
// derivados na leitura e ficam zero nos demais arquétipos.
type ProfileSnapshot struct {
	ModeID     string
	Archetype  string // Archetype* (vocabulário do ADR-0023)
	Topology   string // "dedicated"
	ParamsJSON []byte // params crus do arquétipo (JSON)
	Hash       string // hash do snapshot (SHA-256 truncado, calculado no adapter)

	// host_world_dropin: capacidade por papel (spec 011).
	EnabledRoles []string // "coop" e/ou "invader"
	CoopSlots    int
	InvaderSlots int
}

// SessionManifest é a resposta do GetSessionConfig (spec 017): a visão do
// snapshot que o game server consome no boot da sessão.
type SessionManifest struct {
	SessionID   string
	MatchID     string // vazio em host_world
	GameID      string
	ModeID      string
	Archetype   string
	Topology    string
	ParamsJSON  []byte
	ProfileHash string
}

// ProfileProvider é a Port de saída que lê o perfil de um modo do jogo no
// game-registry (ADR-0023/0024). Devolve ErrProfileUnavailable quando o
// jogo/modo não existe ou a leitura falha.
type ProfileProvider interface {
	ModeProfile(ctx context.Context, gameID, modeID string) (ProfileSnapshot, error)
}

// ServerAllocator é a Port de saída do pool de game servers. O adapter static
// aloca de um pool fixo em memória; o Agones cria uma GameServerAllocation
// (ADR-0011, spec 004) levando os metadados da partida para o GameServer.
type ServerAllocator interface {
	// Allocate reserva um game server livre para a partida. matchID/gameID viram
	// annotations do GameServer no Agones (o static ignora). ErrNoServer se o
	// pool esgotar.
	Allocate(ctx context.Context, matchID, gameID string) (Server, error)
	// Release devolve o server ao pool, liberando-o para nova alocação.
	Release(ctx context.Context, serverID string) error
}

// ServerRegistry é a Port de saída OPCIONAL que consulta a liveness do pool:
// quais game servers ainda existem na fonte de verdade. Só o adapter Agones a
// implementa (LIST de GameServers da fleet); o static não (seus servers nunca
// somem). Quando o Service recebe registry nil, não reconcilia — cai só no TTL
// sweeper (spec 008, ADR-0022).
type ServerRegistry interface {
	// LiveServerIDs devolve o conjunto de IDs de game servers existentes na
	// fleet. O ID é o mesmo que Allocate grava em Server.ID.
	LiveServerIDs(ctx context.Context) (map[string]struct{}, error)
}

// ConnectClaims são os claims opcionais por arquétipo carregados no connect
// token (ADR-0023). Cada campo vira claim só quando não-vazio, para não poluir o
// token. Cresce por arquétipo sem alterar a assinatura de Issue.
type ConnectClaims struct {
	TeamID string // queue_arena: time do jogador na partida NvN
	Role   string // host_world_dropin: "host" | "coop" | "invader"
	// ModeID é o modo do jogo (ADR-0024, spec 017). Diferente dos demais, é
	// SEMPRE emitido (inclusive "default") — o game server sabe o modo no
	// connection approval sem chamada extra.
	ModeID string
}

// ConnectTokenIssuer é a Port de saída que cunha o connect token (JWT EdDSA)
// validado offline pelo game server via JWKS (ADR-0010/0011).
type ConnectTokenIssuer interface {
	// Issue emite um JWT EdDSA com sub/match_id/server/exp e os claims opcionais
	// (ADR-0023) — o game server os lê no connection approval. refID é o match_id
	// (queue_arena) ou o sessionID do mundo (host_world_dropin).
	Issue(playerID, refID, serverAddr string, claims ConnectClaims) (token string, err error)
	// JWKS devolve a chave pública (JSON) para verificação offline.
	JWKS(ctx context.Context) ([]byte, error)
}

// ResultPublisher é a Port de saída do fan-out do resultado. O adapter real
// (SNSPublisher) publica o resultado bruto no tópico match-results; cada sink
// consome sua fila SQS com idempotência por matchID (ADR-0020). LogPublisher é
// o fallback de dev. Recebe gameID (multi-tenant, ADR-0017) e matchID (chave de
// idempotência que viaja no envelope).
type ResultPublisher interface {
	// modeID viaja no envelope (ADR-0024, aditivo) — sinks só o usarão no M2.
	Publish(ctx context.Context, gameID, matchID, modeID string, outcome MatchOutcome) error
}

// ParticipationPublisher é a Port de saída do fan-out dos eventos de participação
// (spec 013, host_world_dropin). Publica no MESMO tópico match-results, com
// envelope kind=participation_event (decisão #3 do roadmap H2). O mesmo adapter
// (SNS/Log) satisfaz esta Port e a ResultPublisher.
type ParticipationPublisher interface {
	PublishParticipation(ctx context.Context, gameID, sessionID, modeID string, evt ParticipationEvent) error
}

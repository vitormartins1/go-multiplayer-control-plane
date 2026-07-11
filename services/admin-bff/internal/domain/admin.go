// Package domain contém os tipos de negócio puros e as interfaces (Ports) do
// admin-bff. Zero dependências externas (ADR-0002). O admin-bff é um agregador
// read-only: não persiste nada; lê o Prometheus e as APIs dos serviços (ADR-0016).
package domain

import (
	"context"
	"encoding/json"
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
// UnlockedAt é ponteiro: time.Time não-ponteiro nunca é omitido por
// `omitempty` (encoding/json só considera "vazio" nil/zero de tipos
// básicos/slice/map, nunca struct) — sem o ponteiro, toda conquista em
// progresso vazava o zero-time do Go (0001-01-01) pro front.
type PlayerAchievement struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Target      int        `json:"target"`
	Progress    int        `json:"progress"`
	Unlocked    bool       `json:"unlocked"`
	UnlockedAt  *time.Time `json:"unlockedAt,omitempty"`
}

// Player é uma conta de jogador na visão admin (User base). Nunca carrega
// credenciais — vem da API admin do identity (id/handle/createdAt).
type Player struct {
	ID        string    `json:"id"`
	Handle    string    `json:"handle"`
	CreatedAt time.Time `json:"createdAt"`
}

// Game é um jogo registrado (fonte: game-registry — ADR-0017). Os campos de
// capa (003-generated-cover) são repassados ao admin-ui, que gera a imagem
// determinística a partir da seed.
type Game struct {
	ID                  string    `json:"id"`
	Slug                string    `json:"slug"`
	Name                string    `json:"name"`
	CreatedAt           time.Time `json:"createdAt"`
	Image               string    `json:"image"`
	ImageSource         string    `json:"imageSource"`
	GeneratedImageSeed  string    `json:"generatedImageSeed"`
	GeneratedImageStyle string    `json:"generatedImageStyle"`
	// Fleet Agones do jogo (ADR-0026); "" = sem Fleet cadastrada. Só agora
	// exposto pelo BFF — o game-registry já devolve no GET /games desde a
	// spec 012, mas o campo nunca foi propagado até aqui (spec 059).
	Fleet string `json:"fleet"`
	// Description é um texto curto livre sobre o jogo (spec 013); "" = sem descrição.
	Description string `json:"description"`
	// Resumo dos modos de multiplayer (ADR-0024) — biblioteca de jogos
	// (revisão de layout, 2026-07-11): omitempty pois é best-effort (fan-out
	// paralelo em Service.Games, não derruba a listagem se um jogo falhar).
	MultiplayerModes []ModeSummary `json:"multiplayerModes,omitempty"`
}

// ModeSummary é o resumo de um modo nomeado (ADR-0024) — só o essencial pra
// identificar o arquétipo na listagem, sem carregar os params completos.
type ModeSummary struct {
	ModeID      string `json:"modeId"`
	DisplayName string `json:"displayName,omitempty"`
	Archetype   string `json:"archetype"`
}

// Achievement é uma entrada do catálogo de conquistas de um jogo. O critério é
// por evento (EventType) OU por stat (StatKey+StatMode, ADR-0023 A1) — omitempty
// mantém o JSON enxuto quando o campo não se aplica.
type Achievement struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	EventType   string `json:"eventType,omitempty"`
	StatKey     string `json:"statKey,omitempty"`
	StatMode    string `json:"statMode,omitempty"`
	Target      int    `json:"target"`
}

// Status de cada linha na importação em lote.
const (
	ImportStatusCreated   = "created"
	ImportStatusDuplicate = "duplicate"
	ImportStatusInvalid   = "invalid"
	ImportStatusError     = "error"
)

// ImportResult é o resultado de uma linha da importação CSV.
type ImportResult struct {
	Row    int    `json:"row"`
	ID     string `json:"id"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// ImportResponse é o envelope de resposta do endpoint de importação em lote.
type ImportResponse struct {
	Created int            `json:"created"`
	Failed  int            `json:"failed"`
	Results []ImportResult `json:"results"`
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
	// GetPlayer lê um usuário pelo id (spec 020). Devolve ErrPlayerNotFound
	// quando não existe; ErrIdentityUnavailable se o identity estiver inacessível.
	GetPlayer(ctx context.Context, playerID string) (Player, error)
}

// GamePlayer é uma entrada da player base de um jogo (presença, ADR-0019):
// o jogador que "está" no jogo, com o handle desnormalizado do login.
type GamePlayer struct {
	PlayerID  string    `json:"playerId"`
	Handle    string    `json:"handle"`
	FirstSeen time.Time `json:"firstSeen"`
}

// GameRegistry é a Port de saída para o game-registry (gestão de jogos +
// presença por jogo).
type GameRegistry interface {
	ListGames(ctx context.Context) ([]Game, error)
	// CreateGame registra um jogo. Erro de validação do upstream vira
	// UpstreamError (status repassado); indisponibilidade -> ErrGameRegistryUnavailable.
	CreateGame(ctx context.Context, slug, name string) (*Game, error)
	// RenameGame altera o nome do jogo. UpstreamError (400/404) propagado.
	RenameGame(ctx context.Context, id, name string) (*Game, error)
	// SetFleet cadastra/troca a Fleet Agones do jogo (ADR-0026). UpstreamError
	// (400/404) propagado. "" limpa o campo (sem Agones real habilitado).
	SetFleet(ctx context.Context, id, fleet string) (*Game, error)
	// SetDescription atualiza a descrição do jogo (spec 013). UpstreamError
	// (400/404) propagado. "" limpa o campo.
	SetDescription(ctx context.Context, id, description string) (*Game, error)
	// DeleteGame remove o jogo (soft delete no registry, ADR-0025).
	// UpstreamError(404) se inexistente/já deletado.
	DeleteGame(ctx context.Context, id string) error
	// PlayersOf lê a player base do jogo (presença, ADR-0019).
	PlayersOf(ctx context.Context, gameID string) ([]GamePlayer, error)
	// GamesOfPlayer lê os jogos em que o player está presente (consulta reversa).
	GamesOfPlayer(ctx context.Context, playerID string) ([]Game, error)
	// GetMultiplayerProfile lê o perfil de multiplayer do jogo (ADR-0023, cru).
	// UpstreamError(404) se o jogo ou o perfil não existir; indisponibilidade ->
	// ErrGameRegistryUnavailable. O BFF não interpreta a forma do perfil.
	GetMultiplayerProfile(ctx context.Context, gameID string) (json.RawMessage, error)
	// SetMultiplayerProfile faz upsert do perfil (cru). Validação/estado do
	// upstream (400/404) viram UpstreamError; indisponibilidade -> unavailable.
	SetMultiplayerProfile(ctx context.Context, gameID string, profile json.RawMessage) (json.RawMessage, error)
	// Modos nomeados (ADR-0024, spec 017): passthrough cru da coleção
	// /multiplayer-profiles. Erros do upstream (400/404/409) viram UpstreamError
	// com o código intacto; indisponibilidade -> ErrGameRegistryUnavailable.
	ListMultiplayerModes(ctx context.Context, gameID string) (json.RawMessage, error)
	GetMultiplayerMode(ctx context.Context, gameID, modeID string) (json.RawMessage, error)
	SetMultiplayerMode(ctx context.Context, gameID, modeID string, mode json.RawMessage) (json.RawMessage, error)
	DeleteMultiplayerMode(ctx context.Context, gameID, modeID string) error
}

// LeaderboardEntry é uma posição no ranking de um jogo (rank 1-based).
type LeaderboardEntry struct {
	Rank     int64  `json:"rank"`
	PlayerID string `json:"playerId"`
	Score    int64  `json:"score"`
}

// LeaderboardReader é a Port de saída para a API admin do leaderboard-service
// (top-N por jogo, ADR-0017). O board seleciona o ranking (score ou um por stat,
// ADR-0023 / leaderboard/003); "" -> o leaderboard-service assume "score".
type LeaderboardReader interface {
	// TopForGame lê a página [offset, offset+limit) do board na janela pedida
	// (spec 019; "" -> all-time no upstream). Indisponibilidade ->
	// ErrLeaderboardUnavailable.
	TopForGame(ctx context.Context, gameID, board, window string, limit, offset int) ([]LeaderboardEntry, error)
	// SetScore substitui o score do jogador no board independente do valor atual
	// (admin override). Indisponibilidade -> ErrLeaderboardUnavailable.
	SetScore(ctx context.Context, gameID, playerID, board string, score int64) (LeaderboardEntry, error)
}

// AchievementProgressEntry é uma linha do relatório de progresso de uma conquista:
// todos os jogadores com seu progresso/desbloqueio naquela conquista.
// UnlockedAt é ponteiro pelo mesmo motivo de PlayerAchievement.UnlockedAt.
type AchievementProgressEntry struct {
	PlayerID   string     `json:"playerId"`
	Progress   int        `json:"progress"`
	Unlocked   bool       `json:"unlocked"`
	UnlockedAt *time.Time `json:"unlockedAt,omitempty"`
}

// AchievementsReader é a Port de saída para a API admin do achievements-service,
// agora por jogo (multi-tenant, ADR-0017).
type AchievementsReader interface {
	// CatalogForGame lê o catálogo de conquistas do jogo.
	CatalogForGame(ctx context.Context, gameID string) ([]Achievement, error)
	// CreateAchievement cadastra uma conquista no jogo. Validação -> UpstreamError.
	CreateAchievement(ctx context.Context, gameID string, a Achievement) error
	// UpdateAchievement edita uma conquista do jogo. 400/404 -> UpstreamError.
	UpdateAchievement(ctx context.Context, gameID, id string, a Achievement) error
	// DeleteAchievement remove uma conquista do jogo. 404 -> UpstreamError.
	DeleteAchievement(ctx context.Context, gameID, id string) error
	// PlayerAchievements lê os achievements de um jogador no jogo.
	PlayerAchievements(ctx context.Context, gameID, playerID string) ([]PlayerAchievement, error)
	// ImportAchievements cadastra conquistas em lote (spec 011). Processa cada
	// item sequencialmente e retorna o resultado por linha — nunca falha no total.
	ImportAchievements(ctx context.Context, gameID string, items []Achievement) (ImportResponse, error)
	// AchievementProgress devolve o progresso de todos os jogadores na conquista.
	AchievementProgress(ctx context.Context, gameID, achievementID string) ([]AchievementProgressEntry, error)
	// GameEventRules devolve as regras EFETIVAS do jogo: globais de cada
	// arquétipo (Scope="global") + overrides específicos do jogo (Scope="game").
	// Migration 008 do achievements-service (mapeamento evento->achievement
	// data-driven, substitui o vocabulário hardcoded do front).
	GameEventRules(ctx context.Context, gameID string) ([]EventRule, error)
	// CreateEventRule cria uma regra escopada ao jogo. Validação -> UpstreamError.
	CreateEventRule(ctx context.Context, gameID string, r EventRule) (*EventRule, error)
	// UpdateEventRule substitui match/eventTypes de uma regra do jogo (spec
	// 016 do achievements-service) — archetype não é editável. 400/404 ->
	// UpstreamError.
	UpdateEventRule(ctx context.Context, gameID, id string, r EventRule) (*EventRule, error)
	// DeleteEventRule remove uma regra do jogo (nunca uma global — o
	// achievements-service trata como 404 nos dois casos). 404 -> UpstreamError.
	DeleteEventRule(ctx context.Context, gameID, id string) error
	// SuppressEventRule desliga uma regra GLOBAL para o jogo gameID (spec 017
	// do achievements-service, V2b do ADR-0028) — override de jogo não é
	// suprimível (404 -> UpstreamError, mesma resposta de "não existe").
	SuppressEventRule(ctx context.Context, gameID, id string) error
	// UnsuppressEventRule restaura uma regra global suprimida. Sem supressão
	// ativa -> 404 -> UpstreamError.
	UnsuppressEventRule(ctx context.Context, gameID, id string) error
}

// EventRule é uma regra de mapeamento evento->achievement-event-type
// (migration 008 do achievements-service). Scope "global" = regra do
// arquétipo (read-only neste painel); "game" = override específico deste
// jogo (editável/removível).
type EventRule struct {
	ID        string `json:"id"`
	Scope     string `json:"scope"`
	Archetype string `json:"archetype"`
	// Match é opaco (string OU objeto de comparador, spec achievements/014) —
	// o BFF repassa sem interpretar, como o PlayerDataEntry.Value abaixo.
	Match      map[string]json.RawMessage `json:"match"`
	EventTypes []string                   `json:"eventTypes"`
	// Suppressed (spec 017, V2b): sempre presente; só relevante pra Scope
	// "global" — repassado como veio do achievements-service.
	Suppressed bool `json:"suppressed"`
}

// PlayerDataEntry é uma linha de dado de gameplay do jogador (player-data). O
// Value é JSON opaco (RawMessage) — o BFF não interpreta o conteúdo.
type PlayerDataEntry struct {
	Namespace string          `json:"namespace"`
	Key       string          `json:"key"`
	Value     json.RawMessage `json:"value"`
}

// PlayerDataReader é a Port de saída para a API admin do player-data-service
// (dado de gameplay por jogo+jogador, ADR-0017).
type PlayerDataReader interface {
	// ForPlayer lê todo o dado de um jogador num jogo.
	// Indisponibilidade -> ErrPlayerDataUnavailable.
	ForPlayer(ctx context.Context, gameID, playerID string) ([]PlayerDataEntry, error)
}

// CloudSave é o save de um jogador num jogo (cloud-save): metadados + conteúdo
// em base64 (o blob é binário; o BFF só repassa).
type CloudSave struct {
	ContentType string    `json:"contentType"`
	Size        int       `json:"size"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Blob        string    `json:"blob"` // base64 (std)
}

// CloudSaveReader é a Port de saída para a API admin do cloud-save-service
// (save por jogo+jogador, ADR-0017).
type CloudSaveReader interface {
	// ForPlayer lê o save de um jogador num jogo. Ausência (404) vira
	// UpstreamError(404) (repassado); indisponibilidade -> ErrCloudSaveUnavailable.
	ForPlayer(ctx context.Context, gameID, playerID string) (CloudSave, error)
}

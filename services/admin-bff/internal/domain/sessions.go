package domain

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// ErrSessionsUnavailable indica que o session-service está inacessível. O
// transport mapeia -> 502.
var ErrSessionsUnavailable = errors.New("admin: sessions source unavailable")

// ErrHistoryUnavailable indica que o store de histórico persistido do session-
// service está fora (Postgres indisponível, spec 025/plan D6). Distinto de
// ErrSessionsUnavailable (session-service inteiro inacessível): aqui o
// service está de pé, só o histórico que não responde. O transport mapeia ->
// 503 history_unavailable.
var ErrHistoryUnavailable = errors.New("admin: session history unavailable")

// SessionPlayer é um jogador da sessão com o handle resolvido no identity
// (best-effort: identity fora -> handle vazio, o id sempre presente).
type SessionPlayer struct {
	ID     string `json:"id"`
	Handle string `json:"handle"`
}

// SessionParticipant é um participante da sessão com o papel ocupado (spec
// 025) — a base do histórico persistido. Handle resolvido no identity, mesmo
// best-effort de SessionPlayer.
type SessionParticipant struct {
	PlayerID string    `json:"playerId"`
	Handle   string    `json:"handle"`
	Role     string    `json:"role,omitempty"`
	JoinedAt time.Time `json:"joinedAt"`
	// LastEvent/LastOutcome (spec 027 do session-service): último evento de
	// participação conhecido — só existem em host_world_dropin. Passthrough
	// 1:1, sem transformação.
	LastEvent   string `json:"lastEvent,omitempty"`
	LastOutcome string `json:"lastOutcome,omitempty"`
}

// GameSession é uma partida (aberta ou fechada) para a aba Sessions e o board
// global Rooms. GameID/GameName ficam vazios na aba por-jogo (já se sabe o jogo)
// e preenchidos no board global (spec 008).
type GameSession struct {
	SessionID     string          `json:"sessionId"`
	MatchID       string          `json:"matchId"`
	GameID        string          `json:"gameId,omitempty"`
	ModeID        string          `json:"modeId,omitempty"` // modo do jogo (ADR-0024, aditivo)
	GameName      string          `json:"gameName,omitempty"`
	State         string          `json:"state"`
	Players       []SessionPlayer `json:"players"`
	ServerAddress string          `json:"serverAddress"`
	OpenedAt      time.Time       `json:"openedAt"`
	ClosedAt      *time.Time      `json:"closedAt,omitempty"`
	CloseReason   string          `json:"closeReason,omitempty"`

	// host_world_dropin (spec 014): origem, host e slots por papel (co-op X/2,
	// invasor Y/1). origin sempre presente; os demais só em mundos host_world.
	Origin       string     `json:"origin,omitempty"`
	HostPlayerID string     `json:"hostPlayerId,omitempty"`
	RoleSlots    []RoleSlot `json:"roleSlots,omitempty"`

	// Detalhe rico (session spec 018): snapshot do modo (params crus — o front
	// lê as chaves que conhece), formação por time e resultado persistido.
	// Passthrough 1:1 do session-service; handles do resultado se resolvem no
	// front via Players.
	Archetype   string          `json:"archetype,omitempty"`
	Topology    string          `json:"topology,omitempty"` // dedicated | p2p (session/031)
	ProfileHash string          `json:"profileHash,omitempty"`
	ModeParams  json.RawMessage `json:"modeParams,omitempty"`
	Teams       []SessionTeam   `json:"teams,omitempty"`
	Result      *MatchResult    `json:"result,omitempty"`

	// Participants é a base do histórico persistido (spec 025): um registro por
	// jogador com o papel ocupado. Presente tanto nas sessões em memória quanto
	// nas lidas do store (ClosedSessionsByGame/PlayerSessions).
	Participants []SessionParticipant `json:"participants,omitempty"`
}

// SessionTeam é um lado da formação registrada (rótulo canônico + jogadores).
type SessionTeam struct {
	TeamID  string   `json:"teamId"`
	Players []string `json:"players"`
}

// MatchResult espelha o outcome polimórfico do session (tagged union): só o
// kind reportado tem payload.
type MatchResult struct {
	Kind        string            `json:"kind"`
	WinLossDraw []PlayerOutcome   `json:"winLossDraw,omitempty"`
	Placement   []PlacementEntry  `json:"placement,omitempty"`
	Collective  *CollectiveResult `json:"collective,omitempty"`
}

// PlayerOutcome é o desfecho de um jogador (queue_arena, win_loss_draw).
type PlayerOutcome struct {
	PlayerID string           `json:"playerId"`
	TeamID   string           `json:"teamId,omitempty"`
	Outcome  string           `json:"outcome"`
	Score    int64            `json:"score"`
	Stats    map[string]int64 `json:"stats,omitempty"`
}

// PlacementEntry é a colocação de um participante (battle_royale, placement).
type PlacementEntry struct {
	PlayerID   string           `json:"playerId"`
	TeamID     string           `json:"teamId,omitempty"`
	Placement  int              `json:"placement"`
	SurvivedMs int64            `json:"survivedMs"`
	Stats      map[string]int64 `json:"stats,omitempty"`
}

// CollectiveResult é o desfecho coletivo (coop_mission, collective).
type CollectiveResult struct {
	Outcome string             `json:"outcome"`
	Players []CollectivePlayer `json:"players,omitempty"`
}

// CollectivePlayer é a participação individual num desfecho coletivo.
type CollectivePlayer struct {
	PlayerID string           `json:"playerId"`
	TeamID   string           `json:"teamId,omitempty"`
	Stats    map[string]int64 `json:"stats,omitempty"`
}

// RoleSlot é a ocupação de um papel num mundo host_world (spec 014). Occupants
// (session spec 019) lista quem está em cada papel — o console renderiza os
// cards por papel com os jogadores atribuídos.
type RoleSlot struct {
	Role      string   `json:"role"`
	Capacity  int      `json:"capacity"`
	Occupied  int      `json:"occupied"`
	Occupants []string `json:"occupants,omitempty"`
}

// GameSessions agrupa as sessões do jogo como o painel consome.
type GameSessions struct {
	Active []GameSession `json:"active"`
	Closed []GameSession `json:"closed"`
}

// ClosedSessionsPage é uma página de sessões fechadas lidas do store
// persistido (spec 025): NextBefore é o cursor da próxima página (nil = não
// há mais páginas).
type ClosedSessionsPage struct {
	Closed     []GameSession `json:"closed"`
	NextBefore *time.Time    `json:"nextBefore,omitempty"`
}

// SessionsReader é a Port de saída da API admin do session-service.
type SessionsReader interface {
	// SessionsByGame lê as sessões do jogo; state ("open"|"closed"|"") é
	// repassado como filtro. Indisponibilidade -> ErrSessionsUnavailable.
	SessionsByGame(ctx context.Context, gameID, state string) (GameSessions, error)
	// AllSessions lê as sessões de todos os jogos (board global do console, 008).
	AllSessions(ctx context.Context, state string) (GameSessions, error)
	// CloseSession encerra a sessão ativa (force-close do operador). 404 do
	// session -> UpstreamError{Status:404}.
	CloseSession(ctx context.Context, matchID string) (GameSession, error)
	// FleetStatus lê o status real de uma Fleet Agones (spec 023): onde está
	// rodando (cluster) e réplicas desired/current/ready/allocated.
	// Indisponibilidade -> ErrSessionsUnavailable.
	FleetStatus(ctx context.Context, fleetName string) (FleetStatus, error)
	// ClosedSessionsByGame lê as sessões fechadas PERSISTIDAS do jogo, paginadas
	// por closed_at (spec 025) — sobrevive a restart do session-service, ao
	// contrário do ring in-memory usado por SessionsByGame. Postgres fora ->
	// ErrHistoryUnavailable.
	ClosedSessionsByGame(ctx context.Context, gameID string, limit int, before time.Time) (ClosedSessionsPage, error)
	// PlayerSessions lê as sessões fechadas do jogo em que o player participou
	// (spec 025, aba History do player), mesma paginação. Postgres fora ->
	// ErrHistoryUnavailable.
	PlayerSessions(ctx context.Context, playerID, gameID string, limit int, before time.Time) (ClosedSessionsPage, error)
}

// FleetStatus é o snapshot de localização/saúde de uma Fleet Agones (spec
// 023), como devolvido pelo session-service. Cluster="" cobre tanto "sem
// fleet cadastrada" (o service nem chama o session-service) quanto
// "session-service sem allocator agones ativo" — distinto de Cluster
// preenchido com FleetExists=false ("Agones ativo, mas a Fleet não está
// NESTE cluster").
type FleetStatus struct {
	Cluster     string `json:"cluster"`
	FleetExists bool   `json:"fleetExists"`
	Desired     int    `json:"desired"`
	Current     int    `json:"current"`
	Ready       int    `json:"ready"`
	Allocated   int    `json:"allocated"`
}

// FanoutHealth são os sinais do fan-out durável (ADR-0020) via PromQL. Ponteiros:
// nil = métrica ausente/Prometheus fora (não confundir com zero), como LiveMetrics.
type FanoutHealth struct {
	PublishOK    *float64 `json:"publishOk"`
	PublishError *float64 `json:"publishError"`
	ConsumeOK    *float64 `json:"consumeOk"`
	ConsumeDup   *float64 `json:"consumeDup"`
	ConsumeError *float64 `json:"consumeError"`
}

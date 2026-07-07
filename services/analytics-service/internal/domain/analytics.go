// Package domain contém os tipos puros e as Ports do analytics-service. Zero
// dependências externas (ADR-0002). Tudo escopado por gameID (ADR-0017).
package domain

import "context"

// Vocabulário do desfecho (ADR-0023, arquétipo queue_arena).
const (
	OutcomeWin  = "win"
	OutcomeLoss = "loss"
	OutcomeDraw = "draw"
)

// PlayerResult é o resultado de um jogador numa partida (do fan-out match-results).
// Outcome é o desfecho polimórfico (ADR-0023); Won permanece por compat/fallback.
type PlayerResult struct {
	PlayerID string
	Outcome  string // "win" | "loss" | "draw"
	Won      bool
	Score    int64
}

// MatchResult é o resultado de uma partida ingerido do fan-out (ADR-0020/0021).
// É a unidade de trabalho do worker pool.
type MatchResult struct {
	MatchID string
	GameID  string
	Results []PlayerResult
}

// GameStat é o rollup de um jogo (avgScore derivado no read).
type GameStat struct {
	GameID         string
	Matches        int64
	UniquePlayers  int64
	Participations int64
	Wins           int64
	Losses         int64
	Draws          int64
	AvgScore       float64
}

// Totals são os totais cross-game do overview.
type Totals struct {
	Matches       int64
	UniquePlayers int64 // distintos globais
	Games         int64 // jogos com atividade
}

// Overview é o panorama cross-game (aba Analytics): totais, jogos e a série de
// partidas por dia somada entre todos os jogos (spec 002).
type Overview struct {
	Totals Totals
	Games  []GameStat
	Daily  []DailyPoint
}

// DailyPoint é uma partida-por-dia da série de um jogo.
type DailyPoint struct {
	Day     string // YYYY-MM-DD (UTC)
	Matches int64
}

// GameAnalytics é o detalhe de um jogo (resumo + série diária + participações
// por papel, host_world_dropin — spec 004).
type GameAnalytics struct {
	GameStat
	Daily []DailyPoint
	// RoleParticipations é a quebra de eventos de participação por papel/evento
	// (host_world_dropin). Vazia no queue_arena. Aditivo (spec 004). Nome distinto
	// do GameStat.Participations (contagem do queue_arena).
	RoleParticipations []ParticipationStat
}

// ParticipationEvent é um evento de participação ingerido do fan-out
// (kind=participation_event, session/013). Unidade de trabalho do caminho de
// participação do pool.
type ParticipationEvent struct {
	GameID    string
	SessionID string
	PlayerID  string
	Role      string
	Event     string
	Outcome   string
}

// ParticipationStat é a contagem agregada de um (papel, evento) num jogo.
type ParticipationStat struct {
	Role  string
	Event string
	Count int64
}

// MatchRepository é a Port de saída de persistência (rollups em Postgres).
type MatchRepository interface {
	// ApplyMatch agrega uma partida de forma idempotente por matchID (numa tx).
	// dup=true se a partida já havia sido processada (no-op).
	ApplyMatch(ctx context.Context, m MatchResult) (dup bool, err error)
	// ApplyParticipation agrega um evento de participação (host_world_dropin) por
	// (gameID, role, event), idempotente por (sessionID, playerID, event) numa tx.
	// dup=true se o evento já havia sido processado (no-op). Spec 004.
	ApplyParticipation(ctx context.Context, evt ParticipationEvent) (dup bool, err error)
	// Overview devolve os totais cross-game + os jogos ordenados por matches desc.
	Overview(ctx context.Context) (Overview, error)
	// GameAnalytics devolve o resumo do jogo + a série diária dos últimos `days`.
	GameAnalytics(ctx context.Context, gameID string, days int) (GameAnalytics, error)
}

// Ingestor é a Port de entrada usada pelo consumidor (driving adapter): entrega
// uma partida ao worker pool e devolve um channel com o resultado (para o
// consumidor deletar a mensagem só no sucesso).
type Ingestor interface {
	Submit(ctx context.Context, m MatchResult) <-chan error
	// SubmitParticipation entrega um evento de participação ao pool (spec 004),
	// pelo mesmo mecanismo de fan-in (channel de retorno) do Submit.
	SubmitParticipation(ctx context.Context, evt ParticipationEvent) <-chan error
}

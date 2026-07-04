// Package domain contém os tipos puros e as Ports do analytics-service. Zero
// dependências externas (ADR-0002). Tudo escopado por gameID (ADR-0017).
package domain

import "context"

// PlayerResult é o resultado de um jogador numa partida (do fan-out match-results).
type PlayerResult struct {
	PlayerID string
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
	AvgScore       float64
}

// Totals são os totais cross-game do overview.
type Totals struct {
	Matches       int64
	UniquePlayers int64 // distintos globais
	Games         int64 // jogos com atividade
}

// Overview é o panorama cross-game (aba Analytics).
type Overview struct {
	Totals Totals
	Games  []GameStat
}

// DailyPoint é uma partida-por-dia da série de um jogo.
type DailyPoint struct {
	Day     string // YYYY-MM-DD (UTC)
	Matches int64
}

// GameAnalytics é o detalhe de um jogo (resumo + série diária).
type GameAnalytics struct {
	GameStat
	Daily []DailyPoint
}

// MatchRepository é a Port de saída de persistência (rollups em Postgres).
type MatchRepository interface {
	// ApplyMatch agrega uma partida de forma idempotente por matchID (numa tx).
	// dup=true se a partida já havia sido processada (no-op).
	ApplyMatch(ctx context.Context, m MatchResult) (dup bool, err error)
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
}

// Package domain contém os tipos de negócio puros e as interfaces (Ports)
// do leaderboard-service. Zero dependências externas. Ver ADR-0002.
package domain

import (
	"context"
	"regexp"
)

// Board é a métrica de um ranking (ADR-0023). Cada jogo tem N rankings, um por
// board. Dois modos de agregação convivem:
//   - highscore (ZADD GT): "score" (default) + um por stat (melhor de uma
//     partida) — fatia 003.
//   - cumulative (ZINCRBY): "wins" (nº de vitórias) + "<stat>_total" (soma do
//     stat ao longo das partidas) — fatia 004 (A2).
const (
	BoardScore = "score"
	BoardWins  = "wins"

	// battle_royale (BR3, ADR-0023): cumulativo "terminou no top N" e highscore da
	// sobrevivência (melhor de uma partida). BoardWins é reusado para o 1º lugar.
	BoardTopNFinishes = "top_n_finishes"
	BoardSurvivedMs   = "survived_ms"

	// coop_mission (C3, ADR-0023): cumulativo "missões cumpridas" (+1 por
	// MISSION_SUCCESS, igual para todo o time — o desfecho é coletivo).
	BoardMissionsCompleted = "missions_completed"
)

// BattleRoyaleTopNThreshold é o limiar do "top N" (política do sink, consistente
// com os demais sinks da BR3): colocação <= 10 conta como top_n_finishes.
const BattleRoyaleTopNThreshold = 10

// TotalBoard é o board cumulativo de um stat (ex.: "kills" -> "kills_total").
func TotalBoard(stat string) string { return stat + "_total" }

// Janelas de tempo de um ranking (spec 007): derivadas do relógio UTC no
// momento do write — zero configuração. All-time é a janela default (e a única
// que existia até a spec 007); as periódicas respondem "quem foi melhor
// hoje/nesta semana/neste mês". Seasons nomeadas (rollover manual) ficam
// adiadas por evidência.
const (
	WindowAllTime = "alltime"
	WindowDaily   = "daily"
	WindowWeekly  = "weekly"
	WindowMonthly = "monthly"
)

// Windows lista todas as janelas (writes espalham em todas; leituras escolhem uma).
var Windows = []string{WindowAllTime, WindowDaily, WindowWeekly, WindowMonthly}

// ValidWindow diz se o nome de janela é aceitável.
func ValidWindow(window string) bool {
	switch window {
	case WindowAllTime, WindowDaily, WindowWeekly, WindowMonthly:
		return true
	}
	return false
}

// boardRE valida o nome de um board — mesmo identificador estável do scoringStats
// no game-registry (`^[a-z][a-z0-9_]*$`).
var boardRE = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// ValidBoard diz se o nome de board é aceitável.
func ValidBoard(board string) bool { return boardRE.MatchString(board) }

// Entry é uma posição no ranking. Rank é 1-based (para exibição).
type Entry struct {
	Rank     int64
	PlayerID string
	Score    int64
}

// Increment é uma parcela cumulativa a somar num board (ZINCRBY). Usado no
// fan-out para totalizar vitórias e stats por jogador (ADR-0023 A2).
type Increment struct {
	Board    string
	PlayerID string
	Delta    int64
}

// LeaderboardService é a Port de entrada do leaderboard-service. Tudo é escopado
// por (gameID, board) — ranking por jogo (ADR-0017) e por métrica (ADR-0023).
type LeaderboardService interface {
	// Submit aplica highscore e retorna a entry do jogador após a submissão
	// (rank do all-time — contrato pré-janelas preservado). O write espalha em
	// todas as janelas (spec 007).
	Submit(ctx context.Context, gameID, board, playerID string, score int64) (Entry, error)
	// ForceSubmit substitui o score independente do valor atual (admin override).
	ForceSubmit(ctx context.Context, gameID, board, playerID string, score int64) (Entry, error)
	// Top devolve a página [offset, offset+limit) da janela corrente pedida
	// (rank 1-based contínuo). ErrInvalidWindow se a janela não existir.
	Top(ctx context.Context, gameID, board, window string, limit, offset int) ([]Entry, error)
	Around(ctx context.Context, gameID, board, window, playerID string, k int) ([]Entry, error)
	// ApplyCumulative soma as parcelas nos boards cumulativos de uma partida, de
	// forma atômica e idempotente por matchID (ZINCRBY não é idempotente de graça,
	// diferente do ZADD GT). dup=true se a partida já havia sido aplicada (no-op).
	ApplyCumulative(ctx context.Context, gameID, matchID string, incrs []Increment) (dup bool, err error)
}

// LeaderboardStore é a Port de saída (implementada por um adapter Redis). Cada
// (jogo, board, janela) é um sorted set próprio; writes espalham em todas as
// janelas (spec 007), leituras apontam para a janela corrente pedida.
type LeaderboardStore interface {
	Submit(ctx context.Context, gameID, board, playerID string, score int64) error // highscore (ZADD GT, todas as janelas)
	// ForceScore substitui o score independente do valor atual (ZADD sem GT).
	ForceScore(ctx context.Context, gameID, board, playerID string, score int64) error
	// Rank devolve a posição (0-based, ordem do Redis) e o score do jogador na
	// janela corrente; ErrNotRanked se ele ainda não tem score nela.
	Rank(ctx context.Context, gameID, board, window, playerID string) (rank int64, score int64, err error)
	Top(ctx context.Context, gameID, board, window string, limit int) ([]Entry, error)
	// Range devolve as entradas das posições [start, stop] (0-based, inclusivo).
	Range(ctx context.Context, gameID, board, window string, start, stop int64) ([]Entry, error)
	// ApplyCumulative aplica os ZINCRBY de uma partida (em todas as janelas) sob
	// uma guarda de idempotência ÚNICA por matchID, tudo atômico (script Lua):
	// dup=true = já aplicada (no-op em todas as janelas).
	ApplyCumulative(ctx context.Context, gameID, matchID string, incrs []Increment) (dup bool, err error)
}

// TokenVerifier valida o JWT do identity e devolve o playerID (sub).
// Implementada por shared/auth (JWKS).
type TokenVerifier interface {
	Verify(ctx context.Context, token string) (playerID string, err error)
}

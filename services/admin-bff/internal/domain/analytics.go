package domain

import (
	"context"
	"errors"
)

// ErrAnalyticsUnavailable indica que o analytics-service está inacessível. O
// transport mapeia -> 502.
var ErrAnalyticsUnavailable = errors.New("admin: analytics source unavailable")

// AnalyticsGameStat é o rollup de um jogo no overview (gameName injetado pelo BFF).
type AnalyticsGameStat struct {
	GameID        string  `json:"gameId"`
	GameName      string  `json:"gameName"`
	Matches       int64   `json:"matches"`
	UniquePlayers int64   `json:"uniquePlayers"`
	Wins          int64   `json:"wins"`
	Losses        int64   `json:"losses"`
	Draws         int64   `json:"draws"`
	AvgScore      float64 `json:"avgScore"`
}

// AnalyticsTotals são os totais cross-game.
type AnalyticsTotals struct {
	Matches       int64 `json:"matches"`
	UniquePlayers int64 `json:"uniquePlayers"`
	Games         int64 `json:"games"`
}

// AnalyticsDaily é um ponto da série "partidas por dia" (cross-game).
type AnalyticsDaily struct {
	Day     string `json:"day"`
	Matches int64  `json:"matches"`
}

// AnalyticsOverview é o panorama cross-game da aba Analytics (ADR-0021).
type AnalyticsOverview struct {
	Totals AnalyticsTotals     `json:"totals"`
	Games  []AnalyticsGameStat `json:"games"`
	Daily  []AnalyticsDaily    `json:"daily"`
}

// ParticipationStat é a contagem de um (papel, evento) no detalhe de um jogo
// host_world_dropin (spec 014/004). Vazio no queue_arena.
type ParticipationStat struct {
	Role  string `json:"role"`
	Event string `json:"event"`
	Count int64  `json:"count"`
}

// GameAnalyticsDetail é o detalhe de analytics de um jogo (resumo + série diária
// + participações por papel + distribuição de colocação). gameName é injetado
// pelo BFF (spec 014).
type GameAnalyticsDetail struct {
	GameID             string                 `json:"gameId"`
	GameName           string                 `json:"gameName"`
	Matches            int64                  `json:"matches"`
	UniquePlayers      int64                  `json:"uniquePlayers"`
	Wins               int64                  `json:"wins"`
	Losses             int64                  `json:"losses"`
	Draws              int64                  `json:"draws"`
	AvgScore           float64                `json:"avgScore"`
	Daily              []AnalyticsDaily       `json:"daily"`
	RoleParticipations []ParticipationStat    `json:"roleParticipations"`
	Placement          *PlacementDistribution `json:"placement,omitempty"` // battle_royale (BR4); nil fora do BR
	Mission            *MissionDistribution   `json:"mission,omitempty"`   // coop_mission (C4); nil fora do coop
}

// PlacementDistribution é a distribuição de colocação do battle_royale (BR3/BR4):
// histograma 1º/top-10/resto + sobrevivência média. Passthrough do analytics.
type PlacementDistribution struct {
	Winners       int64   `json:"winners"`
	TopN          int64   `json:"topN"`
	Rest          int64   `json:"rest"`
	Entries       int64   `json:"entries"`
	AvgSurvivedMs float64 `json:"avgSurvivedMs"`
}

// MissionDistribution é a distribuição de desfecho da missão do coop_mission
// (C3/C4): sucessos/falhas + taxa de sucesso. Passthrough do analytics.
type MissionDistribution struct {
	Successes   int64   `json:"successes"`
	Failures    int64   `json:"failures"`
	SuccessRate float64 `json:"successRate"`
}

// AnalyticsReader é a Port de saída para a read API do analytics-service.
type AnalyticsReader interface {
	Overview(ctx context.Context) (AnalyticsOverview, error)
	// GameAnalytics lê o detalhe por jogo (GET /admin/games/{id}/analytics),
	// incluindo roleParticipations (host_world_dropin). Fora -> ErrAnalyticsUnavailable.
	GameAnalytics(ctx context.Context, gameID string, days int) (GameAnalyticsDetail, error)
}

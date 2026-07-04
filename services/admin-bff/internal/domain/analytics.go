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

// AnalyticsReader é a Port de saída para a read API do analytics-service.
type AnalyticsReader interface {
	Overview(ctx context.Context) (AnalyticsOverview, error)
}

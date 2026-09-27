package domain

import (
	"context"
	"errors"
)

// ErrAnalyticsUnavailable indica que o analytics-service está inacessível. O
// transport mapeia -> 502.
var ErrAnalyticsUnavailable = errors.New("admin: analytics source unavailable")

// AnalyticsGameStat é o rollup de um jogo no overview (gameName injetado pelo BFF).
// Sessions/AvgSessionMs/Abandoned (analytics/016) são a medida comum a todos os
// arquétipos; Matches/Wins/... são do queue_arena.
type AnalyticsGameStat struct {
	GameID        string  `json:"gameId"`
	GameName      string  `json:"gameName"`
	Sessions      int64   `json:"sessions"`
	AvgSessionMs  int64   `json:"avgSessionMs"`
	Abandoned     int64   `json:"abandoned"`
	Matches       int64   `json:"matches"`
	UniquePlayers int64   `json:"uniquePlayers"`
	Wins          int64   `json:"wins"`
	Losses        int64   `json:"losses"`
	Draws         int64   `json:"draws"`
	AvgScore      float64 `json:"avgScore"`
}

// AnalyticsTotals são os totais cross-game.
type AnalyticsTotals struct {
	Sessions      int64 `json:"sessions"`
	Matches       int64 `json:"matches"`
	UniquePlayers int64 `json:"uniquePlayers"`
	Games         int64 `json:"games"`
}

// AnalyticsDaily é um ponto de série diária (por jogo: partidas/eventos por dia).
type AnalyticsDaily struct {
	Day     string `json:"day"`
	Matches int64  `json:"matches"`
}

// AnalyticsSessionsPoint é um ponto da série "sessões fechadas por dia" do
// overview cross-game (analytics/016 — antes "partidas por dia", só queue_arena).
type AnalyticsSessionsPoint struct {
	Day      string `json:"day"`
	Sessions int64  `json:"sessions"`
}

// AnalyticsOverview é o panorama cross-game da aba Analytics (ADR-0021).
type AnalyticsOverview struct {
	Totals AnalyticsTotals          `json:"totals"`
	Games  []AnalyticsGameStat      `json:"games"`
	Daily  []AnalyticsSessionsPoint `json:"daily"`
}

// ParticipationStat é a contagem de um (modo, papel, evento, outcome) no
// detalhe de um jogo host_world_dropin (spec 014/004/007/009). Vazio no
// queue_arena. ModeID vazio agrupa dados anteriores à spec 009 (sem modo).
type ParticipationStat struct {
	ModeID  string `json:"modeId"`
	Role    string `json:"role"`
	Event   string `json:"event"`
	Outcome string `json:"outcome"`
	Count   int64  `json:"count"`
}

// GameAnalyticsDetail é o detalhe de analytics de um jogo (resumo + série diária
// + participações por papel + distribuição de colocação). gameName é injetado
// pelo BFF (spec 014).
type GameAnalyticsDetail struct {
	GameID   string `json:"gameId"`
	GameName string `json:"gameName"`
	// ModeID é o filtro efetivamente aplicado (analytics-service/012); "" =
	// todos os modos. Eco do parâmetro de entrada.
	ModeID             string                 `json:"modeId,omitempty"`
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

	// Séries diárias dos 3 arquétipos que não tinham (analytics-service/010)
	// — vazias fora do arquétipo correspondente, mesmo padrão de Placement/Mission.
	PlacementDaily     []AnalyticsDaily `json:"placementDaily"`
	MissionDaily       []AnalyticsDaily `json:"missionDaily"`
	ParticipationDaily []AnalyticsDaily `json:"participationDaily"`

	// Stats é o resumo agregado por stat_key (analytics-service/011) — vazio
	// se o jogo nunca reportou stats (sem scoringStats no perfil).
	Stats []StatSummary `json:"stats"`

	// SessionHealth é a distribuição por motivo de fechamento
	// (analytics-service/013) — vazio se o jogo não tem nenhum
	// session_closed processado ainda.
	SessionHealth []SessionCloseStat `json:"sessionHealth"`

	// SessionCloseDaily é a série "sessões fechadas por dia"
	// (analytics-service/015), mesmo padrão das outras séries diárias.
	SessionCloseDaily []AnalyticsDaily `json:"sessionCloseDaily"`
}

// SessionCloseStat é a distribuição agregada por motivo de fechamento
// (analytics-service/013). Passthrough do analytics.
type SessionCloseStat struct {
	ModeID        string  `json:"modeId,omitempty"`
	Archetype     string  `json:"archetype,omitempty"`
	CloseReason   string  `json:"closeReason"`
	Count         int64   `json:"count"`
	DurationSumMs int64   `json:"durationSumMs"`
	AvgDurationMs float64 `json:"avgDurationMs"`
}

// StatSummary é o panorama agregado de uma métrica do jogo
// (analytics-service/011): soma, contagem de ocorrências e média derivada.
// Passthrough do analytics.
type StatSummary struct {
	Key   string  `json:"key"`
	Sum   int64   `json:"sum"`
	Count int64   `json:"count"`
	Avg   float64 `json:"avg"`
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
	// modeID == "" agrega todos os modos (analytics-service/012).
	GameAnalytics(ctx context.Context, gameID string, days int, modeID string) (GameAnalyticsDetail, error)
}

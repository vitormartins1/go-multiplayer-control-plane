// Superfície HTTP admin do analytics-service (ADR-0021): overview cross-game +
// detalhe por jogo. /admin sem auth no serviço — internal key na borda (ADR-0018).
package transport

import (
	"context"
	"net/http"
	"strconv"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/analytics-service/internal/domain"
	"github.com/vitormartins1/go-multiplayer-control-plane/shared/httpkit"
)

const defaultDailyDays = 30

// Reader é o recorte de leitura consumido pelo admin (satisfeito pelo repositório).
type Reader interface {
	Overview(ctx context.Context) (domain.Overview, error)
	GameAnalytics(ctx context.Context, gameID string, days int) (domain.GameAnalytics, error)
}

// AdminHandler expõe as rotas /admin de analytics.
type AdminHandler struct {
	reader Reader
}

// NewAdminHandler cria o handler.
func NewAdminHandler(reader Reader) *AdminHandler {
	return &AdminHandler{reader: reader}
}

// Register monta as rotas no mux (metrics/healthz moram lá).
func (h *AdminHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/analytics/overview", h.overview)
	mux.HandleFunc("GET /admin/games/{gameId}/analytics", h.gameAnalytics)
}

type gameStatDTO struct {
	GameID         string  `json:"gameId"`
	Matches        int64   `json:"matches"`
	UniquePlayers  int64   `json:"uniquePlayers"`
	Participations int64   `json:"participations"`
	Wins           int64   `json:"wins"`
	Losses         int64   `json:"losses"`
	Draws          int64   `json:"draws"`
	AvgScore       float64 `json:"avgScore"`
}

type overviewDTO struct {
	Totals struct {
		Matches       int64 `json:"matches"`
		UniquePlayers int64 `json:"uniquePlayers"`
		Games         int64 `json:"games"`
	} `json:"totals"`
	Games []gameStatDTO `json:"games"`
	Daily []dailyDTO    `json:"daily"`
}

func (h *AdminHandler) overview(w http.ResponseWriter, r *http.Request) {
	ov, err := h.reader.Overview(r.Context())
	if err != nil {
		httpkit.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}
	var dto overviewDTO
	dto.Totals.Matches = ov.Totals.Matches
	dto.Totals.UniquePlayers = ov.Totals.UniquePlayers
	dto.Totals.Games = ov.Totals.Games
	dto.Games = make([]gameStatDTO, 0, len(ov.Games))
	for _, g := range ov.Games {
		dto.Games = append(dto.Games, toGameStatDTO(g))
	}
	dto.Daily = make([]dailyDTO, 0, len(ov.Daily))
	for _, d := range ov.Daily {
		dto.Daily = append(dto.Daily, dailyDTO{Day: d.Day, Matches: d.Matches})
	}
	httpkit.WriteJSON(w, http.StatusOK, dto)
}

type gameAnalyticsDTO struct {
	gameStatDTO
	Daily              []dailyDTO             `json:"daily"`
	RoleParticipations []participationStatDTO `json:"roleParticipations"`
	Placement          *placementDTO          `json:"placement,omitempty"` // battle_royale (spec 005); ausente fora do BR
	Mission            *missionDTO            `json:"mission,omitempty"`   // coop_mission (spec 006); ausente fora do coop

	// Séries diárias dos 3 arquétipos que não tinham (spec 010) — vazias fora
	// do arquétipo correspondente, mesmo padrão de Placement/Mission.
	PlacementDaily     []dailyDTO `json:"placementDaily"`
	MissionDaily       []dailyDTO `json:"missionDaily"`
	ParticipationDaily []dailyDTO `json:"participationDaily"`

	// Stats é o resumo agregado por stat_key (spec 011) — vazio se o jogo
	// nunca reportou stats (sem scoringStats no perfil).
	Stats []statSummaryDTO `json:"stats"`
}

// statSummaryDTO é o panorama agregado de uma métrica do jogo (spec 011).
type statSummaryDTO struct {
	Key   string  `json:"key"`
	Sum   int64   `json:"sum"`
	Count int64   `json:"count"`
	Avg   float64 `json:"avg"`
}

// placementDTO é a distribuição de colocação do battle_royale (spec 005).
type placementDTO struct {
	Winners       int64   `json:"winners"`
	TopN          int64   `json:"topN"`
	Rest          int64   `json:"rest"`
	Entries       int64   `json:"entries"`
	AvgSurvivedMs float64 `json:"avgSurvivedMs"`
}

// missionDTO é a distribuição de desfecho da missão do coop_mission (spec 006).
type missionDTO struct {
	Successes   int64   `json:"successes"`
	Failures    int64   `json:"failures"`
	SuccessRate float64 `json:"successRate"`
}

type dailyDTO struct {
	Day     string `json:"day"`
	Matches int64  `json:"matches"`
}

// participationStatDTO é a contagem por papel/evento/outcome do host_world_dropin
// (spec 004/007). Vazio no queue_arena. Outcome estava sendo perdido aqui —
// o repositório já lia a coluna (query agrupa por role/event/outcome), só o
// DTO não expunha.
type participationStatDTO struct {
	ModeID  string `json:"modeId"`
	Role    string `json:"role"`
	Event   string `json:"event"`
	Outcome string `json:"outcome"`
	Count   int64  `json:"count"`
}

func (h *AdminHandler) gameAnalytics(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("gameId")
	days := defaultDailyDays
	if q := r.URL.Query().Get("days"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 && n <= 365 {
			days = n
		}
	}
	ga, err := h.reader.GameAnalytics(r.Context(), gameID, days)
	if err != nil {
		httpkit.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}
	dto := gameAnalyticsDTO{
		gameStatDTO: toGameStatDTO(ga.GameStat), Daily: []dailyDTO{}, RoleParticipations: []participationStatDTO{},
		PlacementDaily: []dailyDTO{}, MissionDaily: []dailyDTO{}, ParticipationDaily: []dailyDTO{},
		Stats: []statSummaryDTO{},
	}
	for _, s := range ga.Stats {
		dto.Stats = append(dto.Stats, statSummaryDTO{Key: s.Key, Sum: s.Sum, Count: s.Count, Avg: s.Avg})
	}
	for _, d := range ga.Daily {
		dto.Daily = append(dto.Daily, dailyDTO{Day: d.Day, Matches: d.Matches})
	}
	for _, d := range ga.PlacementDaily {
		dto.PlacementDaily = append(dto.PlacementDaily, dailyDTO{Day: d.Day, Matches: d.Matches})
	}
	for _, d := range ga.MissionDaily {
		dto.MissionDaily = append(dto.MissionDaily, dailyDTO{Day: d.Day, Matches: d.Matches})
	}
	for _, d := range ga.ParticipationDaily {
		dto.ParticipationDaily = append(dto.ParticipationDaily, dailyDTO{Day: d.Day, Matches: d.Matches})
	}
	for _, p := range ga.RoleParticipations {
		dto.RoleParticipations = append(dto.RoleParticipations, participationStatDTO{ModeID: p.ModeID, Role: p.Role, Event: p.Event, Outcome: p.Outcome, Count: p.Count})
	}
	if pd := ga.Placement; pd != nil {
		dto.Placement = &placementDTO{
			Winners: pd.Winners, TopN: pd.TopN, Rest: pd.Rest, Entries: pd.Entries, AvgSurvivedMs: pd.AvgSurvivedMs,
		}
	}
	if md := ga.Mission; md != nil {
		dto.Mission = &missionDTO{Successes: md.Successes, Failures: md.Failures, SuccessRate: md.SuccessRate}
	}
	httpkit.WriteJSON(w, http.StatusOK, dto)
}

func toGameStatDTO(g domain.GameStat) gameStatDTO {
	return gameStatDTO{
		GameID:         g.GameID,
		Matches:        g.Matches,
		UniquePlayers:  g.UniquePlayers,
		Participations: g.Participations,
		Wins:           g.Wins,
		Losses:         g.Losses,
		Draws:          g.Draws,
		AvgScore:       g.AvgScore,
	}
}

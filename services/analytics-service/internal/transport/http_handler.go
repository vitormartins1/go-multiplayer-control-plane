// Superfície HTTP admin do analytics-service (ADR-0021): overview cross-game +
// detalhe por jogo. /admin sem auth no serviço — internal key na borda (ADR-0018).
package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/vitormartins1/go-game-backend/services/analytics-service/internal/domain"
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
	AvgScore       float64 `json:"avgScore"`
}

type overviewDTO struct {
	Totals struct {
		Matches       int64 `json:"matches"`
		UniquePlayers int64 `json:"uniquePlayers"`
		Games         int64 `json:"games"`
	} `json:"totals"`
	Games []gameStatDTO `json:"games"`
}

func (h *AdminHandler) overview(w http.ResponseWriter, r *http.Request) {
	ov, err := h.reader.Overview(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
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
	writeJSON(w, http.StatusOK, dto)
}

type gameAnalyticsDTO struct {
	gameStatDTO
	Daily []dailyDTO `json:"daily"`
}

type dailyDTO struct {
	Day     string `json:"day"`
	Matches int64  `json:"matches"`
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
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	dto := gameAnalyticsDTO{gameStatDTO: toGameStatDTO(ga.GameStat), Daily: []dailyDTO{}}
	for _, d := range ga.Daily {
		dto.Daily = append(dto.Daily, dailyDTO{Day: d.Day, Matches: d.Matches})
	}
	writeJSON(w, http.StatusOK, dto)
}

func toGameStatDTO(g domain.GameStat) gameStatDTO {
	return gameStatDTO{
		GameID:         g.GameID,
		Matches:        g.Matches,
		UniquePlayers:  g.UniquePlayers,
		Participations: g.Participations,
		Wins:           g.Wins,
		AvgScore:       g.AvgScore,
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}

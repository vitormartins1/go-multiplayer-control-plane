// Superfície HTTP admin do session-service (spec 005): a aba Sessions do
// painel lê as partidas de um jogo via BFF. Padrão dos pares (leaderboard):
// /admin sem auth no serviço — internal key na borda (ADR-0018).
package transport

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/vitormartins1/go-game-backend/services/session-service/internal/domain"
)

// SessionDirectory é o recorte do service que o admin consome (interface do
// consumidor — não polui a Port de entrada usada pelo gRPC).
type SessionDirectory interface {
	SessionsByGame(gameID string) (active, closed []domain.Session)
}

// AdminHandler expõe o GET /admin/games/{gameId}/sessions.
type AdminHandler struct {
	sessions SessionDirectory
}

// NewAdminHandler cria o handler admin.
func NewAdminHandler(sessions SessionDirectory) *AdminHandler {
	return &AdminHandler{sessions: sessions}
}

// Register monta a rota no mux existente (JWKS/metrics moram lá).
func (h *AdminHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/games/{gameId}/sessions", h.list)
}

type sessionDTO struct {
	SessionID     string     `json:"sessionId"`
	MatchID       string     `json:"matchId"`
	State         string     `json:"state"`
	Players       []string   `json:"players"`
	ServerAddress string     `json:"serverAddress"`
	OpenedAt      time.Time  `json:"openedAt"`
	ClosedAt      *time.Time `json:"closedAt,omitempty"`
	CloseReason   string     `json:"closeReason,omitempty"`
}

type sessionsResponse struct {
	Active []sessionDTO `json:"active"`
	Closed []sessionDTO `json:"closed"`
}

// list responde as sessões do jogo. `state=open|closed` filtra; sem sessões =
// 200 com listas vazias (jogo pode existir sem partidas — não é 404).
func (h *AdminHandler) list(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("gameId")
	state := r.URL.Query().Get("state")
	if state != "" && state != string(domain.SessionOpen) && state != string(domain.SessionClosed) {
		writeError(w, http.StatusBadRequest, "invalid_state")
		return
	}

	active, closed := h.sessions.SessionsByGame(gameID)
	resp := sessionsResponse{Active: []sessionDTO{}, Closed: []sessionDTO{}}
	if state == "" || state == string(domain.SessionOpen) {
		for _, s := range active {
			resp.Active = append(resp.Active, toSessionDTO(s))
		}
	}
	if state == "" || state == string(domain.SessionClosed) {
		for _, s := range closed {
			resp.Closed = append(resp.Closed, toSessionDTO(s))
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func toSessionDTO(s domain.Session) sessionDTO {
	dto := sessionDTO{
		SessionID:     s.ID,
		MatchID:       s.MatchID,
		State:         string(s.State),
		Players:       s.Players,
		ServerAddress: s.ServerAddress,
		OpenedAt:      s.OpenedAt,
		CloseReason:   string(s.CloseReason),
	}
	if !s.ClosedAt.IsZero() {
		closedAt := s.ClosedAt
		dto.ClosedAt = &closedAt
	}
	return dto
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}

// Superfície HTTP admin do session-service (spec 005): a aba Sessions do
// painel lê as partidas de um jogo via BFF. Padrão dos pares (leaderboard):
// /admin sem auth no serviço — internal key na borda (ADR-0018).
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/vitormartins1/go-game-backend/services/session-service/internal/domain"
)

// SessionDirectory é o recorte do service que o admin consome (interface do
// consumidor — não polui a Port de entrada usada pelo gRPC). Cobre a aba Sessions
// por-jogo (005) e o board global + force-close do console (007).
type SessionDirectory interface {
	SessionsByGame(gameID string) (active, closed []domain.Session)
	AllSessions() (active, closed []domain.Session)
	CloseByAdmin(ctx context.Context, matchID string) (*domain.Session, error)
}

// AdminHandler expõe as rotas /admin de sessões (leitura + encerrar).
type AdminHandler struct {
	sessions SessionDirectory
}

// NewAdminHandler cria o handler admin.
func NewAdminHandler(sessions SessionDirectory) *AdminHandler {
	return &AdminHandler{sessions: sessions}
}

// Register monta as rotas no mux existente (JWKS/metrics moram lá).
func (h *AdminHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/games/{gameId}/sessions", h.list)
	mux.HandleFunc("GET /admin/sessions", h.listAll)
	mux.HandleFunc("POST /admin/sessions/{matchId}/close", h.close)
}

type sessionDTO struct {
	SessionID     string     `json:"sessionId"`
	MatchID       string     `json:"matchId"`
	GameID        string     `json:"gameId"`
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
	if !validState(state) {
		writeError(w, http.StatusBadRequest, "invalid_state")
		return
	}

	active, closed := h.sessions.SessionsByGame(gameID)
	writeJSON(w, http.StatusOK, filterByState(active, closed, state))
}

// listAll responde as sessões de TODOS os jogos (board global do console, 007).
// `state=open|closed` filtra; sem sessões = 200 com listas vazias.
func (h *AdminHandler) listAll(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	if !validState(state) {
		writeError(w, http.StatusBadRequest, "invalid_state")
		return
	}
	active, closed := h.sessions.AllSessions()
	writeJSON(w, http.StatusOK, filterByState(active, closed, state))
}

// close encerra uma sessão ativa por ação do operador (CLOSED/admin, 007).
func (h *AdminHandler) close(w http.ResponseWriter, r *http.Request) {
	matchID := r.PathValue("matchId")
	sess, err := h.sessions.CloseByAdmin(r.Context(), matchID)
	if errors.Is(err, domain.ErrSessionNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "close_failed")
		return
	}
	writeJSON(w, http.StatusOK, toSessionDTO(*sess))
}

func validState(state string) bool {
	return state == "" || state == string(domain.SessionOpen) || state == string(domain.SessionClosed)
}

// filterByState monta a resposta aplicando o filtro de estado (compartilhado
// entre a listagem por-jogo e a global).
func filterByState(active, closed []domain.Session, state string) sessionsResponse {
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
	return resp
}

func toSessionDTO(s domain.Session) sessionDTO {
	dto := sessionDTO{
		SessionID:     s.ID,
		MatchID:       s.MatchID,
		GameID:        s.GameID,
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

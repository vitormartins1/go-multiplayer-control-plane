// Package transport traduz HTTP <-> domínio para o admin-bff. Sem lógica de
// negócio (ADR-0002): mapeia a indisponibilidade de cada fonte para o status
// HTTP do painel correspondente (ADR-0016).
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/c130n/game-backend/services/admin-bff/internal/domain"
)

// AdminService é a Port de entrada consumida pelo handler.
type AdminService interface {
	Topology(ctx context.Context) (domain.Topology, error)
	LiveMetrics(ctx context.Context) (domain.LiveMetrics, error)
	ServiceMetrics(ctx context.Context, id string) (domain.ServiceMetrics, error)
	PlayerAchievements(ctx context.Context, playerID string) ([]domain.PlayerAchievement, error)
}

// Handler expõe a API HTTP do admin-bff (consumida pelo admin-ui).
type Handler struct {
	svc AdminService
}

// NewHandler cria o handler a partir do service.
func NewHandler(svc AdminService) *Handler {
	return &Handler{svc: svc}
}

// Register monta as rotas da API do BFF no mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/topology", h.topology)
	mux.HandleFunc("GET /api/metrics/live", h.liveMetrics)
	mux.HandleFunc("GET /api/services/{id}/metrics", h.serviceMetrics)
	mux.HandleFunc("GET /api/players/{playerID}/achievements", h.playerAchievements)
}

// topology nunca falha por causa do Prometheus: o service degrada (nós unknown).
func (h *Handler) topology(w http.ResponseWriter, r *http.Request) {
	topo, err := h.svc.Topology(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, topo)
}

// liveMetrics depende só do Prometheus: fonte fora -> 503.
func (h *Handler) liveMetrics(w http.ResponseWriter, r *http.Request) {
	lm, err := h.svc.LiveMetrics(r.Context())
	if err != nil {
		if errors.Is(err, domain.ErrMetricsUnavailable) {
			writeError(w, http.StatusServiceUnavailable, "metrics_unavailable")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, lm)
}

// serviceMetrics: métricas de um domínio. id desconhecido -> 404; Prometheus
// fora -> 503.
func (h *Handler) serviceMetrics(w http.ResponseWriter, r *http.Request) {
	sm, err := h.svc.ServiceMetrics(r.Context(), r.PathValue("id"))
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrUnknownService):
			writeError(w, http.StatusNotFound, "not_found")
		case errors.Is(err, domain.ErrMetricsUnavailable):
			writeError(w, http.StatusServiceUnavailable, "metrics_unavailable")
		default:
			writeError(w, http.StatusInternalServerError, "internal")
		}
		return
	}
	writeJSON(w, http.StatusOK, sm)
}

// playerAchievements depende do achievements-service: fonte fora -> 502.
func (h *Handler) playerAchievements(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.PlayerAchievements(r.Context(), r.PathValue("playerID"))
	if err != nil {
		if errors.Is(err, domain.ErrAchievementsUnavailable) {
			writeError(w, http.StatusBadGateway, "achievements_unavailable")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}

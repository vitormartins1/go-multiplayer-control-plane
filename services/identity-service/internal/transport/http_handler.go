// Package transport traduz HTTP para o domínio e vice-versa (handlers do
// identity-service). Sem lógica de negócio. Ver ADR-0002.
package transport

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/c130n/game-backend/services/identity-service/internal/domain"
)

// Handler expõe os endpoints HTTP do identity-service.
type Handler struct {
	auth   domain.AuthService
	tokens domain.TokenIssuer
}

// NewHandler cria o handler a partir das Ports do domínio.
func NewHandler(auth domain.AuthService, tokens domain.TokenIssuer) *Handler {
	return &Handler{auth: auth, tokens: tokens}
}

// Register monta as rotas no mux fornecido (permite ao main adicionar
// /healthz, /metrics etc. no mesmo mux).
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /accounts", h.register)
	mux.HandleFunc("POST /sessions", h.login)
	mux.HandleFunc("POST /sessions/anonymous", h.loginAnonymous)
	mux.HandleFunc("GET /.well-known/jwks.json", h.jwks)
}

type credentials struct {
	Handle   string `json:"handle"`
	Password string `json:"password"`
}

type accountResponse struct {
	ID        string    `json:"id"`
	Handle    string    `json:"handle"`
	CreatedAt time.Time `json:"created_at"`
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req credentials
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	acc, err := h.auth.Register(r.Context(), req.Handle, req.Password)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, accountResponse{ID: acc.ID, Handle: acc.Handle, CreatedAt: acc.CreatedAt})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req credentials
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	token, err := h.auth.Authenticate(r.Context(), req.Handle, req.Password)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (h *Handler) loginAnonymous(w http.ResponseWriter, r *http.Request) {
	token, err := h.auth.LoginAnonymous(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (h *Handler) jwks(w http.ResponseWriter, r *http.Request) {
	b, err := h.tokens.JWKS(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
}

func decode(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}

// writeDomainError mapeia erros de domínio para HTTP status + código estável.
func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidHandle):
		writeError(w, http.StatusBadRequest, "invalid_handle")
	case errors.Is(err, domain.ErrWeakPassword):
		writeError(w, http.StatusBadRequest, "weak_password")
	case errors.Is(err, domain.ErrHandleTaken):
		writeError(w, http.StatusConflict, "handle_taken")
	case errors.Is(err, domain.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "invalid_credentials")
	default:
		writeError(w, http.StatusInternalServerError, "internal")
	}
}

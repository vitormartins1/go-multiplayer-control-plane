// Package transport traduz HTTP para o domínio e vice-versa (handlers do
// identity-service). Sem lógica de negócio. Ver ADR-0002.
package transport

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/vitormartins1/go-game-backend/services/identity-service/internal/domain"
)

// Handler expõe os endpoints HTTP do identity-service.
type Handler struct {
	auth    domain.AuthService
	tokens  domain.TokenIssuer
	players domain.PlayerLister
}

// NewHandler cria o handler a partir das Ports do domínio.
func NewHandler(auth domain.AuthService, tokens domain.TokenIssuer, players domain.PlayerLister) *Handler {
	return &Handler{auth: auth, tokens: tokens, players: players}
}

// Register monta as rotas no mux fornecido (permite ao main adicionar
// /healthz, /metrics etc. no mesmo mux).
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /accounts", h.register)
	mux.HandleFunc("POST /sessions", h.login)
	mux.HandleFunc("POST /sessions/anonymous", h.loginAnonymous)
	mux.HandleFunc("GET /.well-known/jwks.json", h.jwks)

	// Admin (interno, sem auth — ver specs/003-admin-list-users): lista de usuários.
	mux.HandleFunc("GET /admin/players", h.adminListPlayers)
}

type credentials struct {
	Handle   string `json:"handle"`
	Password string `json:"password"`
	// GameId é opcional (login "dentro do jogo", ADR-0019): quando presente,
	// marca presença best-effort no game-registry após autenticar.
	GameId string `json:"gameId"`
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
	token, err := h.auth.Authenticate(r.Context(), req.Handle, req.Password, req.GameId)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (h *Handler) loginAnonymous(w http.ResponseWriter, r *http.Request) {
	// Corpo é opcional; quando presente pode trazer o gameId (login-into-game).
	var req struct {
		GameId string `json:"gameId"`
	}
	// Corpo vazio -> io.EOF, tolerado (gameId fica ""). Assim não depende do
	// Content-Length (cobre também Transfer-Encoding: chunked).
	if err := decode(r, &req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	token, err := h.auth.LoginAnonymous(r.Context(), req.GameId)
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

// playerDTO projeta a conta para o admin: nunca inclui password_hash.
type playerDTO struct {
	ID        string    `json:"id"`
	Handle    string    `json:"handle"`
	CreatedAt time.Time `json:"createdAt"`
}

type listPlayersResponse struct {
	Total   int         `json:"total"`
	Players []playerDTO `json:"players"`
}

// adminListPlayers lista os usuários (paginado) para o painel admin. Sem auth
// (interno). limit/offset vêm da query; ausentes/inválidos caem no default
// (o clamp dos limites fica no service).
func (h *Handler) adminListPlayers(w http.ResponseWriter, r *http.Request) {
	limit := atoiOr(r.URL.Query().Get("limit"), 0) // 0 -> service aplica o default
	offset := atoiOr(r.URL.Query().Get("offset"), 0)

	accounts, total, err := h.players.ListPlayers(r.Context(), limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	players := make([]playerDTO, len(accounts))
	for i, a := range accounts {
		players[i] = playerDTO{ID: a.ID, Handle: a.Handle, CreatedAt: a.CreatedAt}
	}
	writeJSON(w, http.StatusOK, listPlayersResponse{Total: total, Players: players})
}

// atoiOr devolve o inteiro de s, ou def se vazio/inválido.
func atoiOr(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
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

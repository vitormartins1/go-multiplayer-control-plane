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

	"github.com/vitormartins1/go-multiplayer-control-plane/services/identity-service/internal/domain"
	"github.com/vitormartins1/go-multiplayer-control-plane/shared/httpkit"
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
	// Lookup por id (specs/004-admin-player-by-id): handle de UM jogador.
	mux.HandleFunc("GET /admin/players/{playerID}", h.adminGetPlayer)
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
		httpkit.WriteError(w, http.StatusBadRequest, "bad_request")
		return
	}
	acc, err := h.auth.Register(r.Context(), req.Handle, req.Password)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	httpkit.WriteJSON(w, http.StatusCreated, accountResponse{ID: acc.ID, Handle: acc.Handle, CreatedAt: acc.CreatedAt})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req credentials
	if err := decode(r, &req); err != nil {
		httpkit.WriteError(w, http.StatusBadRequest, "bad_request")
		return
	}
	token, err := h.auth.Authenticate(r.Context(), req.Handle, req.Password, req.GameId)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	httpkit.WriteJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (h *Handler) loginAnonymous(w http.ResponseWriter, r *http.Request) {
	// Corpo é opcional; quando presente pode trazer o gameId (login-into-game).
	var req struct {
		GameId string `json:"gameId"`
	}
	// Corpo vazio -> io.EOF, tolerado (gameId fica ""). Assim não depende do
	// Content-Length (cobre também Transfer-Encoding: chunked).
	if err := decode(r, &req); err != nil && !errors.Is(err, io.EOF) {
		httpkit.WriteError(w, http.StatusBadRequest, "bad_request")
		return
	}
	token, err := h.auth.LoginAnonymous(r.Context(), req.GameId)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	httpkit.WriteJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (h *Handler) jwks(w http.ResponseWriter, r *http.Request) {
	b, err := h.tokens.JWKS(r.Context())
	if err != nil {
		httpkit.WriteError(w, http.StatusInternalServerError, "internal")
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
		httpkit.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}

	players := make([]playerDTO, len(accounts))
	for i, a := range accounts {
		players[i] = playerDTO{ID: a.ID, Handle: a.Handle, CreatedAt: a.CreatedAt}
	}
	httpkit.WriteJSON(w, http.StatusOK, listPlayersResponse{Total: total, Players: players})
}

// adminGetPlayer devolve um usuário pelo id (specs/004-admin-player-by-id).
// Conta inexistente -> 404 player_not_found.
func (h *Handler) adminGetPlayer(w http.ResponseWriter, r *http.Request) {
	acc, err := h.players.GetPlayer(r.Context(), r.PathValue("playerID"))
	if err != nil {
		if errors.Is(err, domain.ErrAccountNotFound) {
			httpkit.WriteError(w, http.StatusNotFound, "player_not_found")
			return
		}
		httpkit.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}
	httpkit.WriteJSON(w, http.StatusOK, playerDTO{ID: acc.ID, Handle: acc.Handle, CreatedAt: acc.CreatedAt})
}

// atoiOr devolve o inteiro de s, ou def se vazio/inválido.
func atoiOr(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}

// maxBodyBytes limita o corpo das rotas do identity (credenciais e gameId são
// pequenos; /accounts e /sessions não exigem auth, então o teto importa).
const maxBodyBytes = 64 << 10

func decode(r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// writeDomainError mapeia erros de domínio para HTTP status + código estável.
func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidHandle):
		httpkit.WriteError(w, http.StatusBadRequest, "invalid_handle")
	case errors.Is(err, domain.ErrWeakPassword):
		httpkit.WriteError(w, http.StatusBadRequest, "weak_password")
	case errors.Is(err, domain.ErrHandleTaken):
		httpkit.WriteError(w, http.StatusConflict, "handle_taken")
	case errors.Is(err, domain.ErrInvalidCredentials):
		httpkit.WriteError(w, http.StatusUnauthorized, "invalid_credentials")
	default:
		httpkit.WriteError(w, http.StatusInternalServerError, "internal")
	}
}

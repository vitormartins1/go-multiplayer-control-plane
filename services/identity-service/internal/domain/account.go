// Package domain contém os tipos de negócio puros e as interfaces (Ports)
// do identity-service. Zero dependências externas. Ver ADR-0002.
package domain

import (
	"context"
	"time"
)

// Account é a conta de um jogador registrado.
type Account struct {
	ID           string
	Handle       string
	PasswordHash string // representação irreversível da senha; nunca em claro
	CreatedAt    time.Time
}

// AuthService é a Port de entrada do identity-service (feature 001-auth-core).
// A feature 002-anonymous-login estende com LoginAnonymous; a 002-game-presence
// adiciona o gameID opcional (login "dentro do jogo" — ADR-0019): quando não
// vazio, marca presença best-effort após autenticar.
type AuthService interface {
	Register(ctx context.Context, handle, password string) (*Account, error)
	Authenticate(ctx context.Context, handle, password, gameID string) (token string, err error)
	// LoginAnonymous emite um session token para uma identidade guest
	// efêmera, sem credenciais e sem persistência (feature 002-anonymous-login).
	LoginAnonymous(ctx context.Context, gameID string) (token string, err error)
	// AuthenticateExternal troca um ticket de loja (ex.: Steam) pelo session
	// token: valida com o provedor, acha ou cria a conta vinculada e emite o
	// JWT de sempre (spec 005-steam-login). ErrProviderNotSupported,
	// ErrInvalidTicket, ErrExternalBanned, ErrProviderUnavailable.
	AuthenticateExternal(ctx context.Context, provider, ticket, gameID string) (token string, err error)
}

// ExternalIdentity é o jogador como o provedor externo o identifica.
type ExternalIdentity struct {
	Provider   string // "steam"
	ExternalID string // SteamID64
}

// ExternalIdentityLister lê os vínculos externos de uma conta (spec 006) —
// usado pelo achievements para espelhar conquistas na Steam.
type ExternalIdentityLister interface {
	ListExternalByAccount(ctx context.Context, accountID string) ([]ExternalIdentity, error)
}

// ExternalIdentityVerifier valida um ticket de um provedor (Steam, GOG, EOS…)
// e devolve a identidade externa. Port de saída (adapter em internal/steam).
type ExternalIdentityVerifier interface {
	Provider() string
	Verify(ctx context.Context, ticket string) (ExternalIdentity, error)
}

// ExternalIdentityRepository persiste o vínculo identidade externa → conta.
type ExternalIdentityRepository interface {
	// FindByExternal devolve a conta vinculada; ErrAccountNotFound se não há vínculo.
	FindByExternal(ctx context.Context, provider, externalID string) (*Account, error)
	// CreateWithExternal cria a conta e o vínculo na mesma transação.
	// ErrExternalLinked se o vínculo já existe (corrida com outro login);
	// ErrHandleTaken se o handle já está em uso.
	CreateWithExternal(ctx context.Context, a *Account, provider, externalID string) error
}

// PresenceNotifier é a Port de saída que registra a presença do jogador num
// jogo (login-into-game, ADR-0019). Implementada por um adapter HTTP para o
// game-registry. É best-effort: falha não derruba o login.
type PresenceNotifier interface {
	MarkPresence(ctx context.Context, gameID, playerID, handle string) error
}

// AccountRepository é a Port de saída de persistência (implementada em repository).
type AccountRepository interface {
	Create(ctx context.Context, a *Account) error
	// FindByHandle busca por handle de forma case-insensitive.
	FindByHandle(ctx context.Context, handle string) (*Account, error)
	// ListAccounts devolve contas por created_at desc (paginadas). Projeta
	// id/handle/created_at — nunca o password_hash (feature 003-admin-list-users).
	ListAccounts(ctx context.Context, limit, offset int) ([]Account, error)
	// CountAccounts devolve o total de contas (para a página exibir "N usuários").
	CountAccounts(ctx context.Context) (int, error)
	// FindByID busca uma conta pelo id. Mesma projeção do ListAccounts (sem
	// password_hash). ErrAccountNotFound quando não existe (feature 004).
	FindByID(ctx context.Context, id string) (*Account, error)
}

// PlayerLister é a Port de entrada do listing admin (feature 003-admin-list-users):
// lista as contas paginadas e devolve o total. Read-only, control plane interno.
// A feature 004-admin-player-by-id adiciona o lookup individual.
type PlayerLister interface {
	ListPlayers(ctx context.Context, limit, offset int) (players []Account, total int, err error)
	// GetPlayer devolve a conta pelo id; ErrAccountNotFound quando não existe.
	GetPlayer(ctx context.Context, id string) (*Account, error)
	// ExternalIdentities devolve os vínculos de loja da conta (spec 006);
	// vazio se não houver; ErrAccountNotFound quando a conta não existe.
	ExternalIdentities(ctx context.Context, id string) ([]ExternalIdentity, error)
}

// TokenIssuer é a Port de saída de emissão de tokens.
// Sign aceita claims extras (ex: "anon" na 002) para servir múltiplas features
// sem mudar a interface.
type TokenIssuer interface {
	Sign(ctx context.Context, subject string, claims map[string]any) (token string, err error)
	JWKS(ctx context.Context) ([]byte, error)
}

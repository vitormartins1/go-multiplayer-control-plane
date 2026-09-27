package transport

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Autenticação do gRPC do session (spec 030 / ADR-0030). O interceptor só
// IDENTIFICA o chamador pela metadata e confere se o tipo dele pode chamar o
// RPC; regras sobre o mundo ("é o host?") ficam no service
// (AuthorizeWorldHost), chamadas pelos handlers.

// Metadata aceita (chaves em minúsculas, como o gRPC as normaliza).
const (
	mdAuthorization = "authorization"
	mdInternalKey   = "x-internal-key"
	mdGameServerKey = "x-game-server-key"
	mdWorldToken    = "x-world-token" // host de mundo p2p (spec 031)
)

// CallerKind é o tipo de quem chama o session.
type CallerKind string

const (
	CallerAnonymous  CallerKind = "anonymous"
	CallerPlayer     CallerKind = "player"      // cliente do jogo, JWT do identity
	CallerInternal   CallerKind = "internal"    // matchmaking, internal key (ADR-0018)
	CallerGameServer CallerKind = "game_server" // Dedicated Server, GAME_SERVER_KEY
	CallerWorldHost  CallerKind = "world_host"  // cliente do host de mundo p2p, world token (spec 031)
)

// Caller é o chamador identificado. PlayerID só existe para CallerPlayer;
// SessionID só para CallerWorldHost (o único mundo que ele pode tocar), e
// WorldClosed diz que esse mundo já fechou.
type Caller struct {
	Kind        CallerKind
	PlayerID    string
	SessionID   string
	WorldClosed bool
}

// AuthMode é o modo de rollout (SESSION_AUTH_MODE).
type AuthMode string

const (
	AuthOff     AuthMode = "off"     // sem avaliação
	AuthWarn    AuthMode = "warn"    // avalia, loga e conta; não bloqueia
	AuthEnforce AuthMode = "enforce" // avalia e bloqueia
)

// ParseAuthMode lê o modo; valor vazio ou desconhecido vira warn (o padrão
// seguro para não quebrar um jogo ainda sem credenciais).
func ParseAuthMode(s string) AuthMode {
	switch AuthMode(strings.ToLower(strings.TrimSpace(s))) {
	case AuthOff:
		return AuthOff
	case AuthEnforce:
		return AuthEnforce
	default:
		return AuthWarn
	}
}

// PlayerVerifier valida o JWT do jogador e devolve o player_id (sub).
// Satisfeito por shared/auth.Verifier.
type PlayerVerifier interface {
	Verify(ctx context.Context, token string) (string, error)
}

// WorldTokenResolver devolve o mundo aberto dono de um world token (spec 031).
// Satisfeito pelo service.
type WorldTokenResolver interface {
	ResolveWorldToken(token string) (sessionID string, closed, ok bool)
}

// RejectionCounter conta as falhas de autenticação/autorização
// (session_auth_rejections_total{method,reason}). Satisfeito por um
// adapter fino sobre prometheus.CounterVec no main.
type RejectionCounter interface {
	Inc(method, reason string)
}

// AuthConfig configura a autenticação do servidor.
type AuthConfig struct {
	Mode          AuthMode
	Players       PlayerVerifier     // nil: bearer nunca valida
	InternalKey   string             // vazio: nunca valida (fail closed)
	GameServerKey string             // vazio: nunca valida (fail closed)
	WorldTokens   WorldTokenResolver // nil: world token nunca valida
	Rejections    RejectionCounter
	Log           *slog.Logger
}

const methodReportParticipation = "/session.v1.Session/ReportParticipationEvent"

// methodCallers diz que tipos de chamador cada RPC aceita. RPC fora da tabela
// (reflection, health) não é avaliado.
var methodCallers = map[string][]CallerKind{
	"/session.v1.Session/CreateSession":            {CallerInternal},
	"/session.v1.Session/ListInvadableWorlds":      {CallerInternal},
	"/session.v1.Session/JoinWorld":                {CallerInternal},
	"/session.v1.Session/OpenWorld":                {CallerPlayer},
	"/session.v1.Session/LeaveWorld":               {CallerPlayer, CallerGameServer, CallerWorldHost},
	"/session.v1.Session/CloseWorld":               {CallerPlayer, CallerGameServer, CallerWorldHost},
	"/session.v1.Session/ReportResult":             {CallerGameServer},
	"/session.v1.Session/ReportParticipationEvent": {CallerGameServer, CallerWorldHost},
	"/session.v1.Session/GetSessionConfig":         {CallerGameServer, CallerWorldHost},
	"/session.v1.Session/Heartbeat":                {CallerPlayer, CallerGameServer, CallerWorldHost},
}

type callerKey struct{}

// callerFrom devolve o chamador identificado pelo interceptor (anônimo se a
// autenticação estiver desligada ou se ninguém se identificou).
func callerFrom(ctx context.Context) Caller {
	if c, ok := ctx.Value(callerKey{}).(Caller); ok {
		return c
	}
	return Caller{Kind: CallerAnonymous}
}

// auth avalia credenciais e permissões conforme o modo.
type auth struct{ cfg AuthConfig }

// UnaryAuthInterceptor identifica o chamador, confere a tabela de permissões
// e põe o Caller no ctx. Em warn, a falha é só registrada.
func UnaryAuthInterceptor(cfg AuthConfig) grpc.UnaryServerInterceptor {
	a := &auth{cfg: cfg}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		allowed, evaluated := methodCallers[info.FullMethod]
		if !evaluated || a.cfg.Mode == AuthOff {
			return handler(ctx, req)
		}
		caller, reason := a.identify(ctx)
		// token de mundo fechado só serve para reenviar o evento que o fechou
		// (spec 029); o service confere se é mesmo essa ocorrência.
		if reason == "" && caller.WorldClosed && info.FullMethod != methodReportParticipation {
			caller, reason = Caller{Kind: CallerAnonymous}, "invalid_credential"
		}
		ctx = context.WithValue(ctx, callerKey{}, caller)
		if reason == "" && !kindIn(caller.Kind, allowed) {
			reason = "caller_not_allowed"
		}
		if reason != "" {
			code := codes.Unauthenticated
			if reason == "caller_not_allowed" {
				code = codes.PermissionDenied
			}
			if err := a.reject(ctx, info.FullMethod, reason, code); err != nil {
				return nil, err
			}
		}
		return handler(ctx, req)
	}
}

// identify devolve o chamador e, se houver problema de credencial, o motivo
// (missing_credential / invalid_credential). Credencial presente e inválida
// nunca rebaixa para anônimo silenciosamente.
func (a *auth) identify(ctx context.Context) (Caller, string) {
	md, _ := metadata.FromIncomingContext(ctx)
	if key := first(md, mdInternalKey); key != "" {
		if equalKey(key, a.cfg.InternalKey) {
			return Caller{Kind: CallerInternal}, ""
		}
		return Caller{Kind: CallerAnonymous}, "invalid_credential"
	}
	if key := first(md, mdGameServerKey); key != "" {
		if equalKey(key, a.cfg.GameServerKey) {
			return Caller{Kind: CallerGameServer}, ""
		}
		return Caller{Kind: CallerAnonymous}, "invalid_credential"
	}
	if tok := first(md, mdWorldToken); tok != "" {
		if a.cfg.WorldTokens != nil {
			if sessionID, closed, ok := a.cfg.WorldTokens.ResolveWorldToken(tok); ok {
				return Caller{Kind: CallerWorldHost, SessionID: sessionID, WorldClosed: closed}, ""
			}
		}
		return Caller{Kind: CallerAnonymous}, "invalid_credential"
	}
	if h := first(md, mdAuthorization); h != "" {
		token, ok := strings.CutPrefix(h, "Bearer ")
		if !ok || a.cfg.Players == nil {
			return Caller{Kind: CallerAnonymous}, "invalid_credential"
		}
		playerID, err := a.cfg.Players.Verify(ctx, token)
		if err != nil {
			return Caller{Kind: CallerAnonymous}, "invalid_credential"
		}
		return Caller{Kind: CallerPlayer, PlayerID: playerID}, ""
	}
	return Caller{Kind: CallerAnonymous}, "missing_credential"
}

// reject registra a falha; em enforce devolve o erro gRPC, em warn devolve nil
// (a chamada segue com a semântica anterior).
func (a *auth) reject(ctx context.Context, method, reason string, code codes.Code) error {
	if a.cfg.Rejections != nil {
		a.cfg.Rejections.Inc(method, reason)
	}
	if a.cfg.Log != nil {
		a.cfg.Log.WarnContext(ctx, "session auth: chamada sem permissão",
			slog.String("method", method), slog.String("reason", reason),
			slog.String("caller", string(callerFrom(ctx).Kind)), slog.String("mode", string(a.cfg.Mode)))
	}
	if a.cfg.Mode == AuthEnforce {
		return status.Error(code, "session auth: "+reason)
	}
	return nil
}

func kindIn(k CallerKind, ks []CallerKind) bool {
	for _, x := range ks {
		if x == k {
			return true
		}
	}
	return false
}

func first(md metadata.MD, key string) string {
	if v := md.Get(key); len(v) > 0 {
		return v[0]
	}
	return ""
}

// equalKey compara em tempo constante; chave configurada vazia nunca valida.
func equalKey(got, want string) bool {
	return want != "" && subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

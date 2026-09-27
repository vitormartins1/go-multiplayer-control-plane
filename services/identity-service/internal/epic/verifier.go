// Package epic valida o ID token do EOS Connect (Epic Online Services) —
// adapter de saída da Port domain.ExternalIdentityVerifier (spec 008-epic-login,
// ADR-0033).
//
// Fluxo: o jogo faz login no EOS Connect (com conta Epic, Steam ou de console) e
// recebe um ID token, um JWT assinado pela Epic com o ProductUserId (PUID) do
// jogador no sub. Aqui o token é validado localmente, pela chave pública do JWKS
// da Epic: nenhuma chamada à Epic no caminho do login, só a busca das chaves.
package epic

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/sync/singleflight"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/identity-service/internal/domain"
)

// DefaultJWKSURL é o JWKS público da Epic com as chaves que assinam o ID token.
const DefaultJWKSURL = "https://api.epicgames.dev/epic/oauth/v2/.well-known/jwks.json"

// issuerHost é o host que emite o ID token. Checado por igualdade: um prefixo
// de string aceitaria "api.epicgames.dev.outro-dominio".
const issuerHost = "api.epicgames.dev"

const (
	maxTokenLen        = 8 << 10 // ID tokens têm ~1 KiB; o teto evita parsear lixo arbitrário
	defaultKeyTTL      = time.Hour
	minRefetchInterval = 30 * time.Second // kid desconhecido não vira um GET na Epic por request
)

// algoritmos aceitos: assinatura assimétrica com as chaves do JWKS. "none" e
// HMAC nunca passam (com HMAC, a chave pública viraria segredo compartilhado).
var validMethods = []string{"RS256", "RS384", "RS512", "ES256", "ES384"}

// Config configura o verificador.
type Config struct {
	ClientID string        // client ID do produto no EOS (EPIC_CLIENT_ID): o aud esperado
	JWKSURL  string        // DefaultJWKSURL se vazio (EPIC_JWKS_URL)
	KeyTTL   time.Duration // defaultKeyTTL se zero
}

// Verifier valida ID tokens do EOS Connect.
type Verifier struct {
	cfg    Config
	client *http.Client
	now    func() time.Time

	mu          sync.RWMutex
	keys        map[string]crypto.PublicKey
	fetched     time.Time // último fetch bem-sucedido (TTL do cache)
	lastAttempt time.Time // última tentativa, com ou sem sucesso (limite de refetch)
	lastErr     error     // erro da última tentativa (nil se deu certo)

	sf singleflight.Group
}

var _ domain.ExternalIdentityVerifier = (*Verifier)(nil)

// New cria o verificador. client nil usa um http.Client com timeout de 5s.
func New(cfg Config, client *http.Client) *Verifier {
	if cfg.JWKSURL == "" {
		cfg.JWKSURL = DefaultJWKSURL
	}
	if cfg.KeyTTL == 0 {
		cfg.KeyTTL = defaultKeyTTL
	}
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return &Verifier{cfg: cfg, client: client, now: time.Now, keys: map[string]crypto.PublicKey{}}
}

// Provider é o nome do provedor na rota POST /sessions/{provider}.
func (v *Verifier) Provider() string { return "epic" }

// Verify valida o ID token e devolve o PUID. Token inválido por qualquer motivo
// (formato, assinatura, iss, aud, exp, iat) → ErrInvalidTicket; JWKS da Epic
// inalcançável sem chave em cache para o kid → ErrProviderUnavailable (também
// nas chamadas seguintes, enquanto a busca está limitada).
func (v *Verifier) Verify(ctx context.Context, ticket string) (domain.ExternalIdentity, error) {
	if ticket == "" || len(ticket) > maxTokenLen {
		return domain.ExternalIdentity{}, domain.ErrInvalidTicket
	}

	var unavailable error // a busca de chave falhou por causa da Epic, não do token
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(ticket, claims, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("token sem kid")
		}
		key, err := v.keyForKid(ctx, kid)
		if errors.Is(err, domain.ErrProviderUnavailable) {
			unavailable = err
		}
		return key, err
	}, jwt.WithValidMethods(validMethods),
		jwt.WithAudience(v.cfg.ClientID),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithTimeFunc(v.now))
	if unavailable != nil {
		return domain.ExternalIdentity{}, fmt.Errorf("epic: %w", unavailable)
	}
	if err != nil {
		return domain.ExternalIdentity{}, domain.ErrInvalidTicket
	}

	iss, _ := claims["iss"].(string)
	if u, err := url.Parse(iss); err != nil || u.Scheme != "https" || u.Host != issuerHost {
		return domain.ExternalIdentity{}, domain.ErrInvalidTicket
	}
	if _, ok := claims["iat"]; !ok { // WithIssuedAt só checa o iat quando ele existe
		return domain.ExternalIdentity{}, domain.ErrInvalidTicket
	}
	sub, _ := claims["sub"].(string)
	if sub == "" {
		return domain.ExternalIdentity{}, domain.ErrInvalidTicket
	}
	return domain.ExternalIdentity{Provider: v.Provider(), ExternalID: sub}, nil
}

// errUnknownKid: o JWKS atual (ou o cache, com a busca limitada) não tem o kid.
var errUnknownKid = errors.New("epic: kid desconhecido")

// errRefetchThrottled: houve tentativa de busca há menos de minRefetchInterval.
var errRefetchThrottled = errors.New("epic: busca do jwks limitada")

// keyForKid devolve a chave do kid, buscando o JWKS quando o kid é desconhecido
// ou o cache venceu. Mesmo desenho do shared/auth: buscas simultâneas viram uma,
// no máximo uma a cada minRefetchInterval, e uma chave em cache continua valendo
// se a Epic estiver fora.
func (v *Verifier) keyForKid(ctx context.Context, kid string) (crypto.PublicKey, error) {
	v.mu.RLock()
	key, ok := v.keys[kid]
	stale := v.now().Sub(v.fetched) >= v.cfg.KeyTTL
	v.mu.RUnlock()
	if ok && !stale {
		return key, nil
	}

	_, err, _ := v.sf.Do("jwks", func() (any, error) {
		v.mu.RLock()
		recent := !v.lastAttempt.IsZero() && v.now().Sub(v.lastAttempt) < minRefetchInterval
		v.mu.RUnlock()
		if recent {
			v.mu.RLock()
			lastErr := v.lastErr
			v.mu.RUnlock()
			if lastErr != nil {
				return nil, lastErr // a Epic caiu há pouco: continua indisponível, não "token ruim"
			}
			return nil, errRefetchThrottled
		}
		// sem o cancelamento de quem chegou primeiro: os outros que esperam no
		// singleflight não podem falhar junto. O http.Client tem timeout.
		err := v.refresh(context.WithoutCancel(ctx))
		v.mu.Lock()
		v.lastErr = err
		v.mu.Unlock()
		return nil, err
	})

	v.mu.RLock()
	fresh, found := v.keys[kid]
	v.mu.RUnlock()
	switch {
	case found:
		return fresh, nil
	case ok && err != nil:
		return key, nil // a busca falhou ou foi limitada: segue com a chave conhecida
	case err != nil && !errors.Is(err, errRefetchThrottled):
		return nil, fmt.Errorf("%w: %v", domain.ErrProviderUnavailable, err)
	default: // inclui a chave que a Epic tirou do JWKS (busca ok, kid ausente)
		return nil, errUnknownKid
	}
}

// jwk é o subconjunto dos campos de uma JSON Web Key que interessa aqui.
type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

// refresh busca o JWKS e substitui o cache de chaves.
func (v *Verifier) refresh(ctx context.Context) error {
	v.mu.Lock()
	v.lastAttempt = v.now()
	v.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.cfg.JWKSURL, nil)
	if err != nil {
		return fmt.Errorf("jwks request: %w", err)
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("jwks fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks status %d", resp.StatusCode)
	}
	var set struct {
		Keys []jwk `json:"keys"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(nil, resp.Body, 256<<10)).Decode(&set); err != nil {
		return fmt.Errorf("jwks decode: %w", err)
	}

	keys := make(map[string]crypto.PublicKey, len(set.Keys))
	for _, k := range set.Keys {
		if k.Kid == "" || (k.Use != "" && k.Use != "sig") {
			continue
		}
		pub, err := k.publicKey()
		if err != nil {
			continue // chave de um tipo que não usamos não derruba as outras
		}
		keys[k.Kid] = pub
	}
	v.mu.Lock()
	v.keys = keys
	v.fetched = v.now()
	v.mu.Unlock()
	return nil
}

// publicKey converte a JWK numa chave do crypto/*: RSA ou EC P-256/P-384.
func (k jwk) publicKey() (crypto.PublicKey, error) {
	switch k.Kty {
	case "RSA":
		n, err := b64int(k.N)
		if err != nil {
			return nil, err
		}
		e, err := b64int(k.E)
		if err != nil {
			return nil, err
		}
		if !e.IsInt64() || e.Int64() < 3 {
			return nil, errors.New("expoente RSA inválido")
		}
		return &rsa.PublicKey{N: n, E: int(e.Int64())}, nil
	case "EC":
		var curve elliptic.Curve
		switch k.Crv {
		case "P-256":
			curve = elliptic.P256()
		case "P-384":
			curve = elliptic.P384()
		default:
			return nil, fmt.Errorf("curva %q não suportada", k.Crv)
		}
		x, err := b64int(k.X)
		if err != nil {
			return nil, err
		}
		y, err := b64int(k.Y)
		if err != nil {
			return nil, err
		}
		pub := &ecdsa.PublicKey{Curve: curve, X: x, Y: y}
		if !curve.IsOnCurve(x, y) { //nolint:staticcheck // validação do ponto vindo da rede
			return nil, errors.New("ponto fora da curva")
		}
		return pub, nil
	default:
		return nil, fmt.Errorf("kty %q não suportado", k.Kty)
	}
}

func b64int(s string) (*big.Int, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil || len(b) == 0 {
		return nil, errors.New("inteiro base64url inválido")
	}
	return new(big.Int).SetBytes(b), nil
}

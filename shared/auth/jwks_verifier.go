// Package auth valida o JWT emitido pelo identity-service, buscando as chaves
// públicas no endpoint JWKS (ADR-0009). Compartilhado pelos serviços que
// consomem o token do identity; satisfaz estruturalmente a Port TokenVerifier
// de cada serviço (Verify(ctx, token) (playerID string, error)).
package auth

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"golang.org/x/sync/singleflight"
)

// Claims fixos do token de jogador (revisão 2026-09, G3): o identity emite
// iss/aud e o Verifier exige os dois. Sem isso, qualquer JWT EdDSA assinado
// por uma chave do JWKS valia como token de jogador — inclusive um token
// emitido para outro propósito pela mesma chave no futuro.
const (
	PlayerTokenIssuer   = "go-multiplayer-control-plane/identity"
	PlayerTokenAudience = "game-backend"
)

// minRefetchInterval limita o refetch do JWKS por kid desconhecido (G4): sem
// ele, cada request com um kid inventado virava um GET no identity —
// amplificação barata contra o identity. Rotação legítima de chave espera no
// máximo este intervalo.
const minRefetchInterval = 30 * time.Second

// ErrUnauthorized indica token ausente, malformado, expirado ou com assinatura
// inválida. O transport de cada serviço o mapeia para HTTP 401.
var ErrUnauthorized = errors.New("auth: não autorizado")

// Verifier valida tokens EdDSA contra o JWKS do identity, com cache por kid.
type Verifier struct {
	jwksURL string
	client  *http.Client
	ttl     time.Duration

	mu          sync.RWMutex
	keys        map[string]ed25519.PublicKey
	fetched     time.Time // último fetch bem-sucedido (TTL do cache)
	lastAttempt time.Time // última tentativa, com ou sem sucesso (throttle)

	sf  singleflight.Group // refetches concorrentes viram um só GET
	now func() time.Time
}

// NewVerifier cria um verificador que busca o JWKS em jwksURL e mantém as
// chaves em cache por ttl (com refetch sob kid desconhecido). jwksURL vem de
// configuração de deploy (IDENTITY_JWKS_URL, env var do operador — não é
// input de request) — mas uma URL vazia ou malformada não falhava aqui no
// boot (achado no /code-review): só aparecia tarde, dentro de refresh() no
// primeiro request autenticado, como um 401 genérico (ErrUnauthorized)
// sem pista de que a causa é configuração quebrada. Validado aqui, fail-fast,
// mesmo espírito de envOrFile (shared/config) — erro de deploy não deve
// degradar silenciosamente pra "toda autenticação falha".
func NewVerifier(jwksURL string, ttl time.Duration) (*Verifier, error) {
	u, err := url.Parse(jwksURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("auth: jwksURL inválida (%q) — precisa ser uma URL http(s) completa (ex.: http://identity-service:8080/.well-known/jwks.json)", jwksURL)
	}
	return &Verifier{
		jwksURL: jwksURL,
		// otelhttp propaga o trace context ao buscar o JWKS — o fetch vira um
		// span filho do request e cria o edge <serviço>->identity no service
		// graph (ADR-0014).
		client: &http.Client{
			Timeout:   5 * time.Second,
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		},
		ttl:  ttl,
		keys: map[string]ed25519.PublicKey{},
		now:  time.Now,
	}, nil
}

// Verify valida assinatura, expiração, issuer e audience do token e devolve o
// sub (playerID).
// Qualquer falha vira ErrUnauthorized (sem vazar o motivo ao chamador).
func (v *Verifier) Verify(ctx context.Context, token string) (string, error) {
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("token sem kid")
		}
		return v.keyForKid(ctx, kid)
	}, jwt.WithValidMethods([]string{"EdDSA"}),
		jwt.WithIssuer(PlayerTokenIssuer),
		jwt.WithAudience(PlayerTokenAudience),
		jwt.WithExpirationRequired())
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrUnauthorized, err)
	}
	sub, _ := claims["sub"].(string)
	if sub == "" {
		return "", ErrUnauthorized
	}
	return sub, nil
}

// keyForKid devolve a chave pública do kid, atualizando o cache quando o kid é
// desconhecido ou o cache está vencido. Refetches são coalescidos
// (singleflight) e limitados a um por minRefetchInterval: kid desconhecido
// dentro do intervalo falha sem ir à rede. Se o refetch falhar mas houver uma
// chave em cache (stale), usa-a — disponibilidade do identity não derruba a
// validação de tokens já conhecidos.
func (v *Verifier) keyForKid(ctx context.Context, kid string) (ed25519.PublicKey, error) {
	v.mu.RLock()
	key, ok := v.keys[kid]
	stale := v.now().Sub(v.fetched) >= v.ttl
	v.mu.RUnlock()
	if ok && !stale {
		return key, nil
	}

	// A checagem do intervalo mora DENTRO do singleflight: quem chega enquanto
	// um fetch está em andamento espera o resultado dele (em vez de ver o
	// lastAttempt recém-gravado e desistir antes das chaves chegarem).
	_, err, _ := v.sf.Do("jwks", func() (any, error) {
		v.mu.RLock()
		recent := v.now().Sub(v.lastAttempt) < minRefetchInterval
		v.mu.RUnlock()
		if recent {
			return nil, errRefetchThrottled
		}
		return nil, v.refresh(ctx)
	})
	if err != nil && !errors.Is(err, errRefetchThrottled) {
		if ok {
			return key, nil // fallback: usa a chave stale
		}
		return nil, err
	}

	v.mu.RLock()
	fresh, found := v.keys[kid]
	v.mu.RUnlock()
	switch {
	case found:
		return fresh, nil
	case ok && err != nil:
		return key, nil // throttled: segue com a chave stale conhecida
	default: // inclui kid revogado (fetch ok e o kid saiu do JWKS)
		return nil, fmt.Errorf("auth: kid desconhecido %q", kid)
	}
}

// errRefetchThrottled: houve tentativa de fetch há menos de minRefetchInterval.
var errRefetchThrottled = errors.New("auth: refetch do jwks limitado")

// refresh busca o JWKS e substitui o cache de chaves.
func (v *Verifier) refresh(ctx context.Context) error {
	v.mu.Lock()
	v.lastAttempt = v.now()
	v.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return fmt.Errorf("auth: jwks request: %w", err)
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("auth: jwks fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("auth: jwks status %d", resp.StatusCode)
	}

	var set struct {
		Keys []struct {
			Kty string `json:"kty"`
			Crv string `json:"crv"`
			X   string `json:"x"`
			Kid string `json:"kid"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&set); err != nil {
		return fmt.Errorf("auth: jwks decode: %w", err)
	}

	keys := make(map[string]ed25519.PublicKey, len(set.Keys))
	for _, k := range set.Keys {
		if k.Kty != "OKP" || k.Crv != "Ed25519" {
			continue
		}
		raw, err := base64.RawURLEncoding.DecodeString(k.X)
		if err != nil || len(raw) != ed25519.PublicKeySize {
			continue
		}
		keys[k.Kid] = ed25519.PublicKey(raw)
	}

	v.mu.Lock()
	v.keys = keys
	v.fetched = v.now()
	v.mu.Unlock()
	return nil
}

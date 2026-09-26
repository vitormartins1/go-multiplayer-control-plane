package auth

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func testKey(t *testing.T) (ed25519.PrivateKey, string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("gerar chave: %v", err)
	}
	sum := sha256.Sum256(pub)
	return priv, base64.RawURLEncoding.EncodeToString(sum[:8])
}

func jwksJSON(keys map[string]ed25519.PublicKey) []byte {
	var jwk []map[string]string
	for kid, pub := range keys {
		jwk = append(jwk, map[string]string{
			"kty": "OKP", "crv": "Ed25519", "alg": "EdDSA", "use": "sig",
			"kid": kid, "x": base64.RawURLEncoding.EncodeToString(pub),
		})
	}
	b, _ := json.Marshal(map[string]any{"keys": jwk})
	return b
}

func signToken(t *testing.T, priv ed25519.PrivateKey, kid, sub string, exp time.Time) string {
	t.Helper()
	return signClaims(t, priv, kid, jwt.MapClaims{
		"sub": sub, "iat": time.Now().Unix(), "exp": exp.Unix(),
		"iss": PlayerTokenIssuer, "aud": PlayerTokenAudience,
	})
}

func signClaims(t *testing.T, priv ed25519.PrivateKey, kid string, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	tok.Header["kid"] = kid
	s, err := tok.SignedString(priv)
	if err != nil {
		t.Fatalf("assinar: %v", err)
	}
	return s
}

func jwksServer(body []byte, hits *int32) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if hits != nil {
			atomic.AddInt32(hits, 1)
		}
		_, _ = w.Write(body)
	}))
}

func mustVerifier(t *testing.T, jwksURL string, ttl time.Duration) *Verifier {
	t.Helper()
	v, err := NewVerifier(jwksURL, ttl)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	return v
}

// TestNewVerifier_JwksURLInvalida_Erro prova o fail-fast (achado no
// /code-review): antes, uma jwksURL vazia/malformada só falhava tarde, no
// primeiro request autenticado, como um 401 genérico sem pista da causa.
func TestNewVerifier_JwksURLInvalida_Erro(t *testing.T) {
	casos := []string{"", "not a url", "ftp://identity/jwks.json", "identity-service/jwks.json"}
	for _, jwksURL := range casos {
		t.Run(jwksURL, func(t *testing.T) {
			if _, err := NewVerifier(jwksURL, time.Minute); err == nil {
				t.Fatalf("NewVerifier(%q) = nil error, quer erro de validação", jwksURL)
			}
		})
	}
}

func TestNewVerifier_JwksURLValida_SemErro(t *testing.T) {
	casos := []string{"http://identity-service:8080/.well-known/jwks.json", "https://identity.example.com/jwks.json"}
	for _, jwksURL := range casos {
		t.Run(jwksURL, func(t *testing.T) {
			if _, err := NewVerifier(jwksURL, time.Minute); err != nil {
				t.Fatalf("NewVerifier(%q) erro inesperado: %v", jwksURL, err)
			}
		})
	}
}

func TestVerify_TokenValido_RetornaPlayerID(t *testing.T) {
	priv, kid := testKey(t)
	srv := jwksServer(jwksJSON(map[string]ed25519.PublicKey{kid: priv.Public().(ed25519.PublicKey)}), nil)
	defer srv.Close()

	v := mustVerifier(t, srv.URL, time.Minute)
	token := signToken(t, priv, kid, "player-123", time.Now().Add(time.Minute))

	sub, err := v.Verify(context.Background(), token)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if sub != "player-123" {
		t.Fatalf("sub = %q, quer player-123", sub)
	}
}

func TestVerify_TokenExpirado_ErrUnauthorized(t *testing.T) {
	priv, kid := testKey(t)
	srv := jwksServer(jwksJSON(map[string]ed25519.PublicKey{kid: priv.Public().(ed25519.PublicKey)}), nil)
	defer srv.Close()

	v := mustVerifier(t, srv.URL, time.Minute)
	token := signToken(t, priv, kid, "player-123", time.Now().Add(-time.Minute))

	_, err := v.Verify(context.Background(), token)
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("err = %v, quer ErrUnauthorized", err)
	}
}

func TestVerify_AssinaturaInvalida_ErrUnauthorized(t *testing.T) {
	priv, kid := testKey(t)
	other, _ := testKey(t)
	srv := jwksServer(jwksJSON(map[string]ed25519.PublicKey{kid: priv.Public().(ed25519.PublicKey)}), nil)
	defer srv.Close()

	v := mustVerifier(t, srv.URL, time.Minute)
	token := signToken(t, other, kid, "player-123", time.Now().Add(time.Minute))

	_, err := v.Verify(context.Background(), token)
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("err = %v, quer ErrUnauthorized", err)
	}
}

func TestVerify_KidDesconhecido_ErrUnauthorized(t *testing.T) {
	priv, kid := testKey(t)
	srv := jwksServer(jwksJSON(map[string]ed25519.PublicKey{kid: priv.Public().(ed25519.PublicKey)}), nil)
	defer srv.Close()

	v := mustVerifier(t, srv.URL, time.Minute)
	token := signToken(t, priv, "kid-fantasma", "player-123", time.Now().Add(time.Minute))

	_, err := v.Verify(context.Background(), token)
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("err = %v, quer ErrUnauthorized", err)
	}
}

func TestVerify_TokenMalformado_ErrUnauthorized(t *testing.T) {
	priv, kid := testKey(t)
	srv := jwksServer(jwksJSON(map[string]ed25519.PublicKey{kid: priv.Public().(ed25519.PublicKey)}), nil)
	defer srv.Close()

	v := mustVerifier(t, srv.URL, time.Minute)

	_, err := v.Verify(context.Background(), "nao-eh-um-jwt")
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("err = %v, quer ErrUnauthorized", err)
	}
}

func TestVerify_Cacheia_NaoRebuscaPorRequest(t *testing.T) {
	priv, kid := testKey(t)
	var hits int32
	srv := jwksServer(jwksJSON(map[string]ed25519.PublicKey{kid: priv.Public().(ed25519.PublicKey)}), &hits)
	defer srv.Close()

	v := mustVerifier(t, srv.URL, time.Minute)
	for i := 0; i < 3; i++ {
		token := signToken(t, priv, kid, "player-123", time.Now().Add(time.Minute))
		if _, err := v.Verify(context.Background(), token); err != nil {
			t.Fatalf("verify %d: %v", i, err)
		}
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("fetches de JWKS = %d, quer 1 (cache)", got)
	}
}

// G3: o token precisa trazer o issuer e a audience do identity.
func TestVerify_SemIssOuAudOuComValorErrado_ErrUnauthorized(t *testing.T) {
	priv, kid := testKey(t)
	srv := jwksServer(jwksJSON(map[string]ed25519.PublicKey{kid: priv.Public().(ed25519.PublicKey)}), nil)
	defer srv.Close()
	v := mustVerifier(t, srv.URL, time.Minute)
	exp := time.Now().Add(time.Minute).Unix()

	cases := map[string]jwt.MapClaims{
		"sem iss/aud":   {"sub": "p1", "exp": exp},
		"iss errado":    {"sub": "p1", "exp": exp, "iss": "outro", "aud": PlayerTokenAudience},
		"aud errado":    {"sub": "p1", "exp": exp, "iss": PlayerTokenIssuer, "aud": "outro"},
		"sem expiração": {"sub": "p1", "iss": PlayerTokenIssuer, "aud": PlayerTokenAudience},
	}
	for name, claims := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := v.Verify(context.Background(), signClaims(t, priv, kid, claims))
			if !errors.Is(err, ErrUnauthorized) {
				t.Fatalf("err = %v, quer ErrUnauthorized", err)
			}
		})
	}
}

// G4: kid inventado em rajada não vira um GET no identity por request —
// no máximo um refetch por minRefetchInterval.
func TestVerify_KidDesconhecidoEmRajada_UmFetchPorIntervalo(t *testing.T) {
	priv, kid := testKey(t)
	var hits int32
	srv := jwksServer(jwksJSON(map[string]ed25519.PublicKey{kid: priv.Public().(ed25519.PublicKey)}), &hits)
	defer srv.Close()
	v := mustVerifier(t, srv.URL, time.Hour)

	for i := 0; i < 20; i++ {
		token := signToken(t, priv, "kid-fantasma", "p1", time.Now().Add(time.Minute))
		if _, err := v.Verify(context.Background(), token); !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("verify %d: err = %v, quer ErrUnauthorized", i, err)
		}
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("fetches de JWKS = %d, quer 1", got)
	}
}

// G4: cache frio sob concorrência — os refetches são coalescidos num só GET.
func TestVerify_CacheFrioConcorrente_UmFetch(t *testing.T) {
	priv, kid := testKey(t)
	var hits int32
	srv := jwksServer(jwksJSON(map[string]ed25519.PublicKey{kid: priv.Public().(ed25519.PublicKey)}), &hits)
	defer srv.Close()
	v := mustVerifier(t, srv.URL, time.Hour)
	token := signToken(t, priv, kid, "p1", time.Now().Add(time.Minute))

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := v.Verify(context.Background(), token); err != nil {
				t.Errorf("verify: %v", err)
			}
		}()
	}
	wg.Wait()
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("fetches de JWKS = %d, quer 1", got)
	}
}

// Rotação de chave: um kid novo é aceito depois do intervalo mínimo.
func TestVerify_RotacaoDeChave_AceitaKidNovoAposIntervalo(t *testing.T) {
	oldPriv, oldKid := testKey(t)
	newPriv, newKid := testKey(t)
	var mu sync.Mutex
	keys := map[string]ed25519.PublicKey{oldKid: oldPriv.Public().(ed25519.PublicKey)}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		_, _ = w.Write(jwksJSON(keys))
	}))
	defer srv.Close()

	clock := time.Now()
	v := mustVerifier(t, srv.URL, time.Hour)
	v.now = func() time.Time { return clock }
	if _, err := v.Verify(context.Background(), signToken(t, oldPriv, oldKid, "p1", time.Now().Add(time.Minute))); err != nil {
		t.Fatalf("verify chave antiga: %v", err)
	}

	mu.Lock()
	keys[newKid] = newPriv.Public().(ed25519.PublicKey)
	mu.Unlock()
	newToken := signToken(t, newPriv, newKid, "p1", time.Now().Add(time.Minute))
	if _, err := v.Verify(context.Background(), newToken); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("dentro do intervalo o kid novo ainda não é buscado; err = %v", err)
	}

	clock = clock.Add(minRefetchInterval + time.Second)
	if _, err := v.Verify(context.Background(), newToken); err != nil {
		t.Fatalf("após o intervalo o kid novo deve valer: %v", err)
	}
}

// Chave revogada: fetch bem-sucedido sem o kid não cai na chave antiga do cache.
func TestVerify_KidRevogado_NaoUsaChaveStale(t *testing.T) {
	priv, kid := testKey(t)
	var mu sync.Mutex
	keys := map[string]ed25519.PublicKey{kid: priv.Public().(ed25519.PublicKey)}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		_, _ = w.Write(jwksJSON(keys))
	}))
	defer srv.Close()

	clock := time.Now()
	v := mustVerifier(t, srv.URL, time.Minute)
	v.now = func() time.Time { return clock }
	token := signToken(t, priv, kid, "p1", time.Now().Add(time.Hour))
	if _, err := v.Verify(context.Background(), token); err != nil {
		t.Fatalf("verify inicial: %v", err)
	}

	mu.Lock()
	delete(keys, kid)
	mu.Unlock()
	clock = clock.Add(2 * time.Minute) // cache vencido e fora do intervalo mínimo
	if _, err := v.Verify(context.Background(), token); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("kid revogado deve falhar; err = %v", err)
	}
}

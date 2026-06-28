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
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{
		"sub": sub, "iat": time.Now().Unix(), "exp": exp.Unix(),
	})
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

func TestVerify_TokenValido_RetornaPlayerID(t *testing.T) {
	priv, kid := testKey(t)
	srv := jwksServer(jwksJSON(map[string]ed25519.PublicKey{kid: priv.Public().(ed25519.PublicKey)}), nil)
	defer srv.Close()

	v := NewVerifier(srv.URL, time.Minute)
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

	v := NewVerifier(srv.URL, time.Minute)
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

	v := NewVerifier(srv.URL, time.Minute)
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

	v := NewVerifier(srv.URL, time.Minute)
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

	v := NewVerifier(srv.URL, time.Minute)

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

	v := NewVerifier(srv.URL, time.Minute)
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

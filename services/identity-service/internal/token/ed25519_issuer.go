// Package token implementa a emissão de JWT (Ed25519) e o JWKS público.
// É um adapter de saída que satisfaz domain.TokenIssuer (ADR-0002).
package token

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/vitormartins1/go-multiplayer-control-plane/shared/auth"
)

// DefaultTTL é a validade padrão do session token.
const DefaultTTL = 15 * time.Minute

// Issuer assina JWTs com Ed25519 e expõe a chave pública via JWKS.
type Issuer struct {
	priv ed25519.PrivateKey
	pub  ed25519.PublicKey
	kid  string
	ttl  time.Duration
}

// New cria um Issuer a partir de uma chave privada Ed25519.
func New(priv ed25519.PrivateKey) *Issuer {
	pub := priv.Public().(ed25519.PublicKey)
	return &Issuer{priv: priv, pub: pub, kid: keyID(pub), ttl: DefaultTTL}
}

// Generate cria um Issuer com um par de chaves efêmero (dev/test).
func Generate() (*Issuer, error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("token: generate key: %w", err)
	}
	return New(priv), nil
}

// Sign emite um JWT EdDSA com sub/iat/exp/iss/aud/kid e os claims extras informados.
func (i *Issuer) Sign(_ context.Context, subject string, claims map[string]any) (string, error) {
	now := time.Now()
	mc := jwt.MapClaims{
		"sub": subject,
		"iat": now.Unix(),
		"exp": now.Add(i.ttl).Unix(),
		// exigidos pelo shared/auth.Verifier (revisão 2026-09, G3)
		"iss": auth.PlayerTokenIssuer,
		"aud": auth.PlayerTokenAudience,
	}
	for k, v := range claims {
		mc[k] = v
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, mc)
	tok.Header["kid"] = i.kid
	signed, err := tok.SignedString(i.priv)
	if err != nil {
		return "", fmt.Errorf("token: sign: %w", err)
	}
	return signed, nil
}

// JWKS retorna o conjunto de chaves públicas (JSON) para verificação local
// pelos consumidores. Nunca inclui a chave privada.
func (i *Issuer) JWKS(_ context.Context) ([]byte, error) {
	jwk := map[string]string{
		"kty": "OKP",
		"crv": "Ed25519",
		"x":   base64.RawURLEncoding.EncodeToString(i.pub),
		"use": "sig",
		"alg": "EdDSA",
		"kid": i.kid,
	}
	set := map[string]any{"keys": []map[string]string{jwk}}
	b, err := json.Marshal(set)
	if err != nil {
		return nil, fmt.Errorf("token: jwks: %w", err)
	}
	return b, nil
}

// PublicKey expõe a chave pública (verificação em testes/consumidores locais).
func (i *Issuer) PublicKey() ed25519.PublicKey { return i.pub }

// KeyID é o identificador (kid) da chave atual.
func (i *Issuer) KeyID() string { return i.kid }

// keyID deriva um kid estável dos primeiros bytes do SHA-256 da chave pública.
func keyID(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	return base64.RawURLEncoding.EncodeToString(sum[:8])
}

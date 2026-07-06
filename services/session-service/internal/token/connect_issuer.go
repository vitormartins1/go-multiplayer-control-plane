// Package token implementa a emissão do connect token (JWT EdDSA) e o JWKS
// público do session-service. É um adapter de saída que satisfaz
// domain.ConnectTokenIssuer (ADR-0002). A chave é PRÓPRIA do session — distinta
// da do identity: cunhar o token de partida é responsabilidade do session.
// O game server valida o token offline via JWKS (ADR-0010/0011).
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

	"github.com/vitormartins1/go-game-backend/services/session-service/internal/domain"
)

var _ domain.ConnectTokenIssuer = (*ConnectIssuer)(nil)

// DefaultTTL é a validade padrão do connect token (janela para o jogador
// conectar no game server após o match).
const DefaultTTL = 5 * time.Minute

// ConnectIssuer assina connect tokens com Ed25519 e expõe a chave pública via JWKS.
type ConnectIssuer struct {
	priv ed25519.PrivateKey
	pub  ed25519.PublicKey
	kid  string
	ttl  time.Duration
}

// New cria um ConnectIssuer a partir de uma chave privada Ed25519.
func New(priv ed25519.PrivateKey) *ConnectIssuer {
	pub := priv.Public().(ed25519.PublicKey)
	return &ConnectIssuer{priv: priv, pub: pub, kid: keyID(pub), ttl: DefaultTTL}
}

// Generate cria um ConnectIssuer com um par de chaves efêmero (dev/test).
func Generate() (*ConnectIssuer, error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("token: generate key: %w", err)
	}
	return New(priv), nil
}

// Issue emite um connect token EdDSA com sub/match_id/server/iat/exp/kid e,
// quando teamID não é vazio, o claim team_id (ADR-0023). O game server valida
// offline (JWKS) que o portador foi autorizado para a partida e o servidor
// indicados e lê o team_id para o gameplay NvN.
func (i *ConnectIssuer) Issue(playerID, matchID, serverAddr, teamID string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":      playerID,
		"match_id": matchID,
		"server":   serverAddr,
		"iat":      now.Unix(),
		"exp":      now.Add(i.ttl).Unix(),
	}
	if teamID != "" {
		claims["team_id"] = teamID // omitido em FFA para não poluir o token
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	tok.Header["kid"] = i.kid
	signed, err := tok.SignedString(i.priv)
	if err != nil {
		return "", fmt.Errorf("token: sign: %w", err)
	}
	return signed, nil
}

// JWKS retorna o conjunto de chaves públicas (JSON) para verificação offline
// pelo game server. Nunca inclui a chave privada.
func (i *ConnectIssuer) JWKS(_ context.Context) ([]byte, error) {
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
func (i *ConnectIssuer) PublicKey() ed25519.PublicKey { return i.pub }

// KeyID é o identificador (kid) da chave atual.
func (i *ConnectIssuer) KeyID() string { return i.kid }

// keyID deriva um kid estável dos primeiros bytes do SHA-256 da chave pública.
func keyID(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	return base64.RawURLEncoding.EncodeToString(sum[:8])
}

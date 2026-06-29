package token

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIssue_TokenVerificavelComChavePublica_RetornaClaims(t *testing.T) {
	iss, err := Generate()
	require.NoError(t, err)

	signed, err := iss.Issue("player-123", "match-42", "10.0.0.1:7777")
	require.NoError(t, err)

	parsed, err := jwt.Parse(signed, func(*jwt.Token) (any, error) {
		return iss.PublicKey(), nil
	}, jwt.WithValidMethods([]string{"EdDSA"}))
	require.NoError(t, err)
	require.True(t, parsed.Valid)

	claims := parsed.Claims.(jwt.MapClaims)
	assert.Equal(t, "player-123", claims["sub"])
	assert.Equal(t, "match-42", claims["match_id"])
	assert.Equal(t, "10.0.0.1:7777", claims["server"])
	assert.Equal(t, iss.KeyID(), parsed.Header["kid"])
	assert.Contains(t, claims, "exp")
	assert.Contains(t, claims, "iat")
}

func TestIssue_TokenExpirado_FalhaNaVerificacao(t *testing.T) {
	iss, err := Generate()
	require.NoError(t, err)
	iss.ttl = -time.Minute // já nasce expirado

	signed, err := iss.Issue("player-123", "match-42", "10.0.0.1:7777")
	require.NoError(t, err)

	_, err = jwt.Parse(signed, func(*jwt.Token) (any, error) {
		return iss.PublicKey(), nil
	}, jwt.WithValidMethods([]string{"EdDSA"}))
	assert.ErrorIs(t, err, jwt.ErrTokenExpired)
}

func TestJWKS_ContemKidPublica_NuncaPrivada(t *testing.T) {
	iss, err := Generate()
	require.NoError(t, err)

	raw, err := iss.JWKS(context.Background())
	require.NoError(t, err)

	var set struct {
		Keys []map[string]string `json:"keys"`
	}
	require.NoError(t, json.Unmarshal(raw, &set))
	require.Len(t, set.Keys, 1)

	k := set.Keys[0]
	assert.Equal(t, "OKP", k["kty"])
	assert.Equal(t, "Ed25519", k["crv"])
	assert.Equal(t, iss.KeyID(), k["kid"])
	assert.NotEmpty(t, k["x"])    // coordenada pública
	assert.NotContains(t, k, "d") // jamais a chave privada
}

// Simula o game server: reconstrói a chave pública a partir do JWKS exposto e
// valida o connect token offline, sem chamar o backend (ADR-0010/0011).
func TestConnectToken_ValidadoPelaChaveReconstruidaDoJWKS(t *testing.T) {
	iss, err := Generate()
	require.NoError(t, err)

	signed, err := iss.Issue("player-123", "match-42", "10.0.0.1:7777")
	require.NoError(t, err)

	raw, err := iss.JWKS(context.Background())
	require.NoError(t, err)

	var set struct {
		Keys []struct {
			X string `json:"x"`
		} `json:"keys"`
	}
	require.NoError(t, json.Unmarshal(raw, &set))
	require.Len(t, set.Keys, 1)

	xBytes, err := base64.RawURLEncoding.DecodeString(set.Keys[0].X)
	require.NoError(t, err)
	pub := ed25519.PublicKey(xBytes)

	parsed, err := jwt.Parse(signed, func(*jwt.Token) (any, error) {
		return pub, nil
	}, jwt.WithValidMethods([]string{"EdDSA"}))
	require.NoError(t, err)
	assert.True(t, parsed.Valid)
}

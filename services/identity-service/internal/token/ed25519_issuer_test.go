package token

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSign_TokenVerificavelComChavePublica_RetornaClaims(t *testing.T) {
	iss, err := Generate()
	require.NoError(t, err)

	signed, err := iss.Sign(context.Background(), "player-123", map[string]any{"anon": true})
	require.NoError(t, err)

	parsed, err := jwt.Parse(signed, func(*jwt.Token) (any, error) {
		return iss.PublicKey(), nil
	}, jwt.WithValidMethods([]string{"EdDSA"}))
	require.NoError(t, err)
	require.True(t, parsed.Valid)

	claims := parsed.Claims.(jwt.MapClaims)
	assert.Equal(t, "player-123", claims["sub"])
	assert.Equal(t, true, claims["anon"])
	assert.Equal(t, iss.KeyID(), parsed.Header["kid"])
	assert.Contains(t, claims, "exp")
	assert.Contains(t, claims, "iat")
}

func TestSign_TokenExpirado_FalhaNaVerificacao(t *testing.T) {
	iss, err := Generate()
	require.NoError(t, err)
	iss.ttl = -time.Minute // já nasce expirado

	signed, err := iss.Sign(context.Background(), "player-123", nil)
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

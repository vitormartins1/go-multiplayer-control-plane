package epic

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/identity-service/internal/domain"
)

const clientID = "xyza7891test"

// fakeEpic é um JWKS falso: as chaves publicadas mudam durante o teste (rotação)
// e ele pode sair do ar.
type fakeEpic struct {
	mu    sync.Mutex
	keys  []map[string]string
	down  bool
	calls atomic.Int32
	srv   *httptest.Server
}

func newFakeEpic(t *testing.T) *fakeEpic {
	t.Helper()
	f := &fakeEpic{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		f.calls.Add(1)
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.down {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": f.keys})
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeEpic) publish(keys ...map[string]string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.keys = keys
}

func (f *fakeEpic) setDown(down bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.down = down
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func rsaJWK(kid string, k *rsa.PrivateKey) map[string]string {
	return map[string]string{"kty": "RSA", "kid": kid, "use": "sig", "alg": "RS256",
		"n": b64(k.N.Bytes()), "e": b64(big.NewInt(int64(k.E)).Bytes())}
}

func ecJWK(kid string, k *ecdsa.PrivateKey) map[string]string {
	return map[string]string{"kty": "EC", "kid": kid, "crv": "P-256",
		"x": b64(k.X.FillBytes(make([]byte, 32))), "y": b64(k.Y.FillBytes(make([]byte, 32)))}
}

// clock é o relógio do verificador nos testes (a expiração depende dele).
var epoch = time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)

func validClaims() jwt.MapClaims {
	return jwt.MapClaims{
		"iss": "https://api.epicgames.dev/auth/v1/oauth",
		"sub": "0002b2c7e8b54f6c9a1d3e4f5a6b7c8d",
		"aud": clientID,
		"iat": epoch.Add(-time.Minute).Unix(),
		"exp": epoch.Add(time.Hour).Unix(),
	}
}

func sign(t *testing.T, method jwt.SigningMethod, kid string, key any, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(method, claims)
	tok.Header["kid"] = kid
	s, err := tok.SignedString(key)
	require.NoError(t, err)
	return s
}

func newVerifier(f *fakeEpic) *Verifier {
	v := New(Config{ClientID: clientID, JWKSURL: f.srv.URL}, nil)
	v.now = func() time.Time { return epoch }
	return v
}

func TestVerify_TokenValidoRSA_DevolvePUID(t *testing.T) {
	rk, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	f := newFakeEpic(t)
	f.publish(rsaJWK("k1", rk))

	ext, err := newVerifier(f).Verify(context.Background(), sign(t, jwt.SigningMethodRS256, "k1", rk, validClaims()))

	require.NoError(t, err)
	assert.Equal(t, domain.ExternalIdentity{Provider: "epic", ExternalID: "0002b2c7e8b54f6c9a1d3e4f5a6b7c8d"}, ext)
}

func TestVerify_TokenValidoEC_DevolvePUID(t *testing.T) {
	ek, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	f := newFakeEpic(t)
	f.publish(ecJWK("e1", ek))

	ext, err := newVerifier(f).Verify(context.Background(), sign(t, jwt.SigningMethodES256, "e1", ek, validClaims()))

	require.NoError(t, err)
	assert.Equal(t, "epic", ext.Provider)
}

func TestVerify_TokenInvalido_ErrInvalidTicket(t *testing.T) {
	rk, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	outra, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	f := newFakeEpic(t)
	f.publish(rsaJWK("k1", rk))

	with := func(k string, val any) jwt.MapClaims {
		c := validClaims()
		c[k] = val
		return c
	}
	semSub := validClaims()
	delete(semSub, "sub")
	semExp := validClaims()
	delete(semExp, "exp")
	semIat := validClaims()
	delete(semIat, "iat")
	noneTok, err := jwt.NewWithClaims(jwt.SigningMethodNone, validClaims()).SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	cases := map[string]string{
		"vazio":                  "",
		"lixo":                   "nao.e.jwt",
		"assinado por outra":     sign(t, jwt.SigningMethodRS256, "k1", outra, validClaims()),
		"alg none":               noneTok,
		"hmac com a chave":       sign(t, jwt.SigningMethodHS256, "k1", []byte("segredo"), validClaims()),
		"iss de outro host":      sign(t, jwt.SigningMethodRS256, "k1", rk, with("iss", "https://api.epicgames.dev.evil.com/auth")),
		"iss sem https":          sign(t, jwt.SigningMethodRS256, "k1", rk, with("iss", "http://api.epicgames.dev/auth")),
		"aud de outro produto":   sign(t, jwt.SigningMethodRS256, "k1", rk, with("aud", "outro-client")),
		"expirado":               sign(t, jwt.SigningMethodRS256, "k1", rk, with("exp", epoch.Add(-time.Second).Unix())),
		"sem exp":                sign(t, jwt.SigningMethodRS256, "k1", rk, semExp),
		"sem iat":                sign(t, jwt.SigningMethodRS256, "k1", rk, semIat),
		"iat no futuro":          sign(t, jwt.SigningMethodRS256, "k1", rk, with("iat", epoch.Add(time.Minute).Unix())),
		"sem sub":                sign(t, jwt.SigningMethodRS256, "k1", rk, semSub),
		"kid que a Epic não tem": sign(t, jwt.SigningMethodRS256, "k9", rk, validClaims()),
	}
	for name, tok := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := newVerifier(f).Verify(context.Background(), tok)
			assert.ErrorIs(t, err, domain.ErrInvalidTicket)
		})
	}
}

func TestVerify_RotacaoDeChave_KidNovoBuscaOJWKSDeNovo(t *testing.T) {
	k1, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	k2, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	f := newFakeEpic(t)
	f.publish(rsaJWK("k1", k1))
	v := newVerifier(f)
	_, err = v.Verify(context.Background(), sign(t, jwt.SigningMethodRS256, "k1", k1, validClaims()))
	require.NoError(t, err)

	// a Epic gira a chave; depois do intervalo mínimo, o kid novo dispara uma busca
	f.publish(rsaJWK("k2", k2))
	v.now = func() time.Time { return epoch.Add(minRefetchInterval) }
	claims := validClaims()
	claims["iat"] = epoch.Unix()
	_, err = v.Verify(context.Background(), sign(t, jwt.SigningMethodRS256, "k2", k2, claims))

	require.NoError(t, err)
	assert.EqualValues(t, 2, f.calls.Load())
}

func TestVerify_KidDesconhecidoRepetido_NaoMartelaAEpic(t *testing.T) {
	rk, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	f := newFakeEpic(t)
	f.publish(rsaJWK("k1", rk))
	v := newVerifier(f)

	for range 5 {
		_, err := v.Verify(context.Background(), sign(t, jwt.SigningMethodRS256, "inventado", rk, validClaims()))
		require.ErrorIs(t, err, domain.ErrInvalidTicket)
	}

	assert.EqualValues(t, 1, f.calls.Load(), "dentro do intervalo mínimo, só a primeira busca vai à rede")
}

func TestVerify_EpicForaSemChaveEmCache_ErrProviderUnavailable(t *testing.T) {
	rk, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	f := newFakeEpic(t)
	f.setDown(true)

	_, err = newVerifier(f).Verify(context.Background(), sign(t, jwt.SigningMethodRS256, "k1", rk, validClaims()))

	assert.ErrorIs(t, err, domain.ErrProviderUnavailable)
}

func TestVerify_ChaveRetiradaDoJWKS_ErrInvalidTicket(t *testing.T) {
	k1, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	k2, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	f := newFakeEpic(t)
	f.publish(rsaJWK("k1", k1))
	v := newVerifier(f)
	_, err = v.Verify(context.Background(), sign(t, jwt.SigningMethodRS256, "k1", k1, validClaims()))
	require.NoError(t, err)

	// o cache vence e a Epic já girou: k1 saiu do JWKS e não vale mais
	f.publish(rsaJWK("k2", k2))
	v.now = func() time.Time { return epoch.Add(defaultKeyTTL + time.Minute) }
	claims := validClaims()
	claims["exp"] = epoch.Add(2 * defaultKeyTTL).Unix()
	_, err = v.Verify(context.Background(), sign(t, jwt.SigningMethodRS256, "k1", k1, claims))

	assert.ErrorIs(t, err, domain.ErrInvalidTicket)
	assert.EqualValues(t, 2, f.calls.Load())
}

func TestVerify_EpicForaChamadasSeguidas_TodasErrProviderUnavailable(t *testing.T) {
	rk, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	f := newFakeEpic(t)
	f.setDown(true)
	v := newVerifier(f)

	for range 3 {
		_, err := v.Verify(context.Background(), sign(t, jwt.SigningMethodRS256, "k1", rk, validClaims()))
		require.ErrorIs(t, err, domain.ErrProviderUnavailable)
	}
	assert.EqualValues(t, 1, f.calls.Load(), "dentro do intervalo mínimo, só a primeira busca vai à rede")

	// a Epic volta: passado o intervalo, a busca seguinte funciona
	f.setDown(false)
	f.publish(rsaJWK("k1", rk))
	v.now = func() time.Time { return epoch.Add(minRefetchInterval) }
	claims := validClaims()
	claims["iat"] = epoch.Unix()
	_, err = v.Verify(context.Background(), sign(t, jwt.SigningMethodRS256, "k1", rk, claims))
	assert.NoError(t, err)
}

func TestVerify_EpicForaComChaveEmCache_UsaAChaveConhecida(t *testing.T) {
	rk, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	f := newFakeEpic(t)
	f.publish(rsaJWK("k1", rk))
	v := newVerifier(f)
	_, err = v.Verify(context.Background(), sign(t, jwt.SigningMethodRS256, "k1", rk, validClaims()))
	require.NoError(t, err)

	// o cache vence e a Epic cai: a chave conhecida continua valendo
	f.setDown(true)
	v.now = func() time.Time { return epoch.Add(defaultKeyTTL + time.Minute) }
	claims := validClaims()
	claims["exp"] = epoch.Add(2 * defaultKeyTTL).Unix()
	_, err = v.Verify(context.Background(), sign(t, jwt.SigningMethodRS256, "k1", rk, claims))

	assert.NoError(t, err)
}

func TestNew_SemJWKSURL_UsaODaEpic(t *testing.T) {
	v := New(Config{ClientID: clientID}, nil)

	assert.Equal(t, DefaultJWKSURL, v.cfg.JWKSURL)
	assert.Equal(t, "epic", v.Provider())
}

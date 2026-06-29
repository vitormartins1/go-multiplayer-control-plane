package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
}

func TestRequireInternalKey_ProtegePrefixo(t *testing.T) {
	srv := httptest.NewServer(RequireInternalKey("segredo", "/admin")(okHandler()))
	defer srv.Close()

	cases := []struct {
		path, key string
		want      int
	}{
		{"/admin/x", "segredo", http.StatusOK},          // chave correta
		{"/admin/x", "errada", http.StatusUnauthorized}, // chave errada
		{"/admin/x", "", http.StatusUnauthorized},       // sem header
		{"/healthz", "", http.StatusOK},                 // fora do prefixo, livre
		{"/metrics", "", http.StatusOK},                 // fora do prefixo, livre
	}
	for _, c := range cases {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+c.path, nil)
		if c.key != "" {
			req.Header.Set(InternalKeyHeader, c.key)
		}
		resp, err := srv.Client().Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		assert.Equal(t, c.want, resp.StatusCode, "%s key=%q", c.path, c.key)
	}
}

func TestRequireInternalKey_ChaveVazia_NegaProtegido(t *testing.T) {
	srv := httptest.NewServer(RequireInternalKey("", "/admin")(okHandler()))
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + "/admin/x")
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "chave não configurada -> fail closed")
}

func TestInternalKeyTransport_InjetaHeader(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get(InternalKeyHeader)
	}))
	defer srv.Close()

	client := &http.Client{Transport: InternalKeyTransport("segredo", nil)}
	resp, err := client.Get(srv.URL + "/admin/x")
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, "segredo", got)
}

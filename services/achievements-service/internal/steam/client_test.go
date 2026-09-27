package steam

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSteam responde como a Web API e guarda o form recebido.
func fakeSteam(t *testing.T, status int, body string) (*httptest.Server, *map[string]string) {
	t.Helper()
	got := map[string]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/ISteamUserStats/SetUserStatsForGame/v1/", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, r.ParseForm())
		for k := range r.PostForm {
			got[k] = r.PostForm.Get(k)
		}
		w.WriteHeader(status)
		_, err := w.Write([]byte(body))
		assert.NoError(t, err)
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

func TestSetAchievement_Sucesso_MandaOFormCerto(t *testing.T) {
	srv, got := fakeSteam(t, http.StatusOK, `{"response":{"result":1}}`)
	c := New(Config{BaseURL: srv.URL, APIKey: "k", AppID: "480"}, nil)

	require.NoError(t, c.SetAchievement(context.Background(), "76561198000000042", "ACH_BOSS"))
	assert.Equal(t, map[string]string{
		"key": "k", "steamid": "76561198000000042", "appid": "480",
		"count": "1", "name[0]": "ACH_BOSS", "value[0]": "1",
	}, *got)
}

func TestSetAchievement_Falhas(t *testing.T) {
	cases := map[string]struct {
		status int
		body   string
	}{
		"resultado diferente de 1": {http.StatusOK, `{"response":{"result":8}}`},
		"chave recusada":           {http.StatusForbidden, `forbidden`},
		"steam fora":               {http.StatusServiceUnavailable, ``},
		"corpo inválido":           {http.StatusOK, `<html>`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			srv, _ := fakeSteam(t, tc.status, tc.body)
			err := New(Config{BaseURL: srv.URL, APIKey: "k", AppID: "480"}, nil).SetAchievement(context.Background(), "1", "ACH")
			assert.Error(t, err)
			assert.NotContains(t, err.Error(), "k=", "o erro não vaza a chave")
		})
	}
}

func TestNew_BaseURLPadrao(t *testing.T) {
	assert.Equal(t, DefaultBaseURL, New(Config{}, nil).cfg.BaseURL)
}

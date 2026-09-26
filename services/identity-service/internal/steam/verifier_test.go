package steam

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/identity-service/internal/domain"
)

// fakeSteam responde como a Web API da Steam; gotQuery guarda a última chamada.
func fakeSteam(t *testing.T, status int, body string, gotQuery *map[string]string) *Verifier {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/ISteamUserAuth/AuthenticateUserTicket/v1/", r.URL.Path)
		if gotQuery != nil {
			q := map[string]string{}
			for k := range r.URL.Query() {
				q[k] = r.URL.Query().Get(k)
			}
			*gotQuery = q
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return New(Config{BaseURL: srv.URL, APIKey: "k", AppID: "480", Identity: "control-plane"}, srv.Client())
}

func TestVerify_TicketValido_DevolveSteamIDEMandaParametros(t *testing.T) {
	var q map[string]string
	v := fakeSteam(t, 200, `{"response":{"params":{"result":"OK","steamid":"76561198000000042","ownersteamid":"76561198000000042","vacbanned":false,"publisherbanned":false}}}`, &q)

	got, err := v.Verify(context.Background(), "14000000abcdef")
	require.NoError(t, err)
	assert.Equal(t, domain.ExternalIdentity{Provider: "steam", ExternalID: "76561198000000042"}, got)
	assert.Equal(t, map[string]string{"key": "k", "appid": "480", "ticket": "14000000abcdef", "identity": "control-plane"}, q)
}

func TestVerify_SteamRecusaOTicket_ErrInvalidTicket(t *testing.T) {
	v := fakeSteam(t, 200, `{"response":{"error":{"errorcode":101,"errordesc":"Invalid ticket"}}}`, nil)
	_, err := v.Verify(context.Background(), "abcd")
	assert.ErrorIs(t, err, domain.ErrInvalidTicket)
}

func TestVerify_PublisherBan_ErrExternalBanned_VACNaoBloqueia(t *testing.T) {
	v := fakeSteam(t, 200, `{"response":{"params":{"result":"OK","steamid":"1","publisherbanned":true}}}`, nil)
	_, err := v.Verify(context.Background(), "abcd")
	assert.ErrorIs(t, err, domain.ErrExternalBanned)

	v = fakeSteam(t, 200, `{"response":{"params":{"result":"OK","steamid":"2","vacbanned":true}}}`, nil)
	got, err := v.Verify(context.Background(), "abcd")
	require.NoError(t, err, "VAC ban é por jogo; não bloqueia o login")
	assert.Equal(t, "2", got.ExternalID)
}

func TestVerify_SteamForaOuChaveRecusada_ErrProviderUnavailable(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusServiceUnavailable} {
		v := fakeSteam(t, status, `Forbidden`, nil)
		_, err := v.Verify(context.Background(), "abcd")
		assert.ErrorIs(t, err, domain.ErrProviderUnavailable, "status %d", status)
	}
}

func TestVerify_TicketMalformado_NaoVaiARede(t *testing.T) {
	var q map[string]string
	v := fakeSteam(t, 200, `{}`, &q)
	for _, ticket := range []string{"", "nao-e-hex", string(make([]byte, maxTicketHexLen+2))} {
		_, err := v.Verify(context.Background(), ticket)
		assert.ErrorIs(t, err, domain.ErrInvalidTicket)
	}
	assert.Nil(t, q, "nenhuma chamada à Steam")
}

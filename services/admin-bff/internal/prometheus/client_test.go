package prometheus

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-game-backend/services/admin-bff/internal/domain"
)

func TestQuery_ParseVetor_LabelsEValor(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "up", r.URL.Query().Get("query"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status":"success",
			"data":{"resultType":"vector","result":[
				{"metric":{"job":"session-service"},"value":[1719600000,"1"]},
				{"metric":{"job":"matchmaking-service"},"value":[1719600000,"0"]}
			]}
		}`))
	}))
	defer srv.Close()

	samples, err := New(srv.URL, srv.Client()).Query(context.Background(), "up")
	require.NoError(t, err)
	require.Len(t, samples, 2)
	assert.Equal(t, "session-service", samples[0].Labels["job"])
	assert.Equal(t, 1.0, samples[0].Value)
	assert.Equal(t, 0.0, samples[1].Value)
}

func TestQuery_ContadorComValorFloat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[
			{"metric":{},"value":[1719600000,"42"]}
		]}}`))
	}))
	defer srv.Close()

	samples, err := New(srv.URL, srv.Client()).Query(context.Background(), "matchmaking_matches_total")
	require.NoError(t, err)
	require.Len(t, samples, 1)
	assert.Equal(t, 42.0, samples[0].Value)
}

func TestQuery_PrometheusFora_ErrMetricsUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	url := srv.URL
	srv.Close() // derruba para simular conexão recusada

	_, err := New(url, &http.Client{}).Query(context.Background(), "up")
	assert.ErrorIs(t, err, domain.ErrMetricsUnavailable)
}

func TestQuery_HTTP500_ErrMetricsUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := New(srv.URL, srv.Client()).Query(context.Background(), "up")
	assert.ErrorIs(t, err, domain.ErrMetricsUnavailable)
}

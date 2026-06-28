package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetrics_Wrap_ContaRequisicaoPorRota(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg, "test")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	h := m.Wrap(mux)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/ping", nil))

	require.Equal(t, http.StatusNoContent, rr.Code)
	// uma série registrada (method=GET, route="GET /ping", status=204)
	assert.Equal(t, 1, testutil.CollectAndCount(m.requests))
}

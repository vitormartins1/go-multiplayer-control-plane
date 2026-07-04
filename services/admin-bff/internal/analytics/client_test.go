package analytics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-game-backend/services/admin-bff/internal/domain"
)

func TestOverview_ParseiaTotaisEJogos(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/admin/analytics/overview", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"totals":{"matches":3,"uniquePlayers":2,"games":1},
			"games":[{"gameId":"g1","matches":3,"uniquePlayers":2,"participations":5,"wins":1,"avgScore":4.5}]}`))
	}))
	defer upstream.Close()

	ov, err := New(upstream.URL, nil).Overview(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(3), ov.Totals.Matches)
	require.Len(t, ov.Games, 1)
	assert.Equal(t, "g1", ov.Games[0].GameID)
	assert.Equal(t, 4.5, ov.Games[0].AvgScore)
	assert.Empty(t, ov.Games[0].GameName, "nome é injetado pelo service, não vem do analytics")
}

func TestOverview_UpstreamFora_ErrAnalyticsUnavailable(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer upstream.Close()
	_, err := New(upstream.URL, nil).Overview(context.Background())
	assert.ErrorIs(t, err, domain.ErrAnalyticsUnavailable)
}

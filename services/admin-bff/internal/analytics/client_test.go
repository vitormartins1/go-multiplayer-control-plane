package analytics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/admin-bff/internal/domain"
)

func TestOverview_ParseiaTotaisEJogos(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/admin/analytics/overview", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"totals":{"matches":3,"uniquePlayers":2,"games":1},
			"games":[{"gameId":"g1","matches":3,"uniquePlayers":2,"participations":5,"wins":1,"losses":2,"draws":1,"avgScore":4.5}]}`))
	}))
	defer upstream.Close()

	ov, err := New(upstream.URL, nil).Overview(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(3), ov.Totals.Matches)
	require.Len(t, ov.Games, 1)
	assert.Equal(t, "g1", ov.Games[0].GameID)
	assert.Equal(t, 4.5, ov.Games[0].AvgScore)
	assert.Equal(t, int64(1), ov.Games[0].Wins)
	assert.Equal(t, int64(2), ov.Games[0].Losses)
	assert.Equal(t, int64(1), ov.Games[0].Draws)
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

func TestGameAnalytics_ParseiaRoleParticipations(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/admin/games/hw/analytics", r.URL.Path)
		assert.Equal(t, "7", r.URL.Query().Get("days"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"gameId":"hw","matches":0,"daily":[],
			"roleParticipations":[{"modeId":"batizado","role":"invader","event":"HOST_KILLED","count":3},{"role":"coop","event":"BOSS_DEFEATED","count":1}]}`))
	}))
	defer upstream.Close()

	d, err := New(upstream.URL, nil).GameAnalytics(context.Background(), "hw", 7)
	require.NoError(t, err)
	assert.Equal(t, "hw", d.GameID)
	require.Len(t, d.RoleParticipations, 2)
	assert.Equal(t, domain.ParticipationStat{ModeID: "batizado", Role: "invader", Event: "HOST_KILLED", Count: 3}, d.RoleParticipations[0])
	assert.Empty(t, d.RoleParticipations[1].ModeID, "sem modo (dado anterior à spec 009) cai no bucket vazio")
	assert.Empty(t, d.GameName, "nome é injetado pelo service")
}

// battle_royale (BR4): o campo placement do analytics é propagado no detalhe.
func TestGameAnalytics_ParseiaPlacement(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"gameId":"br","matches":1,"daily":[],"roleParticipations":[],
			"placement":{"winners":1,"topN":9,"rest":2,"entries":12,"avgSurvivedMs":6500}}`))
	}))
	defer upstream.Close()

	d, err := New(upstream.URL, nil).GameAnalytics(context.Background(), "br", 0)
	require.NoError(t, err)
	require.NotNil(t, d.Placement)
	assert.Equal(t, domain.PlacementDistribution{Winners: 1, TopN: 9, Rest: 2, Entries: 12, AvgSurvivedMs: 6500}, *d.Placement)
}

// Jogo não-BR: sem placement no upstream -> campo nil (omitido no repasse).
func TestGameAnalytics_SemPlacement_Nil(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"gameId":"qa","matches":1,"daily":[],"roleParticipations":[]}`))
	}))
	defer upstream.Close()

	d, err := New(upstream.URL, nil).GameAnalytics(context.Background(), "qa", 0)
	require.NoError(t, err)
	assert.Nil(t, d.Placement)
	assert.Nil(t, d.Mission)
}

// coop_mission (C4): o campo mission do analytics é propagado no detalhe.
func TestGameAnalytics_ParseiaMission(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"gameId":"coop","matches":3,"daily":[],"roleParticipations":[],
			"mission":{"successes":2,"failures":1,"successRate":0.6666666666666666}}`))
	}))
	defer upstream.Close()

	d, err := New(upstream.URL, nil).GameAnalytics(context.Background(), "coop", 0)
	require.NoError(t, err)
	require.NotNil(t, d.Mission)
	assert.Equal(t, int64(2), d.Mission.Successes)
	assert.Equal(t, int64(1), d.Mission.Failures)
	assert.InDelta(t, 2.0/3.0, d.Mission.SuccessRate, 0.001)
}

// séries diárias novas + stats brutos (analytics-service/010+011, spec
// admin-bff/024) — decodificam sem perder dado.
func TestGameAnalytics_ParseiaSeriesDiariasEStats(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"gameId":"g1","matches":1,"daily":[],"roleParticipations":[],
			"placementDaily":[{"day":"2026-07-13","matches":3}],
			"missionDaily":[{"day":"2026-07-13","matches":1}],
			"participationDaily":[{"day":"2026-07-13","matches":42}],
			"stats":[{"key":"kills","sum":42,"count":10,"avg":4.2}]}`))
	}))
	defer upstream.Close()

	d, err := New(upstream.URL, nil).GameAnalytics(context.Background(), "g1", 0)
	require.NoError(t, err)
	require.Len(t, d.PlacementDaily, 1)
	assert.Equal(t, int64(3), d.PlacementDaily[0].Matches)
	require.Len(t, d.MissionDaily, 1)
	assert.Equal(t, int64(1), d.MissionDaily[0].Matches)
	require.Len(t, d.ParticipationDaily, 1)
	assert.Equal(t, int64(42), d.ParticipationDaily[0].Matches)
	require.Len(t, d.Stats, 1)
	assert.Equal(t, domain.StatSummary{Key: "kills", Sum: 42, Count: 10, Avg: 4.2}, d.Stats[0])
}

// Jogo sem os campos novos no upstream (resposta antiga/sem dado) -> slices
// vazias, sem quebrar a decodificação.
func TestGameAnalytics_SemCamposNovos_ListasVazias(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"gameId":"g1","matches":1,"daily":[],"roleParticipations":[]}`))
	}))
	defer upstream.Close()

	d, err := New(upstream.URL, nil).GameAnalytics(context.Background(), "g1", 0)
	require.NoError(t, err)
	assert.Empty(t, d.PlacementDaily)
	assert.Empty(t, d.MissionDaily)
	assert.Empty(t, d.ParticipationDaily)
	assert.Empty(t, d.Stats)
}

func TestGameAnalytics_UpstreamFora_ErrAnalyticsUnavailable(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer upstream.Close()
	_, err := New(upstream.URL, nil).GameAnalytics(context.Background(), "hw", 0)
	assert.ErrorIs(t, err, domain.ErrAnalyticsUnavailable)
}

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c130n/game-backend/services/admin-bff/internal/domain"
)

// fakeMetrics responde por query (mapa promQL -> samples) ou erro global.
type fakeMetrics struct {
	byQuery map[string][]domain.Sample
	err     error
}

func (f fakeMetrics) Query(_ context.Context, promQL string) ([]domain.Sample, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.byQuery[promQL], nil
}

type fakeReader struct {
	out []domain.PlayerAchievement
	err error
	got string
}

func (f *fakeReader) ForPlayer(_ context.Context, playerID string) ([]domain.PlayerAchievement, error) {
	f.got = playerID
	return f.out, f.err
}

func TestTopology_DerivaUpDeUpJob(t *testing.T) {
	metrics := fakeMetrics{byQuery: map[string][]domain.Sample{
		"up": {
			{Labels: map[string]string{"job": "session-service"}, Value: 1},
			{Labels: map[string]string{"job": "matchmaking-service"}, Value: 0},
		},
	}}
	topo, err := New(metrics, &fakeReader{}).Topology(context.Background())
	require.NoError(t, err)
	assert.True(t, topo.SourceAvailable)

	up := map[string]*bool{}
	for _, n := range topo.Nodes {
		up[n.ID] = n.Up
	}
	require.NotNil(t, up["session-service"])
	assert.True(t, *up["session-service"])
	require.NotNil(t, up["matchmaking-service"])
	assert.False(t, *up["matchmaking-service"])
	// não raspado (sem sample) -> down (a fonte respondeu)
	require.NotNil(t, up["identity-service"])
	assert.False(t, *up["identity-service"])
}

func TestTopology_PrometheusFora_Degrada(t *testing.T) {
	metrics := fakeMetrics{err: domain.ErrMetricsUnavailable}
	topo, err := New(metrics, &fakeReader{}).Topology(context.Background())
	require.NoError(t, err) // degrada, não falha
	assert.False(t, topo.SourceAvailable)
	require.NotEmpty(t, topo.Nodes)
	for _, n := range topo.Nodes {
		assert.Nil(t, n.Up, "nó %s deve vir unknown", n.ID)
	}
	assert.NotEmpty(t, topo.Edges)
}

func TestTopology_ArestasConhecidas(t *testing.T) {
	metrics := fakeMetrics{byQuery: map[string][]domain.Sample{"up": {}}}
	topo, err := New(metrics, &fakeReader{}).Topology(context.Background())
	require.NoError(t, err)

	has := func(from, to string) bool {
		for _, e := range topo.Edges {
			if e.From == from && e.To == to {
				return true
			}
		}
		return false
	}
	assert.True(t, has("matchmaking-service", "session-service"))
	assert.True(t, has("matchmaking-service", "identity-service"))
	assert.True(t, has("achievements-service", "identity-service"))
}

func TestLiveMetrics_AgregaContadores(t *testing.T) {
	metrics := fakeMetrics{byQuery: map[string][]domain.Sample{
		"matchmaking_queue_size":           {{Value: 3}},
		"session_active_sessions":          {{Value: 2}},
		"matchmaking_matches_total":        {{Value: 10}},
		"matchmaking_matches_failed_total": {{Value: 1}},
	}}
	lm, err := New(metrics, &fakeReader{}).LiveMetrics(context.Background())
	require.NoError(t, err)
	require.NotNil(t, lm.QueueSize)
	assert.Equal(t, 3.0, *lm.QueueSize)
	assert.Equal(t, 2.0, *lm.ActiveSessions)
	assert.Equal(t, 10.0, *lm.MatchesTotal)
	assert.Equal(t, 1.0, *lm.MatchesFailed)
}

func TestLiveMetrics_MetricaAusente_CampoNil(t *testing.T) {
	metrics := fakeMetrics{byQuery: map[string][]domain.Sample{
		"matchmaking_queue_size": {{Value: 5}},
		// demais ausentes
	}}
	lm, err := New(metrics, &fakeReader{}).LiveMetrics(context.Background())
	require.NoError(t, err)
	require.NotNil(t, lm.QueueSize)
	assert.Equal(t, 5.0, *lm.QueueSize)
	assert.Nil(t, lm.ActiveSessions)
	assert.Nil(t, lm.MatchesTotal)
}

func TestLiveMetrics_PrometheusFora_PropagaErro(t *testing.T) {
	metrics := fakeMetrics{err: domain.ErrMetricsUnavailable}
	_, err := New(metrics, &fakeReader{}).LiveMetrics(context.Background())
	assert.ErrorIs(t, err, domain.ErrMetricsUnavailable)
}

func TestPlayerAchievements_DelegaAoReader(t *testing.T) {
	reader := &fakeReader{out: []domain.PlayerAchievement{{ID: "first_win", Unlocked: true}}}
	got, err := New(fakeMetrics{}, reader).PlayerAchievements(context.Background(), "player-7")
	require.NoError(t, err)
	assert.Equal(t, "player-7", reader.got)
	require.Len(t, got, 1)
	assert.Equal(t, "first_win", got[0].ID)
}

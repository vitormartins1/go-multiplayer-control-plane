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
	out       []domain.PlayerAchievement
	catalog   []domain.Achievement
	err       error
	gotGameID string
	gotPlayer string
	gotCreate domain.Achievement
}

func (f *fakeReader) CatalogForGame(_ context.Context, gameID string) ([]domain.Achievement, error) {
	f.gotGameID = gameID
	return f.catalog, f.err
}
func (f *fakeReader) CreateAchievement(_ context.Context, gameID string, a domain.Achievement) error {
	f.gotGameID, f.gotCreate = gameID, a
	return f.err
}
func (f *fakeReader) PlayerAchievements(_ context.Context, gameID, playerID string) ([]domain.PlayerAchievement, error) {
	f.gotGameID, f.gotPlayer = gameID, playerID
	return f.out, f.err
}

type fakeRegistry struct {
	games   []domain.Game
	created *domain.Game
	err     error
	gotSlug string
	gotName string
}

func (f *fakeRegistry) ListGames(_ context.Context) ([]domain.Game, error) {
	return f.games, f.err
}
func (f *fakeRegistry) CreateGame(_ context.Context, slug, name string) (*domain.Game, error) {
	f.gotSlug, f.gotName = slug, name
	if f.err != nil {
		return nil, f.err
	}
	return &domain.Game{ID: "g1", Slug: slug, Name: name}, nil
}

type fakeDirectory struct {
	page      domain.PlayerPage
	err       error
	gotLimit  int
	gotOffset int
}

func (f *fakeDirectory) ListPlayers(_ context.Context, limit, offset int) (domain.PlayerPage, error) {
	f.gotLimit, f.gotOffset = limit, offset
	return f.page, f.err
}

func TestPlayers_DelegaERepassaPaginacao(t *testing.T) {
	dir := &fakeDirectory{page: domain.PlayerPage{Total: 3, Players: []domain.Player{{ID: "a", Handle: "Alice"}}}}
	page, err := New(fakeMetrics{}, &fakeReader{}, dir, nil).Players(context.Background(), 5, 10)
	require.NoError(t, err)
	assert.Equal(t, 3, page.Total)
	require.Len(t, page.Players, 1)
	assert.Equal(t, 5, dir.gotLimit)
	assert.Equal(t, 10, dir.gotOffset)
}

func TestTopology_DerivaUpDeUpJob(t *testing.T) {
	metrics := fakeMetrics{byQuery: map[string][]domain.Sample{
		"up": {
			{Labels: map[string]string{"job": "session-service"}, Value: 1},
			{Labels: map[string]string{"job": "matchmaking-service"}, Value: 0},
		},
	}}
	topo, err := New(metrics, &fakeReader{}, nil, nil).Topology(context.Background())
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
	topo, err := New(metrics, &fakeReader{}, nil, nil).Topology(context.Background())
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
	topo, err := New(metrics, &fakeReader{}, nil, nil).Topology(context.Background())
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
	lm, err := New(metrics, &fakeReader{}, nil, nil).LiveMetrics(context.Background())
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
	lm, err := New(metrics, &fakeReader{}, nil, nil).LiveMetrics(context.Background())
	require.NoError(t, err)
	require.NotNil(t, lm.QueueSize)
	assert.Equal(t, 5.0, *lm.QueueSize)
	assert.Nil(t, lm.ActiveSessions)
	assert.Nil(t, lm.MatchesTotal)
}

func TestLiveMetrics_PrometheusFora_PropagaErro(t *testing.T) {
	metrics := fakeMetrics{err: domain.ErrMetricsUnavailable}
	_, err := New(metrics, &fakeReader{}, nil, nil).LiveMetrics(context.Background())
	assert.ErrorIs(t, err, domain.ErrMetricsUnavailable)
}

func TestServiceMetrics_REST_UpRedRuntime(t *testing.T) {
	id := "identity-service"
	metrics := fakeMetrics{byQuery: map[string][]domain.Sample{
		`up{job="identity-service"}`:                                     {{Value: 1}},
		`sum(rate(http_requests_total{service="identity-service"}[5m]))`: {{Value: 12.5}},
		`sum(rate(http_requests_total{service="identity-service",status=~"5.."}[5m])) / sum(rate(http_requests_total{service="identity-service"}[5m]))`: {{Value: 0.02}},
		`histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket{service="identity-service"}[5m])) by (le))`:                             {{Value: 0.123}},
		`go_goroutines{job="identity-service"}`:                 {{Value: 42}},
		`process_resident_memory_bytes{job="identity-service"}`: {{Value: 1048576}},
	}}
	sm, err := New(metrics, &fakeReader{}, nil, nil).ServiceMetrics(context.Background(), id)
	require.NoError(t, err)
	require.NotNil(t, sm.Up)
	assert.True(t, *sm.Up)
	require.NotNil(t, sm.RequestRate)
	assert.Equal(t, 12.5, *sm.RequestRate)
	require.NotNil(t, sm.ErrorRate)
	assert.Equal(t, 0.02, *sm.ErrorRate)
	require.NotNil(t, sm.LatencyP95)
	assert.Equal(t, 0.123, *sm.LatencyP95)
	require.NotNil(t, sm.Goroutines)
	assert.Equal(t, 42.0, *sm.Goroutines)
	require.NotNil(t, sm.MemoryBytes)
	assert.Equal(t, 1048576.0, *sm.MemoryBytes)
}

func TestServiceMetrics_GRPC_SemRED_NilButRuntime(t *testing.T) {
	// session-service (gRPC) não expõe http_requests_total -> RED nil; up+runtime presentes.
	metrics := fakeMetrics{byQuery: map[string][]domain.Sample{
		`up{job="session-service"}`:                            {{Value: 1}},
		`go_goroutines{job="session-service"}`:                 {{Value: 8}},
		`process_resident_memory_bytes{job="session-service"}`: {{Value: 2048}},
	}}
	sm, err := New(metrics, &fakeReader{}, nil, nil).ServiceMetrics(context.Background(), "session-service")
	require.NoError(t, err)
	require.NotNil(t, sm.Up)
	assert.True(t, *sm.Up)
	assert.Nil(t, sm.RequestRate, "gRPC não tem RED HTTP")
	assert.Nil(t, sm.ErrorRate)
	assert.Nil(t, sm.LatencyP95)
	require.NotNil(t, sm.Goroutines)
	assert.Equal(t, 8.0, *sm.Goroutines)
}

func TestServiceMetrics_DownSemRuntime(t *testing.T) {
	metrics := fakeMetrics{byQuery: map[string][]domain.Sample{
		`up{job="player-data-service"}`: {{Value: 0}},
	}}
	sm, err := New(metrics, &fakeReader{}, nil, nil).ServiceMetrics(context.Background(), "player-data-service")
	require.NoError(t, err)
	require.NotNil(t, sm.Up)
	assert.False(t, *sm.Up)
	assert.Nil(t, sm.MemoryBytes)
}

func TestServiceMetrics_IdDesconhecido_ErrUnknownService(t *testing.T) {
	_, err := New(fakeMetrics{}, &fakeReader{}, nil, nil).ServiceMetrics(context.Background(), "nao-existe")
	assert.ErrorIs(t, err, domain.ErrUnknownService)
}

func TestServiceMetrics_PrometheusFora_PropagaErro(t *testing.T) {
	metrics := fakeMetrics{err: domain.ErrMetricsUnavailable}
	_, err := New(metrics, &fakeReader{}, nil, nil).ServiceMetrics(context.Background(), "identity-service")
	assert.ErrorIs(t, err, domain.ErrMetricsUnavailable)
}

func TestPlayerAchievements_DelegaComJogoEPlayer(t *testing.T) {
	reader := &fakeReader{out: []domain.PlayerAchievement{{ID: "first_win", Unlocked: true}}}
	got, err := New(fakeMetrics{}, reader, nil, nil).PlayerAchievements(context.Background(), "game-9", "player-7")
	require.NoError(t, err)
	assert.Equal(t, "game-9", reader.gotGameID)
	assert.Equal(t, "player-7", reader.gotPlayer)
	require.Len(t, got, 1)
	assert.Equal(t, "first_win", got[0].ID)
}

func TestGames_DelegaAoRegistry(t *testing.T) {
	reg := &fakeRegistry{games: []domain.Game{{ID: "g1", Slug: "tetris"}}}
	got, err := New(fakeMetrics{}, &fakeReader{}, nil, reg).Games(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "tetris", got[0].Slug)
}

func TestCreateGame_DelegaAoRegistry(t *testing.T) {
	reg := &fakeRegistry{}
	g, err := New(fakeMetrics{}, &fakeReader{}, nil, reg).CreateGame(context.Background(), "tetris", "Tetris")
	require.NoError(t, err)
	assert.Equal(t, "tetris", reg.gotSlug)
	assert.Equal(t, "g1", g.ID)
}

func TestGameCatalogECreateAchievement_DelegamAoReader(t *testing.T) {
	reader := &fakeReader{catalog: []domain.Achievement{{ID: "first_win"}}}
	svc := New(fakeMetrics{}, reader, nil, nil)

	cat, err := svc.GameCatalog(context.Background(), "game-9")
	require.NoError(t, err)
	require.Len(t, cat, 1)
	assert.Equal(t, "game-9", reader.gotGameID)

	err = svc.CreateAchievement(context.Background(), "game-9", domain.Achievement{ID: "speedrun", EventType: "fast_win"})
	require.NoError(t, err)
	assert.Equal(t, "speedrun", reader.gotCreate.ID)
}

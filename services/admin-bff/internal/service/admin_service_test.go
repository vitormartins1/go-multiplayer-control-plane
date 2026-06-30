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
	games     []domain.Game
	created   *domain.Game
	players   []domain.GamePlayer
	err       error
	gotSlug   string
	gotName   string
	gotGameID string
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
func (f *fakeRegistry) PlayersOf(_ context.Context, gameID string) ([]domain.GamePlayer, error) {
	f.gotGameID = gameID
	return f.players, f.err
}

type fakeLeaderboard struct {
	entries   []domain.LeaderboardEntry
	err       error
	gotGameID string
	gotLimit  int
}

func (f *fakeLeaderboard) TopForGame(_ context.Context, gameID string, limit int) ([]domain.LeaderboardEntry, error) {
	f.gotGameID, f.gotLimit = gameID, limit
	return f.entries, f.err
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

type fakePlayerData struct {
	entries   []domain.PlayerDataEntry
	err       error
	gotGameID string
	gotPlayer string
}

func (f *fakePlayerData) ForPlayer(_ context.Context, gameID, playerID string) ([]domain.PlayerDataEntry, error) {
	f.gotGameID, f.gotPlayer = gameID, playerID
	return f.entries, f.err
}

type fakeCloudSave struct {
	save      domain.CloudSave
	err       error
	gotGameID string
	gotPlayer string
}

func (f *fakeCloudSave) ForPlayer(_ context.Context, gameID, playerID string) (domain.CloudSave, error) {
	f.gotGameID, f.gotPlayer = gameID, playerID
	return f.save, f.err
}

func TestPlayers_DelegaERepassaPaginacao(t *testing.T) {
	dir := &fakeDirectory{page: domain.PlayerPage{Total: 3, Players: []domain.Player{{ID: "a", Handle: "Alice"}}}}
	page, err := New(Deps{Metrics: fakeMetrics{}, Reader: &fakeReader{}, Directory: dir}).Players(context.Background(), 5, 10)
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
	topo, err := New(Deps{Metrics: metrics, Reader: &fakeReader{}}).Topology(context.Background())
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
	topo, err := New(Deps{Metrics: metrics, Reader: &fakeReader{}}).Topology(context.Background())
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
	topo, err := New(Deps{Metrics: metrics, Reader: &fakeReader{}}).Topology(context.Background())
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
	lm, err := New(Deps{Metrics: metrics, Reader: &fakeReader{}}).LiveMetrics(context.Background())
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
	lm, err := New(Deps{Metrics: metrics, Reader: &fakeReader{}}).LiveMetrics(context.Background())
	require.NoError(t, err)
	require.NotNil(t, lm.QueueSize)
	assert.Equal(t, 5.0, *lm.QueueSize)
	assert.Nil(t, lm.ActiveSessions)
	assert.Nil(t, lm.MatchesTotal)
}

func TestLiveMetrics_PrometheusFora_PropagaErro(t *testing.T) {
	metrics := fakeMetrics{err: domain.ErrMetricsUnavailable}
	_, err := New(Deps{Metrics: metrics, Reader: &fakeReader{}}).LiveMetrics(context.Background())
	assert.ErrorIs(t, err, domain.ErrMetricsUnavailable)
}

func TestPlayerAchievements_DelegaComJogoEPlayer(t *testing.T) {
	reader := &fakeReader{out: []domain.PlayerAchievement{{ID: "first_win", Unlocked: true}}}
	got, err := New(Deps{Metrics: fakeMetrics{}, Reader: reader}).PlayerAchievements(context.Background(), "game-9", "player-7")
	require.NoError(t, err)
	assert.Equal(t, "game-9", reader.gotGameID)
	assert.Equal(t, "player-7", reader.gotPlayer)
	require.Len(t, got, 1)
	assert.Equal(t, "first_win", got[0].ID)
}

func TestGames_DelegaAoRegistry(t *testing.T) {
	reg := &fakeRegistry{games: []domain.Game{{ID: "g1", Slug: "tetris"}}}
	got, err := New(Deps{Metrics: fakeMetrics{}, Reader: &fakeReader{}, Registry: reg}).Games(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "tetris", got[0].Slug)
}

func TestCreateGame_DelegaAoRegistry(t *testing.T) {
	reg := &fakeRegistry{}
	g, err := New(Deps{Metrics: fakeMetrics{}, Reader: &fakeReader{}, Registry: reg}).CreateGame(context.Background(), "tetris", "Tetris")
	require.NoError(t, err)
	assert.Equal(t, "tetris", reg.gotSlug)
	assert.Equal(t, "g1", g.ID)
}

func TestGameCatalogECreateAchievement_DelegamAoReader(t *testing.T) {
	reader := &fakeReader{catalog: []domain.Achievement{{ID: "first_win"}}}
	svc := New(Deps{Metrics: fakeMetrics{}, Reader: reader})

	cat, err := svc.GameCatalog(context.Background(), "game-9")
	require.NoError(t, err)
	require.Len(t, cat, 1)
	assert.Equal(t, "game-9", reader.gotGameID)

	err = svc.CreateAchievement(context.Background(), "game-9", domain.Achievement{ID: "speedrun", EventType: "fast_win"})
	require.NoError(t, err)
	assert.Equal(t, "speedrun", reader.gotCreate.ID)
}

func TestGameLeaderboard_DelegaAoLeaderboard(t *testing.T) {
	lb := &fakeLeaderboard{entries: []domain.LeaderboardEntry{{Rank: 1, PlayerID: "bob", Score: 300}}}
	got, err := New(Deps{Metrics: fakeMetrics{}, Reader: &fakeReader{}, Leaderboard: lb}).GameLeaderboard(context.Background(), "game-9", 10)
	require.NoError(t, err)
	assert.Equal(t, "game-9", lb.gotGameID)
	assert.Equal(t, 10, lb.gotLimit)
	require.Len(t, got, 1)
	assert.Equal(t, "bob", got[0].PlayerID)
}

func TestGamePlayers_DelegaAoRegistry(t *testing.T) {
	reg := &fakeRegistry{players: []domain.GamePlayer{{PlayerID: "p1", Handle: "Alice"}}}
	got, err := New(Deps{Metrics: fakeMetrics{}, Reader: &fakeReader{}, Registry: reg}).GamePlayers(context.Background(), "game-9")
	require.NoError(t, err)
	assert.Equal(t, "game-9", reg.gotGameID)
	require.Len(t, got, 1)
	assert.Equal(t, "Alice", got[0].Handle)
}

func TestPlayerData_DelegaComJogoEPlayer(t *testing.T) {
	pd := &fakePlayerData{entries: []domain.PlayerDataEntry{{Namespace: "inventory", Key: "gold", Value: []byte(`100`)}}}
	got, err := New(Deps{Metrics: fakeMetrics{}, Reader: &fakeReader{}, PlayerData: pd}).PlayerData(context.Background(), "game-9", "player-7")
	require.NoError(t, err)
	assert.Equal(t, "game-9", pd.gotGameID)
	assert.Equal(t, "player-7", pd.gotPlayer)
	require.Len(t, got, 1)
	assert.Equal(t, "gold", got[0].Key)
}

func TestCloudSave_DelegaComJogoEPlayer(t *testing.T) {
	cs := &fakeCloudSave{save: domain.CloudSave{ContentType: "application/json", Size: 9, Blob: "eyJocCI6NTB9"}}
	got, err := New(Deps{Metrics: fakeMetrics{}, Reader: &fakeReader{}, CloudSave: cs}).CloudSave(context.Background(), "game-9", "player-7")
	require.NoError(t, err)
	assert.Equal(t, "game-9", cs.gotGameID)
	assert.Equal(t, "player-7", cs.gotPlayer)
	assert.Equal(t, 9, got.Size)
	assert.Equal(t, "eyJocCI6NTB9", got.Blob)
}

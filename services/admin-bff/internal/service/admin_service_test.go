package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-game-backend/services/admin-bff/internal/domain"
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
	gotID     string
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
func (f *fakeReader) UpdateAchievement(_ context.Context, gameID, id string, a domain.Achievement) error {
	f.gotGameID, f.gotID, f.gotCreate = gameID, id, a
	return f.err
}
func (f *fakeReader) DeleteAchievement(_ context.Context, gameID, id string) error {
	f.gotGameID, f.gotID = gameID, id
	return f.err
}
func (f *fakeReader) PlayerAchievements(_ context.Context, gameID, playerID string) ([]domain.PlayerAchievement, error) {
	f.gotGameID, f.gotPlayer = gameID, playerID
	return f.out, f.err
}
func (f *fakeReader) ImportAchievements(_ context.Context, gameID string, _ []domain.Achievement) (domain.ImportResponse, error) {
	f.gotGameID = gameID
	return domain.ImportResponse{}, f.err
}

type fakeRegistry struct {
	games       []domain.Game
	players     []domain.GamePlayer
	err         error
	gotSlug     string
	gotName     string
	gotGameID   string
	gotPlayerID string
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
func (f *fakeRegistry) GamesOfPlayer(_ context.Context, playerID string) ([]domain.Game, error) {
	f.gotPlayerID = playerID
	return f.games, f.err
}
func (f *fakeRegistry) RenameGame(_ context.Context, id, name string) (*domain.Game, error) {
	f.gotGameID, f.gotName = id, name
	if f.err != nil {
		return nil, f.err
	}
	return &domain.Game{ID: id, Name: name}, nil
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

func TestPlayerGames_DelegaAoRegistry(t *testing.T) {
	reg := &fakeRegistry{games: []domain.Game{{ID: "g1", Slug: "tetris", Name: "Tetris"}}}
	got, err := New(Deps{Metrics: fakeMetrics{}, Reader: &fakeReader{}, Registry: reg}).PlayerGames(context.Background(), "p1")
	require.NoError(t, err)
	assert.Equal(t, "p1", reg.gotPlayerID)
	require.Len(t, got, 1)
	assert.Equal(t, "tetris", got[0].Slug)
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

type fakeSessions struct {
	sessions  domain.GameSessions
	err       error
	gotGameID string
	gotState  string
}

func (f *fakeSessions) SessionsByGame(_ context.Context, gameID, state string) (domain.GameSessions, error) {
	f.gotGameID, f.gotState = gameID, state
	return f.sessions, f.err
}

func (f *fakeSessions) AllSessions(_ context.Context, state string) (domain.GameSessions, error) {
	f.gotState = state
	return f.sessions, f.err
}

func (f *fakeSessions) CloseSession(_ context.Context, matchID string) (domain.GameSession, error) {
	f.gotGameID = matchID // reaproveita o campo p/ capturar o matchID
	if f.err != nil {
		return domain.GameSession{}, f.err
	}
	return domain.GameSession{MatchID: matchID, State: "closed", CloseReason: "admin"}, nil
}

func TestRooms_EnriqueceHandleENomeDoJogo(t *testing.T) {
	sessions := &fakeSessions{sessions: domain.GameSessions{
		Active: []domain.GameSession{{SessionID: "s1", GameID: "g1", Players: []domain.SessionPlayer{{ID: "p1"}}}},
	}}
	dir := &fakeDirectory{page: domain.PlayerPage{Total: 1, Players: []domain.Player{{ID: "p1", Handle: "alice"}}}}
	reg := &fakeRegistry{games: []domain.Game{{ID: "g1", Name: "Orb Rush"}}}
	svc := New(Deps{Sessions: sessions, Directory: dir, Registry: reg})

	got, err := svc.Rooms(context.Background(), "open")

	require.NoError(t, err)
	assert.Equal(t, "open", sessions.gotState)
	assert.Equal(t, "Orb Rush", got.Active[0].GameName, "board global traz o nome do jogo")
	assert.Equal(t, "alice", got.Active[0].Players[0].Handle)
}

func TestRooms_RegistryFora_SemNome_SemErro(t *testing.T) {
	sessions := &fakeSessions{sessions: domain.GameSessions{
		Active: []domain.GameSession{{SessionID: "s1", GameID: "g1", Players: []domain.SessionPlayer{{ID: "p1"}}}},
	}}
	reg := &fakeRegistry{err: domain.ErrGameRegistryUnavailable}
	svc := New(Deps{Sessions: sessions, Directory: &fakeDirectory{}, Registry: reg})

	got, err := svc.Rooms(context.Background(), "")

	require.NoError(t, err, "registry fora não derruba o board (best-effort)")
	assert.Equal(t, "", got.Active[0].GameName)
	assert.Equal(t, "g1", got.Active[0].GameID, "gameId sempre presente")
}

func TestFanoutHealth_AgregaMetricas_AusenteViraNil(t *testing.T) {
	m := &fakeMetrics{byQuery: map[string][]domain.Sample{
		`sum(fanout_publish_total{result="ok"})`:    {{Value: 10}},
		`sum(fanout_consume_total{result="error"})`: {{Value: 2}},
	}}
	svc := New(Deps{Metrics: m})

	fh, err := svc.FanoutHealth(context.Background())

	require.NoError(t, err)
	require.NotNil(t, fh.PublishOK)
	assert.Equal(t, float64(10), *fh.PublishOK)
	require.NotNil(t, fh.ConsumeError)
	assert.Equal(t, float64(2), *fh.ConsumeError)
	assert.Nil(t, fh.ConsumeOK, "métrica ausente -> nil (não zero)")
}

func TestGameSessions_EnriqueceHandlesBestEffort(t *testing.T) {
	sessions := &fakeSessions{sessions: domain.GameSessions{
		Active: []domain.GameSession{{SessionID: "s1", Players: []domain.SessionPlayer{{ID: "p1"}, {ID: "p-desconhecido"}}}},
		Closed: []domain.GameSession{{SessionID: "s0", Players: []domain.SessionPlayer{{ID: "p2"}}}},
	}}
	dir := &fakeDirectory{page: domain.PlayerPage{Total: 2, Players: []domain.Player{
		{ID: "p1", Handle: "alice"}, {ID: "p2", Handle: "bob"},
	}}}
	svc := New(Deps{Sessions: sessions, Directory: dir})

	got, err := svc.GameSessions(context.Background(), "g1", "open")

	require.NoError(t, err)
	assert.Equal(t, "g1", sessions.gotGameID)
	assert.Equal(t, "open", sessions.gotState)
	assert.Equal(t, "alice", got.Active[0].Players[0].Handle)
	assert.Equal(t, "", got.Active[0].Players[1].Handle, "id sem cadastro fica com handle vazio")
	assert.Equal(t, "bob", got.Closed[0].Players[0].Handle)
}

func TestGameSessions_IdentityFora_HandlesVazios_SemErro(t *testing.T) {
	sessions := &fakeSessions{sessions: domain.GameSessions{
		Active: []domain.GameSession{{SessionID: "s1", Players: []domain.SessionPlayer{{ID: "p1"}}}},
	}}
	dir := &fakeDirectory{err: domain.ErrIdentityUnavailable}
	svc := New(Deps{Sessions: sessions, Directory: dir})

	got, err := svc.GameSessions(context.Background(), "g1", "")

	require.NoError(t, err, "identity fora não derruba a resposta (best-effort)")
	assert.Equal(t, "p1", got.Active[0].Players[0].ID, "id sempre presente")
	assert.Equal(t, "", got.Active[0].Players[0].Handle)
}

func TestGameSessions_SessionFora_PropagaErro(t *testing.T) {
	sessions := &fakeSessions{err: domain.ErrSessionsUnavailable}
	svc := New(Deps{Sessions: sessions, Directory: &fakeDirectory{}})

	_, err := svc.GameSessions(context.Background(), "g1", "")
	assert.ErrorIs(t, err, domain.ErrSessionsUnavailable)
}

type fakeAnalytics struct {
	ov  domain.AnalyticsOverview
	err error
}

func (f *fakeAnalytics) Overview(context.Context) (domain.AnalyticsOverview, error) {
	return f.ov, f.err
}

func TestAnalyticsOverview_InjetaNomeDoJogo(t *testing.T) {
	an := &fakeAnalytics{ov: domain.AnalyticsOverview{
		Totals: domain.AnalyticsTotals{Matches: 3, Games: 1},
		Games:  []domain.AnalyticsGameStat{{GameID: "g1", Matches: 3}},
	}}
	reg := &fakeRegistry{games: []domain.Game{{ID: "g1", Name: "Orb Rush"}}}
	svc := New(Deps{Analytics: an, Registry: reg})

	ov, err := svc.AnalyticsOverview(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "Orb Rush", ov.Games[0].GameName)
}

func TestAnalyticsOverview_AnalyticsFora_PropagaErro(t *testing.T) {
	an := &fakeAnalytics{err: domain.ErrAnalyticsUnavailable}
	svc := New(Deps{Analytics: an, Registry: &fakeRegistry{}})
	_, err := svc.AnalyticsOverview(context.Background())
	assert.ErrorIs(t, err, domain.ErrAnalyticsUnavailable)
}

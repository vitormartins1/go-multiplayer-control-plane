package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-game-backend/services/admin-bff/internal/domain"
)

type fakeService struct {
	topo      domain.Topology
	topoErr   error
	live      domain.LiveMetrics
	liveErr   error
	page      domain.PlayerPage
	pageErr   error
	games     []domain.Game
	gamesErr  error
	createErr error
	catalog   []domain.Achievement
	catErr    error
	ach       []domain.PlayerAchievement
	achErr    error
	leaders   []domain.LeaderboardEntry
	leadErr   error
	pdata     []domain.PlayerDataEntry
	pdataErr  error
	save      domain.CloudSave
	saveErr   error
	gplayers  []domain.GamePlayer
	gplayErr  error
	pgames    []domain.Game
	pgamesErr error
	gotGameID string
	gotPlayer string
	gotLimit  int
	gotOffset int
}

func (f *fakeService) Topology(context.Context) (domain.Topology, error) {
	return f.topo, f.topoErr
}
func (f *fakeService) LiveMetrics(context.Context) (domain.LiveMetrics, error) {
	return f.live, f.liveErr
}
func (f *fakeService) Players(_ context.Context, limit, offset int) (domain.PlayerPage, error) {
	f.gotLimit, f.gotOffset = limit, offset
	return f.page, f.pageErr
}
func (f *fakeService) Games(context.Context) ([]domain.Game, error) {
	return f.games, f.gamesErr
}
func (f *fakeService) CreateGame(_ context.Context, slug, name string) (*domain.Game, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	return &domain.Game{ID: "g1", Slug: slug, Name: name}, nil
}
func (f *fakeService) GameCatalog(_ context.Context, gameID string) ([]domain.Achievement, error) {
	f.gotGameID = gameID
	return f.catalog, f.catErr
}
func (f *fakeService) CreateAchievement(_ context.Context, gameID string, _ domain.Achievement) error {
	f.gotGameID = gameID
	return f.createErr
}
func (f *fakeService) UpdateAchievement(_ context.Context, gameID, id string, _ domain.Achievement) error {
	f.gotGameID, f.gotPlayer = gameID, id
	return f.createErr
}
func (f *fakeService) DeleteAchievement(_ context.Context, gameID, id string) error {
	f.gotGameID, f.gotPlayer = gameID, id
	return f.createErr
}
func (f *fakeService) PlayerAchievements(_ context.Context, gameID, playerID string) ([]domain.PlayerAchievement, error) {
	f.gotGameID, f.gotPlayer = gameID, playerID
	return f.ach, f.achErr
}
func (f *fakeService) GameLeaderboard(_ context.Context, gameID string, limit int) ([]domain.LeaderboardEntry, error) {
	f.gotGameID, f.gotLimit = gameID, limit
	return f.leaders, f.leadErr
}
func (f *fakeService) PlayerData(_ context.Context, gameID, playerID string) ([]domain.PlayerDataEntry, error) {
	f.gotGameID, f.gotPlayer = gameID, playerID
	return f.pdata, f.pdataErr
}
func (f *fakeService) CloudSave(_ context.Context, gameID, playerID string) (domain.CloudSave, error) {
	f.gotGameID, f.gotPlayer = gameID, playerID
	return f.save, f.saveErr
}
func (f *fakeService) GamePlayers(_ context.Context, gameID string) ([]domain.GamePlayer, error) {
	f.gotGameID = gameID
	return f.gplayers, f.gplayErr
}
func (f *fakeService) PlayerGames(_ context.Context, playerID string) ([]domain.Game, error) {
	f.gotPlayer = playerID
	return f.pgames, f.pgamesErr
}

func newServer(svc AdminService) *httptest.Server {
	mux := http.NewServeMux()
	NewHandler(svc).Register(mux)
	return httptest.NewServer(mux)
}

func get(t *testing.T, srv *httptest.Server, path string) *http.Response {
	t.Helper()
	resp, err := srv.Client().Get(srv.URL + path)
	require.NoError(t, err)
	return resp
}

func TestTopology_200(t *testing.T) {
	up := true
	svc := &fakeService{topo: domain.Topology{
		Nodes:           []domain.ServiceNode{{ID: "session-service", Label: "Session", Up: &up}},
		Edges:           []domain.Edge{{From: "matchmaking-service", To: "session-service"}},
		SourceAvailable: true,
	}}
	srv := newServer(svc)
	defer srv.Close()

	resp := get(t, srv, "/api/topology")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body domain.Topology
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.True(t, body.SourceAvailable)
	require.Len(t, body.Nodes, 1)
	require.NotNil(t, body.Nodes[0].Up)
	assert.True(t, *body.Nodes[0].Up)
}

func TestTopology_Degradada_200(t *testing.T) {
	svc := &fakeService{topo: domain.Topology{
		Nodes:           []domain.ServiceNode{{ID: "session-service", Label: "Session", Up: nil}},
		SourceAvailable: false,
	}}
	srv := newServer(svc)
	defer srv.Close()

	resp := get(t, srv, "/api/topology")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode) // degrada, não falha

	var body domain.Topology
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.False(t, body.SourceAvailable)
	assert.Nil(t, body.Nodes[0].Up)
}

func TestLiveMetrics_200(t *testing.T) {
	q := 3.0
	svc := &fakeService{live: domain.LiveMetrics{QueueSize: &q}}
	srv := newServer(svc)
	defer srv.Close()

	resp := get(t, srv, "/api/metrics/live")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body domain.LiveMetrics
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.NotNil(t, body.QueueSize)
	assert.Equal(t, 3.0, *body.QueueSize)
}

func TestLiveMetrics_FonteFora_503(t *testing.T) {
	svc := &fakeService{liveErr: domain.ErrMetricsUnavailable}
	srv := newServer(svc)
	defer srv.Close()

	resp := get(t, srv, "/api/metrics/live")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestPlayers_200_PageETotal(t *testing.T) {
	svc := &fakeService{page: domain.PlayerPage{Total: 7, Players: []domain.Player{{ID: "id-1", Handle: "Alice"}}}}
	srv := newServer(svc)
	defer srv.Close()

	resp := get(t, srv, "/api/players?limit=5&offset=10")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 5, svc.gotLimit)
	assert.Equal(t, 10, svc.gotOffset)

	var body domain.PlayerPage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, 7, body.Total)
	require.Len(t, body.Players, 1)
	assert.Equal(t, "Alice", body.Players[0].Handle)
}

func TestPlayers_IdentityFora_502(t *testing.T) {
	svc := &fakeService{pageErr: domain.ErrIdentityUnavailable}
	srv := newServer(svc)
	defer srv.Close()

	resp := get(t, srv, "/api/players")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
}

func post(t *testing.T, srv *httptest.Server, path, body string) *http.Response {
	t.Helper()
	resp, err := srv.Client().Post(srv.URL+path, "application/json", strings.NewReader(body))
	require.NoError(t, err)
	return resp
}

func TestGames_200(t *testing.T) {
	svc := &fakeService{games: []domain.Game{{ID: "g1", Slug: "tetris", Name: "Tetris"}}}
	srv := newServer(svc)
	defer srv.Close()

	resp := get(t, srv, "/api/games")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var body []domain.Game
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Len(t, body, 1)
	assert.Equal(t, "tetris", body[0].Slug)
}

func TestCreateGame_201(t *testing.T) {
	srv := newServer(&fakeService{})
	defer srv.Close()
	resp := post(t, srv, "/api/games", `{"slug":"tetris","name":"Tetris"}`)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestCreateGame_RepassaUpstream409(t *testing.T) {
	svc := &fakeService{createErr: &domain.UpstreamError{Status: http.StatusConflict, Code: "slug_taken"}}
	srv := newServer(svc)
	defer srv.Close()
	resp := post(t, srv, "/api/games", `{"slug":"tetris","name":"Tetris"}`)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestCreateGame_RegistryFora_502(t *testing.T) {
	svc := &fakeService{createErr: domain.ErrGameRegistryUnavailable}
	srv := newServer(svc)
	defer srv.Close()
	resp := post(t, srv, "/api/games", `{"slug":"tetris","name":"Tetris"}`)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
}

func TestGameCatalog_200(t *testing.T) {
	svc := &fakeService{catalog: []domain.Achievement{{ID: "first_win"}}}
	srv := newServer(svc)
	defer srv.Close()
	resp := get(t, srv, "/api/games/game-9/achievements")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "game-9", svc.gotGameID)
}

func TestCreateAchievement_RepassaUpstream400(t *testing.T) {
	svc := &fakeService{createErr: &domain.UpstreamError{Status: http.StatusBadRequest, Code: "invalid_achievement"}}
	srv := newServer(svc)
	defer srv.Close()
	resp := post(t, srv, "/api/games/g/achievements", `{"id":"X","name":"n","eventType":"kill","target":1}`)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestUpdateAchievement_200(t *testing.T) {
	svc := &fakeService{}
	srv := newServer(svc)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/games/game-9/achievements/first_win", strings.NewReader(`{"name":"N","eventType":"e","target":2}`))
	req.Header.Set("Content-Type", "application/json")
	r2, err := srv.Client().Do(req)
	require.NoError(t, err)
	defer r2.Body.Close()
	require.Equal(t, http.StatusOK, r2.StatusCode)
	assert.Equal(t, "game-9", svc.gotGameID)
	assert.Equal(t, "first_win", svc.gotPlayer)
}

func TestUpdateAchievement_RepassaUpstream404(t *testing.T) {
	svc := &fakeService{createErr: &domain.UpstreamError{Status: http.StatusNotFound, Code: "achievement_not_found"}}
	srv := newServer(svc)
	defer srv.Close()
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/games/g/achievements/x", strings.NewReader(`{"name":"N","eventType":"e","target":1}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestDeleteAchievement_204(t *testing.T) {
	svc := &fakeService{}
	srv := newServer(svc)
	defer srv.Close()
	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/games/game-9/achievements/first_win", nil)
	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Equal(t, "first_win", svc.gotPlayer)
}

func TestDeleteAchievement_RepassaUpstream404(t *testing.T) {
	svc := &fakeService{createErr: &domain.UpstreamError{Status: http.StatusNotFound, Code: "achievement_not_found"}}
	srv := newServer(svc)
	defer srv.Close()
	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/games/g/achievements/x", nil)
	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestPlayerAchievements_PorJogo_200(t *testing.T) {
	svc := &fakeService{ach: []domain.PlayerAchievement{{ID: "first_win", Unlocked: true}}}
	srv := newServer(svc)
	defer srv.Close()

	resp := get(t, srv, "/api/games/game-9/players/player-7/achievements")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "game-9", svc.gotGameID)
	assert.Equal(t, "player-7", svc.gotPlayer)
}

func TestPlayerAchievements_FonteFora_502(t *testing.T) {
	svc := &fakeService{achErr: domain.ErrAchievementsUnavailable}
	srv := newServer(svc)
	defer srv.Close()

	resp := get(t, srv, "/api/games/g/players/x/achievements")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
}

func TestGameLeaderboard_200(t *testing.T) {
	svc := &fakeService{leaders: []domain.LeaderboardEntry{{Rank: 1, PlayerID: "bob", Score: 300}}}
	srv := newServer(svc)
	defer srv.Close()

	resp := get(t, srv, "/api/games/game-9/leaderboard?limit=5")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "game-9", svc.gotGameID)
	assert.Equal(t, 5, svc.gotLimit)

	var body []domain.LeaderboardEntry
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Len(t, body, 1)
	assert.Equal(t, "bob", body[0].PlayerID)
}

func TestGameLeaderboard_Fora_502(t *testing.T) {
	svc := &fakeService{leadErr: domain.ErrLeaderboardUnavailable}
	srv := newServer(svc)
	defer srv.Close()

	resp := get(t, srv, "/api/games/game-9/leaderboard")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
}

func TestGamePlayers_200(t *testing.T) {
	svc := &fakeService{gplayers: []domain.GamePlayer{{PlayerID: "p1", Handle: "Alice"}}}
	srv := newServer(svc)
	defer srv.Close()

	resp := get(t, srv, "/api/games/game-9/players")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "game-9", svc.gotGameID)

	var body []domain.GamePlayer
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Len(t, body, 1)
	assert.Equal(t, "Alice", body[0].Handle)
}

func TestPlayerGames_200(t *testing.T) {
	svc := &fakeService{pgames: []domain.Game{{ID: "g1", Slug: "tetris", Name: "Tetris"}}}
	srv := newServer(svc)
	defer srv.Close()

	resp := get(t, srv, "/api/players/p1/games")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "p1", svc.gotPlayer)

	var body []domain.Game
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Len(t, body, 1)
	assert.Equal(t, "tetris", body[0].Slug)
}

func TestPlayerGames_Fora_502(t *testing.T) {
	svc := &fakeService{pgamesErr: domain.ErrGameRegistryUnavailable}
	srv := newServer(svc)
	defer srv.Close()

	resp := get(t, srv, "/api/players/p1/games")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
}

func TestGamePlayers_Fora_502(t *testing.T) {
	svc := &fakeService{gplayErr: domain.ErrGameRegistryUnavailable}
	srv := newServer(svc)
	defer srv.Close()

	resp := get(t, srv, "/api/games/game-9/players")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
}

func TestPlayerData_PorJogo_200(t *testing.T) {
	svc := &fakeService{pdata: []domain.PlayerDataEntry{{Namespace: "inventory", Key: "gold", Value: []byte(`100`)}}}
	srv := newServer(svc)
	defer srv.Close()

	resp := get(t, srv, "/api/games/game-9/players/player-7/data")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "game-9", svc.gotGameID)
	assert.Equal(t, "player-7", svc.gotPlayer)

	var body []domain.PlayerDataEntry
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Len(t, body, 1)
	assert.Equal(t, "gold", body[0].Key)
	assert.JSONEq(t, `100`, string(body[0].Value))
}

func TestPlayerData_Fora_502(t *testing.T) {
	svc := &fakeService{pdataErr: domain.ErrPlayerDataUnavailable}
	srv := newServer(svc)
	defer srv.Close()

	resp := get(t, srv, "/api/games/g/players/x/data")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
}

func TestCloudSave_200(t *testing.T) {
	svc := &fakeService{save: domain.CloudSave{ContentType: "application/json", Size: 9, Blob: "eyJocCI6NTB9"}}
	srv := newServer(svc)
	defer srv.Close()

	resp := get(t, srv, "/api/games/game-9/players/player-7/save")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "game-9", svc.gotGameID)
	assert.Equal(t, "player-7", svc.gotPlayer)

	var body domain.CloudSave
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, 9, body.Size)
	assert.Equal(t, "eyJocCI6NTB9", body.Blob)
}

func TestCloudSave_SemSave_Repassa404(t *testing.T) {
	svc := &fakeService{saveErr: &domain.UpstreamError{Status: http.StatusNotFound, Code: "not_found"}}
	srv := newServer(svc)
	defer srv.Close()

	resp := get(t, srv, "/api/games/g/players/x/save")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestCloudSave_Fora_502(t *testing.T) {
	svc := &fakeService{saveErr: domain.ErrCloudSaveUnavailable}
	srv := newServer(svc)
	defer srv.Close()

	resp := get(t, srv, "/api/games/g/players/x/save")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
}

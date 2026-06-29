package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c130n/game-backend/services/admin-bff/internal/domain"
)

type fakeService struct {
	topo    domain.Topology
	topoErr error
	live    domain.LiveMetrics
	liveErr error
	ach     []domain.PlayerAchievement
	achErr  error
	gotID   string
}

func (f *fakeService) Topology(context.Context) (domain.Topology, error) {
	return f.topo, f.topoErr
}
func (f *fakeService) LiveMetrics(context.Context) (domain.LiveMetrics, error) {
	return f.live, f.liveErr
}
func (f *fakeService) PlayerAchievements(_ context.Context, id string) ([]domain.PlayerAchievement, error) {
	f.gotID = id
	return f.ach, f.achErr
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

func TestPlayerAchievements_200(t *testing.T) {
	svc := &fakeService{ach: []domain.PlayerAchievement{{ID: "first_win", Unlocked: true}}}
	srv := newServer(svc)
	defer srv.Close()

	resp := get(t, srv, "/api/players/player-7/achievements")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "player-7", svc.gotID)

	var body []domain.PlayerAchievement
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Len(t, body, 1)
	assert.Equal(t, "first_win", body[0].ID)
}

func TestPlayerAchievements_FonteFora_502(t *testing.T) {
	svc := &fakeService{achErr: domain.ErrAchievementsUnavailable}
	srv := newServer(svc)
	defer srv.Close()

	resp := get(t, srv, "/api/players/x/achievements")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
}

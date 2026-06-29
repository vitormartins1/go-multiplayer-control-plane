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
	topo      domain.Topology
	topoErr   error
	live      domain.LiveMetrics
	liveErr   error
	svcMet    domain.ServiceMetrics
	svcErr    error
	page      domain.PlayerPage
	pageErr   error
	ach       []domain.PlayerAchievement
	achErr    error
	gotID     string
	gotSvcID  string
	gotLimit  int
	gotOffset int
}

func (f *fakeService) Topology(context.Context) (domain.Topology, error) {
	return f.topo, f.topoErr
}
func (f *fakeService) LiveMetrics(context.Context) (domain.LiveMetrics, error) {
	return f.live, f.liveErr
}
func (f *fakeService) ServiceMetrics(_ context.Context, id string) (domain.ServiceMetrics, error) {
	f.gotSvcID = id
	return f.svcMet, f.svcErr
}
func (f *fakeService) Players(_ context.Context, limit, offset int) (domain.PlayerPage, error) {
	f.gotLimit, f.gotOffset = limit, offset
	return f.page, f.pageErr
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

func TestServiceMetrics_200_REST(t *testing.T) {
	up := true
	rate := 12.5
	svc := &fakeService{svcMet: domain.ServiceMetrics{ID: "identity-service", Up: &up, RequestRate: &rate}}
	srv := newServer(svc)
	defer srv.Close()

	resp := get(t, srv, "/api/services/identity-service/metrics")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "identity-service", svc.gotSvcID)

	var body domain.ServiceMetrics
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.NotNil(t, body.RequestRate)
	assert.Equal(t, 12.5, *body.RequestRate)
}

func TestServiceMetrics_200_GRPC_RedNull(t *testing.T) {
	up := true
	svc := &fakeService{svcMet: domain.ServiceMetrics{ID: "session-service", Up: &up}} // RED nil
	srv := newServer(svc)
	defer srv.Close()

	resp := get(t, srv, "/api/services/session-service/metrics")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body domain.ServiceMetrics
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Nil(t, body.RequestRate)
	assert.Nil(t, body.LatencyP95)
}

func TestServiceMetrics_IdDesconhecido_404(t *testing.T) {
	svc := &fakeService{svcErr: domain.ErrUnknownService}
	srv := newServer(svc)
	defer srv.Close()

	resp := get(t, srv, "/api/services/nao-existe/metrics")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestServiceMetrics_FonteFora_503(t *testing.T) {
	svc := &fakeService{svcErr: domain.ErrMetricsUnavailable}
	srv := newServer(svc)
	defer srv.Close()

	resp := get(t, srv, "/api/services/identity-service/metrics")
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

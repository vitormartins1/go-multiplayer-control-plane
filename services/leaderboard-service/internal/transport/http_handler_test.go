package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c130n/game-backend/services/leaderboard-service/internal/domain"
)

type fakeVerifier struct {
	playerID string
	err      error
}

func (f fakeVerifier) Verify(_ context.Context, _ string) (string, error) {
	return f.playerID, f.err
}

type fakeService struct {
	submitEntry   domain.Entry
	topEntries    []domain.Entry
	aroundEntries []domain.Entry
	aroundErr     error
	gotPlayerID   string
	gotScore      int64
	gotLimit      int
	gotK          int
}

func (f *fakeService) Submit(_ context.Context, playerID string, score int64) (domain.Entry, error) {
	f.gotPlayerID = playerID
	f.gotScore = score
	return f.submitEntry, nil
}

func (f *fakeService) Top(_ context.Context, limit int) ([]domain.Entry, error) {
	f.gotLimit = limit
	return f.topEntries, nil
}

func (f *fakeService) Around(_ context.Context, playerID string, k int) ([]domain.Entry, error) {
	f.gotPlayerID = playerID
	f.gotK = k
	return f.aroundEntries, f.aroundErr
}

func newServer(svc domain.LeaderboardService, v domain.TokenVerifier) *httptest.Server {
	mux := http.NewServeMux()
	NewHandler(svc, v).Register(mux)
	return httptest.NewServer(mux)
}

func do(t *testing.T, srv *httptest.Server, method, path, token, body string) *http.Response {
	t.Helper()
	var r *http.Request
	var err error
	if body != "" {
		r, err = http.NewRequest(method, srv.URL+path, strings.NewReader(body))
	} else {
		r, err = http.NewRequest(method, srv.URL+path, nil)
	}
	require.NoError(t, err)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := srv.Client().Do(r)
	require.NoError(t, err)
	return resp
}

func TestSubmit_200(t *testing.T) {
	svc := &fakeService{submitEntry: domain.Entry{Rank: 3, PlayerID: "p1", Score: 1500}}
	srv := newServer(svc, fakeVerifier{playerID: "p1"})
	defer srv.Close()

	resp := do(t, srv, http.MethodPost, "/scores", "tok", `{"score":1500}`)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int64(1500), svc.gotScore)
	assert.Equal(t, "p1", svc.gotPlayerID)

	var got entryDTO
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	assert.Equal(t, int64(3), got.Rank)
	assert.Equal(t, int64(1500), got.Score)
}

func TestSubmit_SemScore_400(t *testing.T) {
	srv := newServer(&fakeService{}, fakeVerifier{playerID: "p1"})
	defer srv.Close()

	resp := do(t, srv, http.MethodPost, "/scores", "tok", `{}`)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestTop_200_PassaLimit(t *testing.T) {
	svc := &fakeService{topEntries: []domain.Entry{{Rank: 1, PlayerID: "p2", Score: 300}}}
	srv := newServer(svc, fakeVerifier{playerID: "p1"})
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/leaderboard?limit=5", "tok", "")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 5, svc.gotLimit)

	var body struct {
		Entries []entryDTO `json:"entries"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Len(t, body.Entries, 1)
	assert.Equal(t, "p2", body.Entries[0].PlayerID)
}

func TestMe_200_PassaAround(t *testing.T) {
	svc := &fakeService{aroundEntries: []domain.Entry{{Rank: 3, PlayerID: "p1", Score: 100}}}
	srv := newServer(svc, fakeVerifier{playerID: "p1"})
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/leaderboard/me?around=3", "tok", "")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 3, svc.gotK)
	assert.Equal(t, "p1", svc.gotPlayerID)
}

func TestMe_NaoRankeado_404(t *testing.T) {
	svc := &fakeService{aroundErr: domain.ErrNotRanked}
	srv := newServer(svc, fakeVerifier{playerID: "p1"})
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/leaderboard/me", "tok", "")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestAuth_SemBearer_401(t *testing.T) {
	srv := newServer(&fakeService{}, fakeVerifier{playerID: "p1"})
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/leaderboard", "", "")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAuth_TokenInvalido_401(t *testing.T) {
	srv := newServer(&fakeService{}, fakeVerifier{err: errors.New("token inválido")})
	defer srv.Close()

	resp := do(t, srv, http.MethodPost, "/scores", "qualquer", `{"score":1}`)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

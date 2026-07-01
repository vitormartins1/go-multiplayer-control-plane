package transport

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-game-backend/services/cloud-save-service/internal/domain"
)

type fakeVerifier struct {
	playerID string
	err      error
}

func (f fakeVerifier) Verify(_ context.Context, _ string) (string, error) {
	return f.playerID, f.err
}

type fakeService struct {
	loadSnap    domain.Snapshot
	loadErr     error
	gotSnap     domain.Snapshot
	gotGameID   string
	gotPlayerID string
}

func (f *fakeService) Save(_ context.Context, gameID, playerID string, snapshot domain.Snapshot) error {
	f.gotGameID = gameID
	f.gotPlayerID = playerID
	f.gotSnap = snapshot
	return nil
}

func (f *fakeService) Load(_ context.Context, gameID, playerID string) (domain.Snapshot, error) {
	f.gotGameID = gameID
	f.gotPlayerID = playerID
	return f.loadSnap, f.loadErr
}

func (f *fakeService) LoadFor(_ context.Context, gameID, playerID string) (domain.Snapshot, error) {
	f.gotGameID = gameID
	f.gotPlayerID = playerID
	return f.loadSnap, f.loadErr
}

func newServer(svc domain.CloudSaveService, v domain.TokenVerifier) *httptest.Server {
	mux := http.NewServeMux()
	NewHandler(svc, v).Register(mux)
	return httptest.NewServer(mux)
}

// do dispara uma requisição player-facing. game vazio omite o header X-Game-Id.
func do(t *testing.T, srv *httptest.Server, method, path, token, game, contentType string, body []byte) *http.Response {
	t.Helper()
	var r *http.Request
	var err error
	if body != nil {
		r, err = http.NewRequest(method, srv.URL+path, bytes.NewReader(body))
	} else {
		r, err = http.NewRequest(method, srv.URL+path, nil)
	}
	require.NoError(t, err)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	if game != "" {
		r.Header.Set("X-Game-Id", game)
	}
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	resp, err := srv.Client().Do(r)
	require.NoError(t, err)
	return resp
}

func TestSave_204_PreservaContentType(t *testing.T) {
	svc := &fakeService{}
	srv := newServer(svc, fakeVerifier{playerID: "p1"})
	defer srv.Close()

	resp := do(t, srv, http.MethodPut, "/me/save", "tok", "g1", "application/json", []byte(`{"hp":50}`))
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Equal(t, []byte(`{"hp":50}`), svc.gotSnap.Blob)
	assert.Equal(t, "application/json", svc.gotSnap.ContentType)
	assert.Equal(t, "g1", svc.gotGameID)
	assert.Equal(t, "p1", svc.gotPlayerID)
}

func TestSave_SemGame_400(t *testing.T) {
	svc := &fakeService{}
	srv := newServer(svc, fakeVerifier{playerID: "p1"})
	defer srv.Close()

	resp := do(t, srv, http.MethodPut, "/me/save", "tok", "", "application/json", []byte(`{"hp":50}`))
	defer resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Empty(t, svc.gotGameID) // não chegou ao service
}

func TestLoad_SemGame_400(t *testing.T) {
	srv := newServer(&fakeService{}, fakeVerifier{playerID: "p1"})
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/me/save", "tok", "", "", nil)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestSave_SemContentType_Default(t *testing.T) {
	svc := &fakeService{}
	srv := newServer(svc, fakeVerifier{playerID: "p1"})
	defer srv.Close()

	resp := do(t, srv, http.MethodPut, "/me/save", "tok", "g1", "", []byte("bytes-do-save"))
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Equal(t, "application/octet-stream", svc.gotSnap.ContentType)
}

func TestSave_TooLarge_413(t *testing.T) {
	srv := newServer(&fakeService{}, fakeVerifier{playerID: "p1"})
	defer srv.Close()

	big := bytes.Repeat([]byte("a"), maxSaveBytes+1)
	resp := do(t, srv, http.MethodPut, "/me/save", "tok", "g1", "application/octet-stream", big)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode)
}

func TestLoad_200_RetornaBlobEContentType(t *testing.T) {
	svc := &fakeService{loadSnap: domain.Snapshot{Blob: []byte("save-bytes"), ContentType: "application/json"}}
	srv := newServer(svc, fakeVerifier{playerID: "p9"})
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/me/save", "tok", "g1", "", nil)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	b, _ := io.ReadAll(resp.Body)
	assert.Equal(t, []byte("save-bytes"), b)
	assert.Equal(t, "g1", svc.gotGameID)
	assert.Equal(t, "p9", svc.gotPlayerID)
}

func TestLoad_Inexistente_404(t *testing.T) {
	svc := &fakeService{loadErr: domain.ErrNotFound}
	srv := newServer(svc, fakeVerifier{playerID: "p1"})
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/me/save", "tok", "g1", "", nil)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestAuth_SemBearer_401(t *testing.T) {
	srv := newServer(&fakeService{}, fakeVerifier{playerID: "p1"})
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/me/save", "", "g1", "", nil)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAuth_TokenInvalido_401(t *testing.T) {
	srv := newServer(&fakeService{}, fakeVerifier{err: errors.New("token inválido")})
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/me/save", "qualquer", "g1", "", nil)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAdminLoad_200_MetadadosEBlobBase64(t *testing.T) {
	updated := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	svc := &fakeService{loadSnap: domain.Snapshot{
		Blob:        []byte("save-bytes-\x00\x01"),
		ContentType: "application/json",
		UpdatedAt:   updated,
	}}
	srv := newServer(svc, fakeVerifier{}) // admin não usa o verifier
	defer srv.Close()

	// admin não exige Bearer nem X-Game-Id (jogo/jogador no path).
	resp := do(t, srv, http.MethodGet, "/admin/games/g1/players/p1/save", "", "", "", nil)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "g1", svc.gotGameID)
	assert.Equal(t, "p1", svc.gotPlayerID)

	var got struct {
		ContentType string    `json:"contentType"`
		Size        int       `json:"size"`
		UpdatedAt   time.Time `json:"updatedAt"`
		Blob        string    `json:"blob"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	assert.Equal(t, "application/json", got.ContentType)
	assert.Equal(t, len("save-bytes-\x00\x01"), got.Size)
	assert.True(t, got.UpdatedAt.Equal(updated))
	decoded, err := base64.StdEncoding.DecodeString(got.Blob)
	require.NoError(t, err)
	assert.Equal(t, []byte("save-bytes-\x00\x01"), decoded)
}

func TestAdminLoad_Inexistente_404(t *testing.T) {
	svc := &fakeService{loadErr: domain.ErrNotFound}
	srv := newServer(svc, fakeVerifier{})
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/admin/games/g1/players/fantasma/save", "", "", "", nil)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

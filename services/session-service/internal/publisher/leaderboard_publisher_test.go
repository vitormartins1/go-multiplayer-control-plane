package publisher

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-game-backend/services/session-service/internal/domain"
)

func TestLeaderboardPublish_ScorePorJogador(t *testing.T) {
	var mu sync.Mutex
	got := map[string]int64{} // path -> score
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Score int64 `json:"score"`
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		mu.Lock()
		got[r.URL.Path] = body.Score
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := NewLeaderboard(srv.URL, srv.Client(), discard())
	err := p.Publish(context.Background(), "game-1", []domain.PlayerResult{
		{PlayerID: "alice", Score: 100},
		{PlayerID: "bob", Score: 300},
	})
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, int64(100), got["/admin/games/game-1/players/alice/scores"])
	assert.Equal(t, int64(300), got["/admin/games/game-1/players/bob/scores"])
}

func TestLeaderboardPublish_Fora_BestEffort_NaoFalha(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	p := NewLeaderboard(srv.URL, srv.Client(), discard())
	err := p.Publish(context.Background(), "game-1", []domain.PlayerResult{{PlayerID: "p1", Score: 1}})
	assert.NoError(t, err)
}

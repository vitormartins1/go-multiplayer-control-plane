package publisher

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c130n/game-backend/services/session-service/internal/domain"
)

func discard() *slog.Logger { return slog.New(slog.NewJSONHandler(io.Discard, nil)) }

func TestPublish_MatchPlayedTodos_MatchWonVencedores(t *testing.T) {
	var mu sync.Mutex
	type call struct{ path, body string }
	var calls []call
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		calls = append(calls, call{r.URL.Path, string(b)})
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := NewAchievements(srv.URL, srv.Client(), discard())
	err := p.Publish(context.Background(), "game-1", []domain.PlayerResult{
		{PlayerID: "winner", Won: true},
		{PlayerID: "loser", Won: false},
	})
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	// winner: match_played + match_won; loser: só match_played -> 3 chamadas.
	require.Len(t, calls, 3)
	got := map[string]int{}
	for _, c := range calls {
		assert.Contains(t, c.path, "/admin/games/game-1/players/")
		got[c.path+"|"+c.body]++
	}
	assert.Equal(t, 1, got["/admin/games/game-1/players/winner/events|{\"type\":\"match_played\"}"])
	assert.Equal(t, 1, got["/admin/games/game-1/players/winner/events|{\"type\":\"match_won\"}"])
	assert.Equal(t, 1, got["/admin/games/game-1/players/loser/events|{\"type\":\"match_played\"}"])
}

func TestPublish_AchievementsFora_BestEffort_NaoFalha(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := NewAchievements(srv.URL, srv.Client(), discard())
	err := p.Publish(context.Background(), "game-1", []domain.PlayerResult{{PlayerID: "p1", Won: true}})
	assert.NoError(t, err, "best-effort: não falha o report mesmo com achievements fora")
}

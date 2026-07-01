package publisher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/vitormartins1/go-game-backend/services/session-service/internal/domain"
)

var _ domain.ResultPublisher = (*Leaderboard)(nil)

// Leaderboard é o fan-out do score para o leaderboard server-side
// (POST /admin/games/{gameId}/players/{playerID}/scores). Best-effort, com a
// internal key (ADR-0018) injetada no http.Client. Irmão do Achievements.
type Leaderboard struct {
	baseURL string
	http    *http.Client
	log     *slog.Logger
}

// NewLeaderboard cria o publisher. O httpClient deve injetar a internal key.
func NewLeaderboard(baseURL string, httpClient *http.Client, log *slog.Logger) *Leaderboard {
	return &Leaderboard{baseURL: baseURL, http: httpClient, log: log}
}

// Publish envia o score de cada jogador ao ranking do jogo (highscore no store).
func (l *Leaderboard) Publish(ctx context.Context, gameID string, results []domain.PlayerResult) error {
	for _, r := range results {
		l.send(ctx, gameID, r.PlayerID, r.Score)
	}
	return nil // best-effort: falhas individuais são logadas em send
}

func (l *Leaderboard) send(ctx context.Context, gameID, playerID string, score int64) {
	u := l.baseURL + "/admin/games/" + url.PathEscape(gameID) + "/players/" + url.PathEscape(playerID) + "/scores"
	body, _ := json.Marshal(map[string]int64{"score": score})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		l.logFail(gameID, playerID, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := l.http.Do(req)
	if err != nil {
		l.logFail(gameID, playerID, err)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		l.logFail(gameID, playerID, fmt.Errorf("status %d", resp.StatusCode))
	}
}

func (l *Leaderboard) logFail(gameID, playerID string, err error) {
	l.log.Warn("leaderboard fan-out falhou (best-effort)",
		slog.String("game_id", gameID),
		slog.String("player_id", playerID),
		slog.String("err", err.Error()),
	)
}

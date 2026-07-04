// Package leaderboard é o adapter de saída que lê a API admin do
// leaderboard-service (top-N por jogo, ADR-0017) e satisfaz
// domain.LeaderboardReader. Indisponibilidade vira domain.ErrLeaderboardUnavailable.
package leaderboard

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/vitormartins1/go-game-backend/services/admin-bff/internal/domain"
	"github.com/vitormartins1/go-game-backend/services/admin-bff/internal/upstream"
)

var _ domain.LeaderboardReader = (*Client)(nil)

// Client lê o leaderboard-service em baseURL (ex.: http://leaderboard-service:8080).
type Client struct {
	baseURL string
	http    *http.Client
}

// New cria o client. O httpClient permite injetar timeout/transport (otelhttp +
// internal key).
func New(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = upstream.DefaultClient()
	}
	return &Client{baseURL: baseURL, http: httpClient}
}

// topResponse é o shape de GET /admin/games/{g}/leaderboard.
type topResponse struct {
	Entries []struct {
		Rank     int64  `json:"rank"`
		PlayerID string `json:"playerId"`
		Score    int64  `json:"score"`
	} `json:"entries"`
}

// SetScore substitui o score do jogador independente do valor atual (admin override).
func (c *Client) SetScore(ctx context.Context, gameID, playerID string, score int64) (domain.LeaderboardEntry, error) {
	payload, _ := json.Marshal(struct {
		Score int64 `json:"score"`
	}{score})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut,
		c.baseURL+"/admin/games/"+url.PathEscape(gameID)+"/players/"+url.PathEscape(playerID)+"/scores",
		bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := upstream.Do(c.http, req, domain.ErrLeaderboardUnavailable)
	if err != nil {
		return domain.LeaderboardEntry{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	var e struct {
		Rank     int64  `json:"rank"`
		PlayerID string `json:"playerId"`
		Score    int64  `json:"score"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&e); err != nil {
		return domain.LeaderboardEntry{}, fmt.Errorf("leaderboard: decode: %w", err)
	}
	return domain.LeaderboardEntry{Rank: e.Rank, PlayerID: e.PlayerID, Score: e.Score}, nil
}

// TopForGame lê o top-N do jogo.
func (c *Client) TopForGame(ctx context.Context, gameID string, limit int) ([]domain.LeaderboardEntry, error) {
	u := c.baseURL + "/admin/games/" + url.PathEscape(gameID) + "/leaderboard"
	if limit > 0 {
		u += "?limit=" + strconv.Itoa(limit)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	resp, err := upstream.Do(c.http, req, domain.ErrLeaderboardUnavailable)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var body topResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("leaderboard: decode: %w", err)
	}
	out := make([]domain.LeaderboardEntry, 0, len(body.Entries))
	for _, e := range body.Entries {
		out = append(out, domain.LeaderboardEntry{Rank: e.Rank, PlayerID: e.PlayerID, Score: e.Score})
	}
	return out, nil
}

package publisher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/c130n/game-backend/services/session-service/internal/domain"
)

var _ domain.ResultPublisher = (*Achievements)(nil)

// Achievements é o fan-out real: traduz o resultado da partida em eventos de
// gameplay e os envia ao achievements server-side (POST /admin/games/{gameId}/
// players/{playerID}/events). É best-effort: loga falhas e não derruba o
// ReportResult (o fim da partida não pode depender do achievements estar no ar).
// A auth serviço-a-serviço (X-Internal-Key, ADR-0018) vai no http.Client.
type Achievements struct {
	baseURL string
	http    *http.Client
	log     *slog.Logger
}

// NewAchievements cria o publisher. O httpClient deve injetar a internal key
// (auth.InternalKeyTransport).
func NewAchievements(baseURL string, httpClient *http.Client, log *slog.Logger) *Achievements {
	return &Achievements{baseURL: baseURL, http: httpClient, log: log}
}

// Publish envia, por jogador: match_played (todos) e match_won (vencedores).
func (a *Achievements) Publish(ctx context.Context, gameID string, results []domain.PlayerResult) error {
	for _, r := range results {
		a.send(ctx, gameID, r.PlayerID, "match_played")
		if r.Won {
			a.send(ctx, gameID, r.PlayerID, "match_won")
		}
	}
	return nil // best-effort: falhas individuais são logadas em send
}

func (a *Achievements) send(ctx context.Context, gameID, playerID, eventType string) {
	u := a.baseURL + "/admin/games/" + url.PathEscape(gameID) + "/players/" + url.PathEscape(playerID) + "/events"
	body, _ := json.Marshal(map[string]string{"type": eventType})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		a.logFail(gameID, playerID, eventType, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.http.Do(req)
	if err != nil {
		a.logFail(gameID, playerID, eventType, err)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		a.logFail(gameID, playerID, eventType, fmt.Errorf("status %d", resp.StatusCode))
	}
}

func (a *Achievements) logFail(gameID, playerID, eventType string, err error) {
	a.log.Warn("result fan-out falhou (best-effort)",
		slog.String("game_id", gameID),
		slog.String("player_id", playerID),
		slog.String("event", eventType),
		slog.String("err", err.Error()),
	)
}

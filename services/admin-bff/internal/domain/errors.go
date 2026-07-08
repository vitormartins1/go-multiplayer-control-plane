package domain

import (
	"errors"
	"fmt"
)

var (
	// ErrMetricsUnavailable indica que o Prometheus está inacessível. O service
	// degrada a topologia (nós unknown) e o transport mapeia /metrics/live -> 503.
	ErrMetricsUnavailable = errors.New("admin: metrics source unavailable")
	// ErrAchievementsUnavailable indica que o achievements-service está
	// inacessível. O transport mapeia -> 502.
	ErrAchievementsUnavailable = errors.New("admin: achievements source unavailable")
	// ErrIdentityUnavailable indica que o identity-service está inacessível. O
	// transport mapeia -> 502.
	ErrIdentityUnavailable = errors.New("admin: identity source unavailable")
	// ErrGameRegistryUnavailable indica que o game-registry está inacessível. O
	// transport mapeia -> 502.
	ErrGameRegistryUnavailable = errors.New("admin: game-registry source unavailable")
	// ErrLeaderboardUnavailable indica que o leaderboard-service está
	// inacessível. O transport mapeia -> 502.
	ErrLeaderboardUnavailable = errors.New("admin: leaderboard source unavailable")
	// ErrPlayerDataUnavailable indica que o player-data-service está
	// inacessível. O transport mapeia -> 502.
	ErrPlayerDataUnavailable = errors.New("admin: player-data source unavailable")
	// ErrCloudSaveUnavailable indica que o cloud-save-service está inacessível.
	// O transport mapeia -> 502.
	ErrCloudSaveUnavailable = errors.New("admin: cloud-save source unavailable")
	// ErrPlayerNotFound indica que o jogador não existe no identity (spec 020).
	// O transport mapeia -> 404.
	ErrPlayerNotFound = errors.New("admin: player not found")
)

// UpstreamError carrega um erro de validação (4xx) repassado de um serviço
// upstream — o transport reescreve o mesmo status (ex.: slug duplicado -> 409),
// em vez de mascarar como 502. Code é o código de erro do corpo do upstream.
type UpstreamError struct {
	Status int
	Code   string
}

func (e *UpstreamError) Error() string {
	return fmt.Sprintf("admin: upstream error %d (%s)", e.Status, e.Code)
}

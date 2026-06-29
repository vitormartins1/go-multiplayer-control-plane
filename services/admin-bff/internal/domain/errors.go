package domain

import "errors"

var (
	// ErrMetricsUnavailable indica que o Prometheus está inacessível. O service
	// degrada a topologia (nós unknown) e o transport mapeia /metrics/live -> 503.
	ErrMetricsUnavailable = errors.New("admin: metrics source unavailable")
	// ErrAchievementsUnavailable indica que o achievements-service está
	// inacessível. O transport mapeia -> 502.
	ErrAchievementsUnavailable = errors.New("admin: achievements source unavailable")
	// ErrUnknownService indica um id de serviço fora de KnownServices. O
	// transport mapeia -> 404.
	ErrUnknownService = errors.New("admin: unknown service")
)

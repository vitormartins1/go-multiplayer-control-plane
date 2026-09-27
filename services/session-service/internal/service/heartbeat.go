package service

import (
	"context"
	"fmt"
	"time"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/domain"
)

// DefaultWorldHeartbeatTTL é quanto um mundo que manda heartbeat aguenta sem
// outro antes de o sweeper fechá-lo (spec 032). O intervalo sugerido ao jogo é
// um terço disso: perder dois heartbeats seguidos ainda não fecha o mundo.
const DefaultWorldHeartbeatTTL = 90 * time.Second

// WithWorldHeartbeatTTL troca o TTL do heartbeat de mundo (WORLD_HEARTBEAT_TTL).
func WithWorldHeartbeatTTL(d time.Duration) Option {
	return func(s *Service) {
		if d > 0 {
			s.heartbeatTTL = d
		}
	}
}

// Heartbeat marca o mundo como vivo (spec 032). A partir do primeiro, o mundo
// sai da regra de idade do sweeper e passa a depender dos heartbeats.
func (s *Service) Heartbeat(_ context.Context, sessionID string) (time.Duration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[sessionID]
	if !ok || sess.Origin != domain.OriginHostWorld {
		return 0, fmt.Errorf("session service: heartbeat: %w", domain.ErrWorldNotFound)
	}
	sess.LastHeartbeatAt = s.now()
	s.heartbeats++
	// piso de 1 s: com um TTL minúsculo (config errada) o intervalo truncado
	// para segundos viraria 0 e o cliente martelaria o session.
	return max(s.heartbeatTTL/3, time.Second), nil
}

// HeartbeatsTotal alimenta a métrica session_world_heartbeats_total.
func (s *Service) HeartbeatsTotal() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.heartbeats
}

// expiredLocked diz se a sessão venceu e com que motivo (chamar com s.mu).
// Mundo com heartbeat: vence pelo heartbeat (abandoned), nunca pela idade.
// Sem heartbeat: a regra de sempre, idade > ttl (ttl).
func (s *Service) expiredLocked(sess *domain.Session, ageCutoff time.Time) (domain.CloseReason, bool) {
	if !sess.LastHeartbeatAt.IsZero() {
		return domain.CloseAbandoned, sess.LastHeartbeatAt.Before(s.now().Add(-s.heartbeatTTL))
	}
	return domain.CloseTTL, sess.OpenedAt.Before(ageCutoff)
}

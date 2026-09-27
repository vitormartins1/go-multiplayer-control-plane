package service

// Leituras para as métricas Prometheus (CounterFunc/GaugeFunc no main).

import (
	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/domain"
)

// countParticipation incrementa o contador do evento (participation_events_total).
func (s *Service) countParticipation(event string) {
	s.mu.Lock()
	s.participationEvents[event]++
	s.mu.Unlock()
}

// ParticipationTotal devolve o total de eventos de participação do tipo (métrica
// participation_events_total{event} — CounterFunc no main).
func (s *Service) ParticipationTotal(event string) float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.participationEvents[event]
}

// OccupiedSlots devolve o total de slots ocupados de um papel entre os mundos
// abertos (métrica world_role_slots{role}).
func (s *Service) OccupiedSlots(role string) float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int
	for _, sess := range s.sessions {
		if slot, ok := sess.RoleSlots[role]; ok {
			n += len(slot.Occupants)
		}
	}
	return float64(n)
}

// ActiveSessions devolve o número de sessões ativas (métrica).
func (s *Service) ActiveSessions() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return float64(len(s.sessions))
}

// ClosedTotal devolve o total de sessões fechadas pelo motivo (métrica
// sessions_closed_total{reason} — registrada como CounterFunc no main).
func (s *Service) ClosedTotal(reason domain.CloseReason) float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed[reason]
}

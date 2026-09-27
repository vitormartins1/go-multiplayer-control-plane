package service

// Superfície admin do session: listagens para o console e o fechamento
// forçado (specs 005, 024).

import (
	"context"
	"fmt"
	"sort"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/domain"
)

// SessionsByGame devolve as sessões do jogo para a superfície admin (spec 005):
// abertas (do mapa de ativas) e fechadas (do histórico bounded), ambas em ordem
// da mais recente para a mais antiga. Devolve cópias — o chamador não enxerga
// o estado interno.
func (s *Service) SessionsByGame(gameID string) (active, closed []domain.Session) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, sess := range s.sessions {
		if sess.GameID == gameID {
			active = append(active, *sess)
		}
	}
	sort.Slice(active, func(i, j int) bool { return active[i].OpenedAt.After(active[j].OpenedAt) })

	// o ring guarda em ordem de fechamento; percorrer de trás = mais recente primeiro.
	for i := len(s.history) - 1; i >= 0; i-- {
		if s.history[i].GameID == gameID {
			closed = append(closed, *s.history[i])
		}
	}
	return active, closed
}

// AllSessions devolve as sessões de TODOS os jogos para o board global do
// console (spec 007): ativas (do mapa) e fechadas (do histórico bounded), da mais
// recente para a mais antiga. Devolve cópias. Irmão global do SessionsByGame.
func (s *Service) AllSessions() (active, closed []domain.Session) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, sess := range s.sessions {
		active = append(active, *sess)
	}
	sort.Slice(active, func(i, j int) bool { return active[i].OpenedAt.After(active[j].OpenedAt) })

	for i := len(s.history) - 1; i >= 0; i-- {
		closed = append(closed, *s.history[i])
	}
	return active, closed
}

// CloseByAdmin encerra uma sessão ativa por ação do operador (CLOSED/admin):
// reusa o fechamento unificado (libera o server, entra no histórico). Devolve
// ErrSessionNotFound se não houver sessão ativa para o id (idempotente do
// ponto de vista do operador). Spec 007; id aceita matchID OU sessionID
// (spec 024) — ver resolveAdminCloseID.
func (s *Service) CloseByAdmin(ctx context.Context, id string) (*domain.Session, error) {
	sessionID, ok := s.resolveAdminCloseID(id)
	if !ok {
		return nil, fmt.Errorf("session service: close by admin: %w", domain.ErrSessionNotFound)
	}
	sess, err := s.closeSession(ctx, sessionID, domain.CloseAdmin)
	if err != nil {
		return nil, fmt.Errorf("session service: close by admin: %w", err)
	}
	return sess, nil
}

// resolveAdminCloseID aceita tanto um matchID (caminho legado: queue_arena/
// coop_mission/battle_royale, populado por CreateSession no matchIndex)
// quanto um sessionID (host_world_dropin, cuja identidade externa já é o
// próprio sessionID — nunca ganha matchId) — o operador no admin console não
// distingue os dois formatos, então o resolvedor tenta ambos (spec 024).
func (s *Service) resolveAdminCloseID(id string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sessionID, ok := s.matchIndex[id]; ok {
		return sessionID, true
	}
	if _, ok := s.sessions[id]; ok {
		return id, true
	}
	return "", false
}

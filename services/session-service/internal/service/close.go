package service

// Fechamento de sessões: o caminho único (closeSession), a persistência e o
// fan-out best-effort do fechamento, e os varredores (TTL/heartbeat e órfãs).

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/domain"
)

// closeSession é o fechamento unificado (spec 004): marca CLOSED com motivo,
// tira do mapa de ativas e libera o server — usado por ReportResult (result),
// pelo sweeper (ttl/abandoned) e pelo host_world (host_left). Opera pela chave
// interna (sessionID). Libera o server PRIMEIRO em relação a qualquer fan-out:
// o fim da partida não pode depender de destino externo.
func (s *Service) closeSession(ctx context.Context, sessionID string, reason domain.CloseReason) (*domain.Session, error) {
	return s.closeSessionIf(ctx, sessionID, func(*domain.Session) (domain.CloseReason, bool) { return reason, true })
}

// errNotExpired: o closeSessionIf achou a sessão mas a condição não vale mais
// (ex.: o heartbeat chegou entre a varredura do Sweep e o fechamento).
var errNotExpired = errors.New("session service: session no longer expired")

// closeSessionIf fecha a sessão só se `should` ainda aprovar, avaliado sob o
// mesmo lock que tira a sessão do mapa — sem janela entre decidir e fechar.
func (s *Service) closeSessionIf(ctx context.Context, sessionID string, should func(*domain.Session) (domain.CloseReason, bool)) (*domain.Session, error) {
	s.mu.Lock()
	sess, ok := s.sessions[sessionID]
	if !ok {
		s.mu.Unlock()
		return nil, domain.ErrSessionNotFound
	}
	reason, doClose := should(sess)
	if !doClose {
		s.mu.Unlock()
		return nil, errNotExpired
	}
	delete(s.sessions, sessionID)
	if sess.MatchID != "" {
		delete(s.matchIndex, sess.MatchID) // queue_arena: limpa o índice reverso
	}
	if sess.WorldTokenHash != "" {
		s.retireWorldTokenLocked(sess.WorldTokenHash) // p2p: só vale para o reenvio do fechamento
	}
	sess.State = domain.SessionClosed
	sess.ClosedAt = s.now()
	sess.CloseReason = reason
	s.closed[reason]++
	s.history = append(s.history, sess)
	if len(s.history) > s.historyLimit {
		s.history = s.history[len(s.history)-s.historyLimit:]
	}
	s.mu.Unlock()

	var releaseErr error
	if sess.ServerID != "" { // mundo p2p não tem GameServer (spec 031)
		releaseErr = s.allocator.Release(ctx, sess.ServerID)
	}
	// persiste o histórico DEPOIS do release, best-effort, independente do
	// resultado dele (spec 025) — a sessão já está CLOSED e fora do mapa de
	// ativas; o registro do que aconteceu não deve depender de o server ter
	// sido devolvido ao pool com sucesso.
	s.persistHistory(ctx, sess)
	// publica o fechamento no fan-out (spec 028) — alimenta o session health
	// do analytics-service. Mesmo espírito best-effort do persistHistory.
	s.publishSessionClosed(ctx, sess)
	if releaseErr != nil {
		return sess, fmt.Errorf("release: %w", releaseErr)
	}
	return sess, nil
}

// persistHistory grava o registro de histórico da sessão fechada, best-effort
// (spec 025): falha só loga + incrementa a métrica
// session_history_persist_failures_total, nunca propaga — o fechamento da
// sessão não pode depender de um destino externo (mesma filosofia do fan-out,
// ADR-0020). No-op se nenhum HistoryStore foi configurado (dev sem Postgres).
func (s *Service) persistHistory(ctx context.Context, sess *domain.Session) {
	if s.historyStore == nil {
		return
	}
	if err := s.historyStore.SaveClosed(ctx, sess); err != nil {
		s.mu.Lock()
		s.historyPersistFailures++
		s.mu.Unlock()
		s.log.Error("session history: falha ao persistir sessão fechada", "sessionId", sess.ID, "gameId", sess.GameID, "err", err)
	}
}

// HistoryPersistFailures devolve o total de falhas ao persistir o histórico
// (métrica session_history_persist_failures_total — CounterFunc no main).
func (s *Service) HistoryPersistFailures() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.historyPersistFailures
}

// publishSessionClosed publica o fechamento no fan-out, best-effort (spec
// 028): falha só loga + incrementa a métrica
// session_closed_publish_failures_total, nunca propaga — o fechamento da
// sessão não pode depender de um destino externo (mesma filosofia de
// persistHistory). No-op se nenhum SessionClosedPublisher foi configurado.
func (s *Service) publishSessionClosed(ctx context.Context, sess *domain.Session) {
	if s.sessionClosedPublisher == nil {
		return
	}
	if err := s.sessionClosedPublisher.PublishSessionClosed(ctx, sess); err != nil {
		s.mu.Lock()
		s.sessionClosedPublishFailures++
		s.mu.Unlock()
		s.log.Error("session closed: falha ao publicar no fan-out", "sessionId", sess.ID, "gameId", sess.GameID, "err", err)
	}
}

// SessionClosedPublishFailures devolve o total de falhas ao publicar o
// fechamento (métrica session_closed_publish_failures_total — CounterFunc
// no main).
func (s *Service) SessionClosedPublishFailures() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessionClosedPublishFailures
}

// Sweep fecha as sessões vencidas e devolve quantas fechou: as abertas há mais
// que ttl (CLOSED/ttl) e, entre os mundos que mandam heartbeat, os que ficaram
// sem heartbeat além do WORLD_HEARTBEAT_TTL (CLOSED/abandoned, spec 032). Erros
// de release não interrompem a varredura.
func (s *Service) Sweep(ctx context.Context, ttl time.Duration) int {
	cutoff := s.now().Add(-ttl)
	s.mu.Lock()
	expired := make(map[string]struct{})
	for sessionID, sess := range s.sessions {
		if _, ok := s.expiredLocked(sess, cutoff); ok {
			expired[sessionID] = struct{}{}
		}
	}
	s.mu.Unlock()

	closed := 0
	for sessionID := range expired {
		// reavalia sob o lock do fechamento: um heartbeat que chegou enquanto
		// outras sessões fechavam (release/SNS lentos) salva o mundo.
		// ErrSessionNotFound = fechada por outro caminho no meio; ok.
		if _, err := s.closeSessionIf(ctx, sessionID, func(sess *domain.Session) (domain.CloseReason, bool) {
			return s.expiredLocked(sess, cutoff)
		}); err == nil || !errors.Is(err, errNotExpired) && !errors.Is(err, domain.ErrSessionNotFound) {
			closed++ // fechou (release com erro também conta: a sessão saiu)
		}
	}
	return closed
}

// reconcile fecha (CLOSED/abandoned) as sessões OPEN cujo game server sumiu da
// fonte de verdade (registry) — o game server abandonou a partida sem
// ReportResult (spec 008, ADR-0022). No-op se não há registry (modo static).
// Respeita o grace period: sessões abertas há menos que grace ficam de fora,
// para tolerar staleness do LIST logo após o Create. Se a listagem falhar, NÃO
// fecha nada nesse tick — um blip de leitura no k8s não pode causar fechamento
// em massa de sessões saudáveis.
func (s *Service) reconcile(ctx context.Context, grace time.Duration, log *slog.Logger) int {
	if s.registry == nil {
		return 0
	}
	live, err := s.registry.LiveServerIDs(ctx)
	if err != nil {
		log.Warn("reconcile: listagem de game servers falhou — tick pulado", "err", err)
		return 0
	}

	cutoff := s.now().Add(-grace)
	s.mu.Lock()
	var orphans []string
	for sessionID, sess := range s.sessions {
		if sess.ServerID == "" {
			continue // mundo p2p (spec 031): não há GameServer para sumir
		}
		if _, alive := live[sess.ServerID]; !alive && sess.OpenedAt.Before(cutoff) {
			orphans = append(orphans, sessionID)
		}
	}
	s.mu.Unlock()

	for _, sessionID := range orphans {
		// ErrSessionNotFound = fechada por result/admin entre a varredura e o close; ok.
		_, _ = s.closeSession(ctx, sessionID, domain.CloseAbandoned)
	}
	return len(orphans)
}

// RunSweeper roda Sweep (TTL) e reconcile (órfãs) a cada interval até o ctx
// encerrar (goroutine no main, shutdown limpo via context — mesmo padrão do Run
// do matchmaking). O TTL é backstop; a reconciliação fecha órfãs em segundos
// quando há registry (modo Agones).
func (s *Service) RunSweeper(ctx context.Context, ttl, interval time.Duration, log *slog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n := s.Sweep(ctx, ttl); n > 0 {
				log.Info("sweeper: sessões expiradas fechadas", "count", n, "ttl", ttl)
			}
			if n := s.reconcile(ctx, s.orphanGrace, log); n > 0 {
				log.Info("reconcile: sessões órfãs fechadas", "count", n, "grace", s.orphanGrace)
			}
		}
	}
}

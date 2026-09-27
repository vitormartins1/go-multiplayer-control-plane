package service

// Arquétipo host_world_dropin (specs 011–013, 029–032): abrir mundo, injetar
// e tirar phantoms, diretório de invasão e eventos de participação.

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"

	"github.com/google/uuid"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/domain"
)

// OpenWorld abre um mundo persistente para o host (host_world_dropin, spec 011):
// lê o perfil, aloca um server, deriva os slots por papel e emite o connect
// token do host. Qualquer problema com o perfil (ausente, arquétipo != host_world
// _dropin, rede) vira ErrUnsupportedArchetype — só esse arquétipo abre mundo.
// Em modo p2p (spec 031) não aloca: o cliente do host hospeda, os phantoms o
// alcançam por attrs.HostConnection e o host recebe um world token para reportar.
func (s *Service) OpenWorld(ctx context.Context, hostPlayerID, gameID, modeID string, attrs domain.WorldAttributes) (*domain.Session, domain.ConnectGrant, string, error) {
	fail := func(err error) (*domain.Session, domain.ConnectGrant, string, error) {
		return nil, domain.ConnectGrant{}, "", fmt.Errorf("session service: open world: %w", err)
	}
	if s.profiles == nil {
		return fail(domain.ErrUnsupportedArchetype)
	}
	snap, err := s.modeSnapshot(ctx, gameID, modeID)
	if err != nil {
		return fail(fmt.Errorf("%v: %w", err, domain.ErrUnsupportedArchetype))
	}
	// a checagem de arquétipo subiu do adapter (que virou genérico por modo,
	// ADR-0024) para cá: só host_world_dropin abre mundo.
	if snap.Archetype != domain.ArchetypeHostWorldDropIn {
		return fail(fmt.Errorf("archetype %q: %w", snap.Archetype, domain.ErrUnsupportedArchetype))
	}
	p2p := snap.Topology == domain.TopologyP2P
	if p2p {
		if err := validHostConnection(attrs.HostConnection); err != nil {
			return fail(err)
		}
		// um mundo p2p aberto por host: abrir outro (o jogo caiu e voltou, ou
		// um cliente abusivo) fecha o anterior — sem isto, qualquer jogador
		// enchia o diretório de invasão de mundos que ninguém hospeda.
		s.closeHostP2PWorlds(ctx, hostPlayerID)
	}

	sessionID := uuid.NewString()
	var server domain.Server
	var worldToken, worldTokenHash string
	if p2p {
		// o claim server do connect token leva o id de conexão do host: o host
		// confere que o token foi emitido para o mundo dele (plan 031).
		server = domain.Server{Address: attrs.HostConnection}
		if worldToken, worldTokenHash, err = newWorldToken(); err != nil {
			return fail(err)
		}
	} else {
		if server, err = s.allocator.Allocate(ctx, sessionID, gameID, snap.Fleet); err != nil {
			return fail(fmt.Errorf("allocate: %w", err))
		}
	}

	// token do host (papel host) — rollback do server se a emissão falhar.
	tok, err := s.issuer.Issue(hostPlayerID, sessionID, server.Address, domain.ConnectClaims{Role: domain.RoleHost, ModeID: modeID})
	if err != nil {
		if server.ID != "" {
			if relErr := s.allocator.Release(ctx, server.ID); relErr != nil {
				s.log.Warn("open world: release após falha no token", "session_id", sessionID, "err", relErr)
			}
		}
		return fail(fmt.Errorf("issue token: %w", err))
	}

	openedAt := s.now()
	sess := &domain.Session{
		ID:             sessionID,
		GameID:         gameID,
		ModeID:         modeID,
		Profile:        &snap,
		Players:        []string{hostPlayerID},
		Participants:   []domain.Participant{{PlayerID: hostPlayerID, Role: domain.RoleHost, JoinedAt: openedAt}},
		ServerID:       server.ID,
		Origin:         domain.OriginHostWorld,
		HostPlayerID:   hostPlayerID,
		RoleSlots:      roleSlotsFromProfile(snap),
		HostSoulLevel:  attrs.SoulLevel,
		WeaponTier:     attrs.WeaponTier,
		PasswordHash:   domain.HashWorldPassword(attrs.Password),
		WorldTokenHash: worldTokenHash,
		State:          domain.SessionOpen,
		OpenedAt:       openedAt,
	}
	if p2p {
		sess.HostConnection = attrs.HostConnection
	} else {
		sess.ServerAddress = server.Address
	}

	s.mu.Lock()
	s.sessions[sessionID] = sess // host_world: identidade = sessionID, sem matchIndex
	if worldTokenHash != "" {
		s.worldTokens[worldTokenHash] = worldTokenEntry{sessionID: sessionID}
	}
	s.mu.Unlock()

	return sess, domain.ConnectGrant{PlayerID: hostPlayerID, Token: tok}, worldToken, nil
}

// roleSlotsFromProfile deriva a capacidade por papel do perfil, só para os papéis
// habilitados (EnabledRoles). O host não ocupa slot — é o dono do mundo.
func roleSlotsFromProfile(p domain.ProfileSnapshot) map[string]*domain.RoleSlot {
	slots := make(map[string]*domain.RoleSlot)
	for _, role := range p.EnabledRoles {
		switch role {
		case domain.RoleCoop:
			slots[domain.RoleCoop] = &domain.RoleSlot{Capacity: p.CoopSlots}
		case domain.RoleInvader:
			slots[domain.RoleInvader] = &domain.RoleSlot{Capacity: p.InvaderSlots}
		}
	}
	return slots
}

// JoinWorld injeta um phantom no mundo ocupando um slot do papel e emite o
// connect token com o claim role. O slot é reservado sob o lock; o token é
// cunhado fora (cripto) e o slot é revertido se a emissão falhar.
func (s *Service) JoinWorld(ctx context.Context, sessionID, playerID, role string) (domain.ConnectGrant, domain.WorldEndpoint, error) {
	s.mu.Lock()
	sess, ok := s.sessions[sessionID]
	if !ok || sess.Origin != domain.OriginHostWorld {
		s.mu.Unlock()
		return domain.ConnectGrant{}, domain.WorldEndpoint{}, fmt.Errorf("session service: join world: %w", domain.ErrWorldNotFound)
	}
	slot, ok := sess.RoleSlots[role]
	if !ok {
		s.mu.Unlock()
		return domain.ConnectGrant{}, domain.WorldEndpoint{}, fmt.Errorf("session service: join world: %w", domain.ErrRoleNotEnabled)
	}
	if sess.HostPlayerID == playerID || playerInWorld(sess, playerID) {
		s.mu.Unlock()
		return domain.ConnectGrant{}, domain.WorldEndpoint{}, fmt.Errorf("session service: join world: %w", domain.ErrAlreadyJoined)
	}
	if len(slot.Occupants) >= slot.Capacity {
		s.mu.Unlock()
		return domain.ConnectGrant{}, domain.WorldEndpoint{}, fmt.Errorf("session service: join world: %w", domain.ErrRoleFull)
	}
	slot.Occupants = append(slot.Occupants, playerID)
	// Players acumula PARTICIPANTES (ver domain.Session): re-entrada de quem já
	// morreu neste mundo (novo sinal/invasão) não duplica a lista.
	firstJoin := !slices.Contains(sess.Players, playerID)
	if firstJoin {
		sess.Players = append(sess.Players, playerID)
	}
	sess.Participants = upsertParticipant(sess.Participants, playerID, role, s.now())
	endpoint := domain.WorldEndpoint{ServerAddress: sess.ServerAddress, HostConnection: sess.HostConnection}
	modeID := sess.ModeID
	s.mu.Unlock()

	// claim server: o endereço do GameServer, ou o id de conexão do host em p2p.
	tokenServer := endpoint.ServerAddress
	if endpoint.HostConnection != "" {
		tokenServer = endpoint.HostConnection
	}
	tok, err := s.issuer.Issue(playerID, sessionID, tokenServer, domain.ConnectClaims{Role: role, ModeID: modeID})
	if err != nil {
		s.mu.Lock()
		removeFromWorld(sess, playerID) // reverte a reserva do slot
		if firstJoin {
			// nunca chegou a entrar: não conta como participante
			sess.Players = slices.DeleteFunc(sess.Players, func(p string) bool { return p == playerID })
			sess.Participants = slices.DeleteFunc(sess.Participants, func(p domain.Participant) bool { return p.PlayerID == playerID })
		}
		s.mu.Unlock()
		return domain.ConnectGrant{}, domain.WorldEndpoint{}, fmt.Errorf("session service: join world: issue token: %w", err)
	}
	return domain.ConnectGrant{PlayerID: playerID, Token: tok}, endpoint, nil
}

// AuthorizeWorldHost confere que playerID é o host do mundo aberto sessionID
// (spec 030 / ADR-0030): só o host invoca sinal no próprio mundo e só ele (ou
// o Dedicated Server) fecha o mundo ou tira outro jogador.
func (s *Service) AuthorizeWorldHost(_ context.Context, sessionID, playerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[sessionID]
	if !ok || sess.Origin != domain.OriginHostWorld {
		return fmt.Errorf("session service: authorize host: %w", domain.ErrWorldNotFound)
	}
	if playerID == "" || sess.HostPlayerID != playerID {
		return fmt.Errorf("session service: authorize host: %w", domain.ErrNotWorldHost)
	}
	return nil
}

// ListInvadableWorlds devolve os mundos abertos do jogo com slot de invasor livre
// (spec 012), varrendo o mapa de ativas sob o lock — irmão de SessionsByGame. O
// mapa só contém sessões OPEN (closeSession as remove), então não checa State.
// Fonte de dados pura: nenhum filtro de faixa/weapon/senha (política é do
// matchmaking, 006). Vazio → lista vazia, sem erro. modeID não vazio filtra
// pelo modo em que o mundo foi aberto (spec matchmaking/012); vazio = todos.
func (s *Service) ListInvadableWorlds(_ context.Context, gameID, modeID string) ([]domain.InvadableWorld, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var matches []*domain.Session
	for _, sess := range s.sessions {
		if sess.Origin != domain.OriginHostWorld || sess.GameID != gameID {
			continue
		}
		if modeID != "" && sess.ModeID != modeID {
			continue // outro modo do mesmo jogo (spec matchmaking/012)
		}
		slot, ok := sess.RoleSlots[domain.RoleInvader]
		if !ok {
			continue // papel invasor não habilitado neste mundo
		}
		if slot.Capacity-len(slot.Occupants) <= 0 {
			continue // slot de invasor cheio
		}
		matches = append(matches, sess)
	}
	// Ordena por OpenedAt asc (mundo mais antigo primeiro): iteração de mapa Go
	// é aleatória, e o matchmaking (006) tenta os mundos na ordem devolvida
	// aqui — sem isto, dois JoinWorld concorrentes na mesma faixa podiam
	// escolher mundos diferentes a cada chamada (follow-up #11 do arquétipo).
	sort.Slice(matches, func(i, j int) bool { return matches[i].OpenedAt.Before(matches[j].OpenedAt) })

	out := make([]domain.InvadableWorld, 0, len(matches))
	for _, sess := range matches {
		slot := sess.RoleSlots[domain.RoleInvader]
		out = append(out, domain.InvadableWorld{
			SessionID:        sess.ID,
			HostSoulLevel:    sess.HostSoulLevel,
			WeaponTier:       sess.WeaponTier,
			FreeInvaderSlots: slot.Capacity - len(slot.Occupants),
			PasswordHash:     sess.PasswordHash,
		})
	}
	return out, nil
}

// LeaveWorld libera o slot do phantom; o mundo segue aberto. No-op idempotente se
// o jogador não estiver no mundo. Se o jogador for o host, encerra o mundo.
func (s *Service) LeaveWorld(ctx context.Context, sessionID, playerID string) error {
	s.mu.Lock()
	sess, ok := s.sessions[sessionID]
	if !ok || sess.Origin != domain.OriginHostWorld {
		s.mu.Unlock()
		return fmt.Errorf("session service: leave world: %w", domain.ErrWorldNotFound)
	}
	if sess.HostPlayerID == playerID {
		s.mu.Unlock()
		return s.CloseWorld(ctx, sessionID) // host saindo encerra o mundo
	}
	removeFromWorld(sess, playerID) // no-op se ausente
	s.mu.Unlock()
	return nil
}

// CloseWorld encerra o mundo (CloseHostLeft), libera o server e o tira das ativas.
// Traduz a ausência para ErrWorldNotFound (vocabulário do host_world_dropin).
func (s *Service) CloseWorld(ctx context.Context, sessionID string) error {
	if _, err := s.closeSession(ctx, sessionID, domain.CloseHostLeft); err != nil {
		if errors.Is(err, domain.ErrSessionNotFound) {
			return fmt.Errorf("session service: close world: %w", domain.ErrWorldNotFound)
		}
		return fmt.Errorf("session service: close world: %w", err)
	}
	return nil
}

// ReportParticipationEvent recebe um evento por participante do server do mundo
// (spec 013): valida o vocabulário, aplica o efeito no mundo (slot/fechamento)
// e DEPOIS publica no fan-out (participation_event) — ver o comentário do fix
// 2026-08-19 abaixo. Falha no publish volta como erro para o server reenviar a
// mesma ocorrência; o reenvio do HOST_KILLED que já fechou o mundo só
// republica (spec 029).
func (s *Service) ReportParticipationEvent(ctx context.Context, sessionID string, evt domain.ParticipationEvent) error {
	if err := validateParticipation(evt); err != nil {
		return fmt.Errorf("session service: participation event: %w", err)
	}
	if s.participation == nil {
		return fmt.Errorf("session service: participation event: no participation publisher configured")
	}

	s.mu.Lock()
	sess, ok := s.sessions[sessionID]
	if !ok || sess.Origin != domain.OriginHostWorld {
		closed, retry := s.closingRetryLocked(sessionID, evt)
		s.mu.Unlock()
		if !retry {
			return fmt.Errorf("session service: participation event: %w", domain.ErrWorldNotFound)
		}
		// reenvio da ocorrência que fechou o mundo (spec 029): nada a mutar,
		// só garantir que ela chegue ao fan-out.
		if err := s.participation.PublishParticipation(ctx, closed.GameID, sessionID, closed.ModeID, evt); err != nil {
			return fmt.Errorf("session service: participation event: publish: %w", err)
		}
		s.countParticipation(evt.Event)
		return nil
	}
	gameID, modeID := sess.GameID, sess.ModeID
	// mundo p2p: quem reporta é o jogo do host, não um servidor confiável —
	// o evento só pode ser sobre quem está (ou esteve) no mundo, senão um host
	// forjaria conquistas para qualquer conta (achado da revisão da spec 031).
	untrusted := sess.HostConnection != ""
	participant := hasParticipant(sess, evt.PlayerID)
	s.mu.Unlock()
	if untrusted && !participant {
		return fmt.Errorf("session service: participation event: player not in p2p world: %w", domain.ErrInvalidParticipation)
	}

	// MUTA ANTES de publicar (fix 2026-08-19): o desenho antigo ("publica
	// antes: falha => nada muda") deixava o GAMEPLAY refém da infra de
	// analytics — com o SNS/localstack fora, o LEFT nunca liberava o slot do
	// phantom e o mundo ficava inelegível pra reinvasão (bug achado pela
	// bateria da spec 011 do data plane). Toda mutação aqui é IDEMPOTENTE
	// (overwrite de outcome, removeFromWorld no-op, close NotFound-ok), então
	// o retry do server re-aplica sem efeito colateral; o at-least-once do
	// publish continua garantido pelo ERRO no fim (o server re-reporta com o
	// MESMO event_id — o consumer deduplica).
	s.mu.Lock()
	if cur, ok := s.sessions[sessionID]; ok {
		setParticipantOutcome(cur.Participants, evt.PlayerID, evt.Event, evt.Outcome)
	}
	s.mu.Unlock()

	switch evt.Event {
	case domain.EventDied, domain.EventLeft:
		// libera o slot do phantom; no-op idempotente se o jogador não está lá.
		s.mu.Lock()
		if cur, ok := s.sessions[sessionID]; ok {
			removeFromWorld(cur, evt.PlayerID)
		}
		s.mu.Unlock()
	case domain.EventHostKilled:
		// o host morreu -> encerra o mundo. Guarda a ocorrência na sessão para
		// que o reenvio (publish ou release falho) seja reconhecido depois que
		// o mundo sair do mapa de ativos (spec 029). ErrSessionNotFound = já
		// fechado (ok).
		s.mu.Lock()
		// só a primeira ocorrência marca: dois HOST_KILLED concorrentes (DS e
		// reenvio, ou event_ids diferentes) não trocam a ocorrência que fechou.
		if cur, ok := s.sessions[sessionID]; ok && cur.ClosingEvent == nil {
			closing := evt
			cur.ClosingEvent = &closing
		}
		s.mu.Unlock()
		if _, err := s.closeSession(ctx, sessionID, domain.CloseHostKilled); err != nil &&
			!errors.Is(err, domain.ErrSessionNotFound) {
			return fmt.Errorf("session service: participation event: close: %w", err)
		}
	}

	if err := s.participation.PublishParticipation(ctx, gameID, sessionID, modeID, evt); err != nil {
		return fmt.Errorf("session service: participation event: publish: %w", err)
	}
	s.countParticipation(evt.Event)
	return nil
}

// closingRetryLocked procura no histórico em memória o mundo fechado por esta
// mesma ocorrência (spec 029): mesmo event_id ou, com event_id vazio (data
// plane antigo), mesmo player_id + event. Exige s.mu travado.
func (s *Service) closingRetryLocked(sessionID string, evt domain.ParticipationEvent) (*domain.Session, bool) {
	for i := len(s.history) - 1; i >= 0; i-- {
		h := s.history[i]
		if h.ID != sessionID {
			continue
		}
		c := h.ClosingEvent
		if c == nil || c.Event != evt.Event {
			return nil, false
		}
		if c.EventID != "" || evt.EventID != "" {
			return h, c.EventID == evt.EventID
		}
		return h, c.PlayerID == evt.PlayerID
	}
	return nil, false
}

// validateParticipation valida o vocabulário do evento (spec 013). Não valida o
// role: o server do mundo é autoritativo sobre o que aconteceu.
func validateParticipation(evt domain.ParticipationEvent) error {
	switch evt.Event {
	case domain.EventJoined, domain.EventBossDefeated, domain.EventHostKilled, domain.EventDied, domain.EventLeft:
	default:
		return domain.ErrInvalidParticipation
	}
	switch evt.Outcome {
	case domain.OutcomeSuccess, domain.OutcomeFail, domain.OutcomeNeutral:
	default:
		return domain.ErrInvalidParticipation
	}
	return nil
}

// playerInWorld indica se o jogador já ocupa algum slot do mundo (não conta o host).
func playerInWorld(sess *domain.Session, playerID string) bool {
	for _, slot := range sess.RoleSlots {
		for _, p := range slot.Occupants {
			if p == playerID {
				return true
			}
		}
	}
	return false
}

// removeFromWorld tira o jogador do slot que o contém — libera a CAPACIDADE.
// NÃO mexe em Players: participação é histórica (quem morreu/saiu continua na
// lista da sala; ver domain.Session.Players). Deve ser chamado sob o lock.
// Idempotente: no-op se o jogador não está em nenhum slot.
func removeFromWorld(sess *domain.Session, playerID string) {
	for _, slot := range sess.RoleSlots {
		for i, p := range slot.Occupants {
			if p == playerID {
				slot.Occupants = append(slot.Occupants[:i], slot.Occupants[i+1:]...)
				return
			}
		}
	}
}

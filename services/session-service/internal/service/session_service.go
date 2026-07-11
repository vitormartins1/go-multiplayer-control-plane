// Package service contém o núcleo do session-service: orquestra a criação de
// sessões (aloca server → emite connect tokens → registra) e o encerramento
// (publica resultado → libera server → encerra). Só depende das interfaces de
// domain (ADR-0002). Estado em memória, protegido por mutex.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/domain"
)

var _ domain.SessionService = (*Service)(nil)

// Service orquestra sessões a partir das Ports de saída (allocator, issuer,
// publisher, profiles) e mantém as sessões ativas em memória.
type Service struct {
	allocator     domain.ServerAllocator
	issuer        domain.ConnectTokenIssuer
	publisher     domain.ResultPublisher
	participation domain.ParticipationPublisher // fan-out dos eventos de participação (spec 013)
	profiles      domain.ProfileProvider        // lê o perfil do jogo (host_world_dropin); spec 011
	now           func() time.Time              // injetável: o sweeper de TTL precisa ser testável

	// reconciliação de órfãs (spec 008): registry nil = sem fonte de verdade de
	// liveness (modo static) → só o TTL sweeper roda.
	registry    domain.ServerRegistry
	orphanGrace time.Duration

	mu       sync.Mutex
	sessions map[string]*domain.Session // sessionID -> sessão ativa (spec 011)
	// matchIndex mapeia matchID -> sessionID para o caminho queue_arena, cuja
	// identidade externa é o matchID. host_world não popula (identidade = sessionID).
	matchIndex map[string]string
	closed     map[domain.CloseReason]float64
	// contador de eventos de participação por tipo (métrica participation_events_total).
	participationEvents map[string]float64

	// histórico bounded de fechadas (spec 005): ring em memória, best-effort
	// de dev — some no restart, persistência é evolução.
	history      []*domain.Session
	historyLimit int
}

// Option configura o Service na construção (só testes precisam disso).
type Option func(*Service)

// WithClock troca o relógio (default time.Now) — o sweeper decide TTL por ele.
func WithClock(now func() time.Time) Option {
	return func(s *Service) { s.now = now }
}

// WithHistoryLimit define o tamanho do histórico de fechadas (default 200).
func WithHistoryLimit(n int) Option {
	return func(s *Service) {
		if n > 0 {
			s.historyLimit = n
		}
	}
}

// WithProfileProvider liga a leitura do perfil de multiplayer (specs 011/017).
// Desde o ADR-0024 é OBRIGATÓRIO para criar sessões: sem ele, Create falha com
// ErrProfileUnavailable e OpenWorld com ErrUnsupportedArchetype (wiring bug).
func WithProfileProvider(p domain.ProfileProvider) Option {
	return func(s *Service) { s.profiles = p }
}

// WithParticipationPublisher liga o fan-out dos eventos de participação (spec 013,
// host_world_dropin). Sem ele, ReportParticipationEvent responde erro (mundo
// host_world sem fan-out configurado é bug de wire-up).
func WithParticipationPublisher(p domain.ParticipationPublisher) Option {
	return func(s *Service) { s.participation = p }
}

// WithServerRegistry liga a reconciliação de órfãs (spec 008): o sweeper passa a
// fechar sessões cujo game server sumiu da fonte de verdade. Sem esta option
// (registry nil), o service só reconcilia por TTL.
func WithServerRegistry(r domain.ServerRegistry) Option {
	return func(s *Service) { s.registry = r }
}

// WithOrphanGrace define a janela mínima de vida antes de uma sessão ser
// candidata a fechamento por reconciliação (default 60s) — tolera staleness do
// LIST e alocação recém-propagada.
func WithOrphanGrace(d time.Duration) Option {
	return func(s *Service) {
		if d > 0 {
			s.orphanGrace = d
		}
	}
}

// New cria o service a partir das Ports de saída. O ProfileProvider (host_world
// _dropin) é opcional via WithProfileProvider — sem ele, OpenWorld falha com
// ErrUnsupportedArchetype e o queue_arena segue intacto.
func New(allocator domain.ServerAllocator, issuer domain.ConnectTokenIssuer, publisher domain.ResultPublisher, opts ...Option) *Service {
	s := &Service{
		allocator:           allocator,
		issuer:              issuer,
		publisher:           publisher,
		now:                 time.Now,
		orphanGrace:         time.Minute,
		sessions:            make(map[string]*domain.Session),
		matchIndex:          make(map[string]string),
		closed:              make(map[domain.CloseReason]float64),
		participationEvents: make(map[string]float64),
		historyLimit:        200,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Create aloca um game server, emite um connect token por jogador e registra a
// sessão (OPEN). O perfil do MODO é congelado como snapshot ANTES de alocar
// (ADR-0024, spec 017) — se o perfil não puder ser lido, nada é reservado. Se a
// emissão de token falhar, o server alocado é devolvido ao pool.
func (s *Service) Create(ctx context.Context, matchID, gameID, modeID string, teams []domain.TeamAssignment) (*domain.Session, []domain.ConnectGrant, error) {
	players, teamByPlayer, err := flattenTeams(teams)
	if err != nil {
		return nil, nil, fmt.Errorf("session service: create: %w", err)
	}

	snap, err := s.modeSnapshot(ctx, gameID, modeID)
	if err != nil {
		return nil, nil, fmt.Errorf("session service: create: %w", err)
	}

	server, err := s.allocator.Allocate(ctx, matchID, gameID, snap.Fleet)
	if err != nil {
		return nil, nil, fmt.Errorf("session service: create: allocate: %w", err)
	}

	grants := make([]domain.ConnectGrant, 0, len(players))
	for _, playerID := range players {
		// o token carrega o team_id do jogador (vazio em FFA) e o mode_id (sempre)
		// — o game server lê ambos no connection approval (ADR-0023/0024).
		tok, err := s.issuer.Issue(playerID, matchID, server.Address, domain.ConnectClaims{TeamID: teamByPlayer[playerID], ModeID: modeID})
		if err != nil {
			// rollback: não deixa o server preso se não conseguimos cunhar os tokens.
			_ = s.allocator.Release(ctx, server.ID)
			return nil, nil, fmt.Errorf("session service: create: issue token: %w", err)
		}
		grants = append(grants, domain.ConnectGrant{PlayerID: playerID, Token: tok})
	}

	sess := &domain.Session{
		ID:            uuid.NewString(),
		MatchID:       matchID,
		GameID:        gameID,
		ModeID:        modeID,
		Profile:       &snap,
		Players:       players,
		TeamByPlayer:  teamByPlayer,
		ServerID:      server.ID,
		ServerAddress: server.Address,
		Origin:        domain.OriginMatch,
		State:         domain.SessionOpen,
		OpenedAt:      s.now(),
	}

	s.mu.Lock()
	s.sessions[sess.ID] = sess
	s.matchIndex[matchID] = sess.ID // identidade externa do queue_arena é o matchID
	s.mu.Unlock()

	return sess, grants, nil
}

// flattenTeams achata a composição em uma lista ordenada de players e um mapa
// playerID->teamID (só entradas com time; FFA devolve mapa vazio). Preserva a
// ordem dos assignments/players. ErrInvalidTeams se um jogador aparece em mais
// de um time (a única inconsistência possível quando os teams são a fonte).
func flattenTeams(teams []domain.TeamAssignment) ([]string, map[string]string, error) {
	var players []string
	teamByPlayer := make(map[string]string)
	seen := make(map[string]bool)
	for _, t := range teams {
		for _, p := range t.Players {
			if seen[p] {
				return nil, nil, domain.ErrInvalidTeams
			}
			seen[p] = true
			players = append(players, p)
			if t.TeamID != "" {
				teamByPlayer[p] = t.TeamID
			}
		}
	}
	return players, teamByPlayer, nil
}

// ReportResult publica o resultado no fan-out e, só então, libera o server e
// encerra a sessão (CLOSED/result). ErrSessionNotFound se não houver sessão para
// o match.
//
// Ordem invertida vs. specs 002/003 (ADR-0020): publica ANTES de fechar. Se a
// publicação falhar, retorna erro e NÃO encerra a sessão — o Dedicated Server
// refaz o ReportResult (retry, Fase 6 do data plane) e o TTL sweeper cobre a
// sessão órfã. O resultado não se perde silenciosamente.
func (s *Service) ReportResult(ctx context.Context, matchID string, outcome domain.MatchOutcome) error {
	if err := validateOutcome(outcome); err != nil {
		return fmt.Errorf("session service: report result: %w", err)
	}

	s.mu.Lock()
	sessionID, ok := s.matchIndex[matchID]
	var sess *domain.Session
	if ok {
		sess, ok = s.sessions[sessionID]
	}
	gameID, modeID := "", ""
	var snap *domain.ProfileSnapshot
	if ok {
		gameID, modeID, snap = sess.GameID, sess.ModeID, sess.Profile
	}
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("session service: report result: %w", domain.ErrSessionNotFound)
	}

	// kind × arquétipo do snapshot (ADR-0024, spec 017): um server não consegue
	// reportar a FORMA errada de resultado — nada é publicado no mismatch e a
	// sessão segue aberta (o server pode corrigir e reenviar).
	if err := validateOutcomeKind(snap, outcome.Kind); err != nil {
		return fmt.Errorf("session service: report result: %w", err)
	}

	// carimba o team_id canônico a partir da composição registrada na formação
	// (ADR-0023): o session é o dono do rótulo, não o que o game server ecoou.
	// TeamByPlayer é imutável após o Create, então é seguro ler fora do lock.
	stampTeams(outcome, sess.TeamByPlayer)

	if err := s.publisher.Publish(ctx, gameID, matchID, modeID, outcome); err != nil {
		return fmt.Errorf("session service: report result: publish: %w", err)
	}

	// Grava o outcome publicado na sessão (spec 018): o admin mostra
	// exatamente o que saiu no fan-out (team_id carimbado). Sob lock — o
	// sweeper/reconciler podem fechar a sessão concorrentemente.
	s.mu.Lock()
	sess.Result = &outcome
	s.mu.Unlock()

	if _, err := s.closeSession(ctx, sessionID, domain.CloseResult); err != nil {
		return fmt.Errorf("session service: report result: %w", err)
	}
	return nil
}

// stampTeams sobrescreve o TeamID de cada entrada com o time registrado na
// formação (ADR-0023). Jogador fora da composição (ou FFA) fica com TeamID
// vazio. Muta in-place via o ponteiro do kind ativo (tagged union do outcome).
func stampTeams(o domain.MatchOutcome, teamByPlayer map[string]string) {
	if o.WinLossDraw != nil {
		for i := range o.WinLossDraw.Players {
			o.WinLossDraw.Players[i].TeamID = teamByPlayer[o.WinLossDraw.Players[i].PlayerID]
		}
	}
	if o.Placement != nil {
		for i := range o.Placement.Entries {
			o.Placement.Entries[i].TeamID = teamByPlayer[o.Placement.Entries[i].PlayerID]
		}
	}
	if o.Collective != nil {
		for i := range o.Collective.Players {
			o.Collective.Players[i].TeamID = teamByPlayer[o.Collective.Players[i].PlayerID]
		}
	}
}

// modeSnapshot lê e congela o perfil do modo (ADR-0024, spec 017). profiles nil
// é erro de wiring — aparece como ErrProfileUnavailable, alto e cedo.
func (s *Service) modeSnapshot(ctx context.Context, gameID, modeID string) (domain.ProfileSnapshot, error) {
	if s.profiles == nil {
		return domain.ProfileSnapshot{}, fmt.Errorf("profile provider não configurado: %w", domain.ErrProfileUnavailable)
	}
	snap, err := s.profiles.ModeProfile(ctx, gameID, modeID)
	if err != nil {
		return domain.ProfileSnapshot{}, err
	}
	return snap, nil
}

// kindByArchetype é o kind de outcome terminal que cada arquétipo produz
// (ADR-0024, spec 017). host_world_dropin não está aqui: usa
// ReportParticipationEvent — qualquer ReportResult é mismatch.
var kindByArchetype = map[string]string{
	domain.ArchetypeQueueArena:   domain.OutcomeWinLossDraw,
	domain.ArchetypeBattleRoyale: domain.OutcomePlacement,
	domain.ArchetypeCoopMission:  domain.OutcomeCollective,
}

// validateOutcomeKind confronta o kind reportado com o arquétipo do snapshot
// (spec 017). Snapshot ausente não deve ocorrer (o Create o exige) — tratado
// como mismatch para não deixar passar resultado sem procedência.
func validateOutcomeKind(snap *domain.ProfileSnapshot, kind string) error {
	if snap == nil {
		return fmt.Errorf("sessão sem snapshot de perfil: %w", domain.ErrOutcomeMismatch)
	}
	expected, ok := kindByArchetype[snap.Archetype]
	if !ok || expected != kind {
		return fmt.Errorf("arquétipo %q não produz outcome %q: %w", snap.Archetype, kind, domain.ErrOutcomeMismatch)
	}
	return nil
}

// GetSessionConfig devolve o session manifest do snapshot (ADR-0024, spec 017).
// refID resolve primeiro como matchID (queue-form) e depois como sessionID
// (host_world) — a mesma rechaveação do restante do service.
func (s *Service) GetSessionConfig(ctx context.Context, refID string) (*domain.SessionManifest, error) {
	s.mu.Lock()
	sessionID, ok := s.matchIndex[refID]
	if !ok {
		sessionID = refID
	}
	sess, ok := s.sessions[sessionID]
	s.mu.Unlock()
	if !ok || sess.State != domain.SessionOpen || sess.Profile == nil {
		return nil, fmt.Errorf("session service: get session config: %w", domain.ErrSessionNotFound)
	}
	snap := sess.Profile
	return &domain.SessionManifest{
		SessionID:   sess.ID,
		MatchID:     sess.MatchID,
		GameID:      sess.GameID,
		ModeID:      sess.ModeID,
		Archetype:   snap.Archetype,
		Topology:    snap.Topology,
		ParamsJSON:  append([]byte(nil), snap.ParamsJSON...),
		ProfileHash: snap.Hash,
	}, nil
}

// validateOutcome valida o outcome estruturalmente, despachando por kind
// (ADR-0023). A coerência kind×arquétipo é de validateOutcomeKind (spec 017);
// dentro do kind, o game server é autoritativo sobre o resultado (não recomputa
// a corrida nem exige colocações contíguas).
func validateOutcome(o domain.MatchOutcome) error {
	switch o.Kind {
	case domain.OutcomeWinLossDraw:
		if o.WinLossDraw == nil || len(o.WinLossDraw.Players) == 0 {
			return domain.ErrInvalidOutcome
		}
		for _, p := range o.WinLossDraw.Players {
			switch p.Outcome {
			case domain.ResultWin, domain.ResultLoss, domain.ResultDraw:
			default:
				return domain.ErrInvalidOutcome
			}
		}
		return nil
	case domain.OutcomePlacement:
		if o.Placement == nil || len(o.Placement.Entries) == 0 {
			return domain.ErrInvalidOutcome
		}
		// estrutural: colocação >= 1 e sobrevivência >= 0. Empates e
		// não-contiguidade são permitidos (server autoritativo, ficha §5).
		for _, e := range o.Placement.Entries {
			if e.Placement < 1 || e.SurvivedMs < 0 {
				return domain.ErrInvalidOutcome
			}
		}
		return nil
	case domain.OutcomeCollective:
		if o.Collective == nil || len(o.Collective.Players) == 0 {
			return domain.ErrInvalidOutcome
		}
		// estrutural: o desfecho da sessão está no vocabulário. Os valores de
		// stats NÃO são checados (server autoritativo — como no placement, que
		// só valida survivedMs, um campo dedicado, não o mapa stats).
		if !missionOutcomes[o.Collective.Outcome] {
			return domain.ErrInvalidOutcome
		}
		return nil
	default:
		return domain.ErrUnsupportedOutcome
	}
}

// missionOutcomes é o vocabulário do desfecho coletivo do coop_mission (C2).
var missionOutcomes = map[string]bool{
	domain.MissionSuccess: true,
	domain.MissionFailure: true,
}

// closeSession é o fechamento unificado (spec 004): marca CLOSED com motivo,
// tira do mapa de ativas e libera o server — usado por ReportResult (result),
// pelo sweeper (ttl/abandoned) e pelo host_world (host_left). Opera pela chave
// interna (sessionID). Libera o server PRIMEIRO em relação a qualquer fan-out:
// o fim da partida não pode depender de destino externo.
func (s *Service) closeSession(ctx context.Context, sessionID string, reason domain.CloseReason) (*domain.Session, error) {
	s.mu.Lock()
	sess, ok := s.sessions[sessionID]
	if !ok {
		s.mu.Unlock()
		return nil, domain.ErrSessionNotFound
	}
	delete(s.sessions, sessionID)
	if sess.MatchID != "" {
		delete(s.matchIndex, sess.MatchID) // queue_arena: limpa o índice reverso
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

	if err := s.allocator.Release(ctx, sess.ServerID); err != nil {
		return sess, fmt.Errorf("release: %w", err)
	}
	return sess, nil
}

// Sweep fecha (CLOSED/ttl) as sessões abertas há mais que ttl e devolve
// quantas fechou. Erros de release não interrompem a varredura.
func (s *Service) Sweep(ctx context.Context, ttl time.Duration) int {
	cutoff := s.now().Add(-ttl)
	s.mu.Lock()
	var expired []string
	for sessionID, sess := range s.sessions {
		if sess.OpenedAt.Before(cutoff) {
			expired = append(expired, sessionID)
		}
	}
	s.mu.Unlock()

	for _, sessionID := range expired {
		// ErrSessionNotFound = fechada por result entre a varredura e o close; ok.
		_, _ = s.closeSession(ctx, sessionID, domain.CloseTTL)
	}
	return len(expired)
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

// OpenWorld abre um mundo persistente para o host (host_world_dropin, spec 011):
// lê o perfil, aloca um server, deriva os slots por papel e emite o connect
// token do host. Qualquer problema com o perfil (ausente, arquétipo != host_world
// _dropin, rede) vira ErrUnsupportedArchetype — só esse arquétipo abre mundo.
func (s *Service) OpenWorld(ctx context.Context, hostPlayerID, gameID, modeID string, attrs domain.WorldAttributes) (*domain.Session, domain.ConnectGrant, error) {
	if s.profiles == nil {
		return nil, domain.ConnectGrant{}, fmt.Errorf("session service: open world: %w", domain.ErrUnsupportedArchetype)
	}
	snap, err := s.modeSnapshot(ctx, gameID, modeID)
	if err != nil {
		return nil, domain.ConnectGrant{}, fmt.Errorf("session service: open world: %v: %w", err, domain.ErrUnsupportedArchetype)
	}
	// a checagem de arquétipo subiu do adapter (que virou genérico por modo,
	// ADR-0024) para cá: só host_world_dropin abre mundo.
	if snap.Archetype != domain.ArchetypeHostWorldDropIn {
		return nil, domain.ConnectGrant{}, fmt.Errorf("session service: open world: archetype %q: %w", snap.Archetype, domain.ErrUnsupportedArchetype)
	}

	sessionID := uuid.NewString()
	server, err := s.allocator.Allocate(ctx, sessionID, gameID, snap.Fleet)
	if err != nil {
		return nil, domain.ConnectGrant{}, fmt.Errorf("session service: open world: allocate: %w", err)
	}

	// token do host (papel host) — rollback do server se a emissão falhar.
	tok, err := s.issuer.Issue(hostPlayerID, sessionID, server.Address, domain.ConnectClaims{Role: domain.RoleHost, ModeID: modeID})
	if err != nil {
		_ = s.allocator.Release(ctx, server.ID)
		return nil, domain.ConnectGrant{}, fmt.Errorf("session service: open world: issue token: %w", err)
	}

	sess := &domain.Session{
		ID:            sessionID,
		GameID:        gameID,
		ModeID:        modeID,
		Profile:       &snap,
		Players:       []string{hostPlayerID},
		ServerID:      server.ID,
		ServerAddress: server.Address,
		Origin:        domain.OriginHostWorld,
		HostPlayerID:  hostPlayerID,
		RoleSlots:     roleSlotsFromProfile(snap),
		HostSoulLevel: attrs.SoulLevel,
		WeaponTier:    attrs.WeaponTier,
		PasswordHash:  domain.HashWorldPassword(attrs.Password),
		State:         domain.SessionOpen,
		OpenedAt:      s.now(),
	}

	s.mu.Lock()
	s.sessions[sessionID] = sess // host_world: identidade = sessionID, sem matchIndex
	s.mu.Unlock()

	return sess, domain.ConnectGrant{PlayerID: hostPlayerID, Token: tok}, nil
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
func (s *Service) JoinWorld(ctx context.Context, sessionID, playerID, role string) (domain.ConnectGrant, string, error) {
	s.mu.Lock()
	sess, ok := s.sessions[sessionID]
	if !ok || sess.Origin != domain.OriginHostWorld {
		s.mu.Unlock()
		return domain.ConnectGrant{}, "", fmt.Errorf("session service: join world: %w", domain.ErrWorldNotFound)
	}
	slot, ok := sess.RoleSlots[role]
	if !ok {
		s.mu.Unlock()
		return domain.ConnectGrant{}, "", fmt.Errorf("session service: join world: %w", domain.ErrRoleNotEnabled)
	}
	if sess.HostPlayerID == playerID || playerInWorld(sess, playerID) {
		s.mu.Unlock()
		return domain.ConnectGrant{}, "", fmt.Errorf("session service: join world: %w", domain.ErrAlreadyJoined)
	}
	if len(slot.Occupants) >= slot.Capacity {
		s.mu.Unlock()
		return domain.ConnectGrant{}, "", fmt.Errorf("session service: join world: %w", domain.ErrRoleFull)
	}
	slot.Occupants = append(slot.Occupants, playerID)
	sess.Players = append(sess.Players, playerID)
	serverAddr := sess.ServerAddress
	modeID := sess.ModeID
	s.mu.Unlock()

	tok, err := s.issuer.Issue(playerID, sessionID, serverAddr, domain.ConnectClaims{Role: role, ModeID: modeID})
	if err != nil {
		s.mu.Lock()
		removeFromWorld(sess, playerID) // reverte a reserva do slot
		s.mu.Unlock()
		return domain.ConnectGrant{}, "", fmt.Errorf("session service: join world: issue token: %w", err)
	}
	return domain.ConnectGrant{PlayerID: playerID, Token: tok}, serverAddr, nil
}

// ListInvadableWorlds devolve os mundos abertos do jogo com slot de invasor livre
// (spec 012), varrendo o mapa de ativas sob o lock — irmão de SessionsByGame. O
// mapa só contém sessões OPEN (closeSession as remove), então não checa State.
// Fonte de dados pura: nenhum filtro de faixa/weapon/senha (política é do
// matchmaking, 006). Vazio → lista vazia, sem erro.
func (s *Service) ListInvadableWorlds(_ context.Context, gameID string) ([]domain.InvadableWorld, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var matches []*domain.Session
	for _, sess := range s.sessions {
		if sess.Origin != domain.OriginHostWorld || sess.GameID != gameID {
			continue
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
// (spec 013): valida o vocabulário, publica no fan-out (participation_event) e,
// só após o publish, aplica o efeito no slot. Publica ANTES de mutar (ADR-0020):
// se a publicação falha, o estado não muda e o server refaz o report.
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
		s.mu.Unlock()
		return fmt.Errorf("session service: participation event: %w", domain.ErrWorldNotFound)
	}
	gameID, modeID := sess.GameID, sess.ModeID
	s.mu.Unlock()

	// publica ANTES de mutar: falha => nada muda, o server refaz o report.
	if err := s.participation.PublishParticipation(ctx, gameID, sessionID, modeID, evt); err != nil {
		return fmt.Errorf("session service: participation event: publish: %w", err)
	}
	s.countParticipation(evt.Event)

	switch evt.Event {
	case domain.EventDied, domain.EventLeft:
		// libera o slot do phantom; re-lookup sob o lock (o mundo pode ter fechado
		// entre o publish e a mutação). No-op idempotente se o jogador não está lá.
		s.mu.Lock()
		if cur, ok := s.sessions[sessionID]; ok {
			removeFromWorld(cur, evt.PlayerID)
		}
		s.mu.Unlock()
	case domain.EventHostKilled:
		// o host morreu -> encerra o mundo. ErrSessionNotFound = já fechado (ok).
		if _, err := s.closeSession(ctx, sessionID, domain.CloseHostKilled); err != nil &&
			!errors.Is(err, domain.ErrSessionNotFound) {
			return fmt.Errorf("session service: participation event: close: %w", err)
		}
	}
	return nil
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

// removeFromWorld tira o jogador do slot que o contém e da lista de players.
// Deve ser chamado sob o lock. Idempotente: no-op se o jogador não está presente.
func removeFromWorld(sess *domain.Session, playerID string) {
	for _, slot := range sess.RoleSlots {
		for i, p := range slot.Occupants {
			if p == playerID {
				slot.Occupants = append(slot.Occupants[:i], slot.Occupants[i+1:]...)
				for j, pp := range sess.Players {
					if pp == playerID {
						sess.Players = append(sess.Players[:j], sess.Players[j+1:]...)
						break
					}
				}
				return
			}
		}
	}
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

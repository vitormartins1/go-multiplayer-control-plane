// Package service contém o núcleo do session-service: orquestra a criação de
// sessões (aloca server → emite connect tokens → registra) e o encerramento
// (publica resultado → libera server → encerra). Só depende das interfaces de
// domain (ADR-0002). Estado em memória, protegido por mutex.
package service

import (
	"context"
	"fmt"
	"log/slog"
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

	log *slog.Logger // falhas best-effort (persistência de histórico, spec 025)

	mu       sync.Mutex
	sessions map[string]*domain.Session // sessionID -> sessão ativa (spec 011)
	// matchIndex mapeia matchID -> sessionID para o caminho queue_arena, cuja
	// identidade externa é o matchID. host_world não popula (identidade = sessionID).
	matchIndex map[string]string
	// worldTokens mapeia hash do world token -> mundo p2p (spec 031). Ao
	// fechar, a entrada fica marcada como fechada por um tempo (closedTokens,
	// mesmo limite do histórico): o host ainda precisa reenviar o HOST_KILLED
	// cujo publish falhou (spec 029) — e só isso.
	worldTokens  map[string]worldTokenEntry
	closedTokens []string // hashes fechados, do mais antigo ao mais novo
	// heartbeat de mundo (spec 032): TTL e contador da métrica.
	heartbeatTTL time.Duration
	heartbeats   float64
	closed       map[domain.CloseReason]float64
	// contador de eventos de participação por tipo (métrica participation_events_total).
	participationEvents map[string]float64

	// histórico bounded de fechadas (spec 005): ring em memória, best-effort
	// de dev — serve o "recém-fechado" sem ida ao banco e é o fallback se o
	// Postgres estiver fora (spec 025).
	history      []*domain.Session
	historyLimit int

	// historyStore persiste sessões fechadas (spec 025). nil = sem persistência
	// configurada (dev sem Postgres, ou testes) — closeSession pula a gravação
	// sem erro, o ring in-memory continua sendo a única fonte.
	historyStore           domain.HistoryStore
	historyPersistFailures float64

	// sessionClosedPublisher alimenta o "session health" do analytics-service
	// (spec 028). nil = sem fan-out configurado — closeSession pula a
	// publicação sem erro (mesmo espírito do historyStore).
	sessionClosedPublisher       domain.SessionClosedPublisher
	sessionClosedPublishFailures float64
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

// WithHistoryStore liga a persistência de sessões fechadas (spec 025). Sem
// esta option (historyStore nil), closeSession não grava em lugar nenhum além
// do ring in-memory — dev sem Postgres configurado segue funcionando, só sem
// histórico sobrevivendo a restart.
func WithHistoryStore(store domain.HistoryStore) Option {
	return func(s *Service) { s.historyStore = store }
}

// WithLogger troca o logger (default slog.Default()) — usado só para falhas
// best-effort que não podem propagar como erro (persistência de histórico,
// spec 025).
func WithLogger(log *slog.Logger) Option {
	return func(s *Service) { s.log = log }
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

// WithSessionClosedPublisher liga o fan-out do fechamento de sessão (spec
// 028, session health do analytics-service). Sem esta option
// (sessionClosedPublisher nil), closeSession não publica nada além do que
// já publicava — dev sem esse adapter configurado segue funcionando.
func WithSessionClosedPublisher(p domain.SessionClosedPublisher) Option {
	return func(s *Service) { s.sessionClosedPublisher = p }
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
		heartbeatTTL:        DefaultWorldHeartbeatTTL,
		log:                 slog.Default(),
		sessions:            make(map[string]*domain.Session),
		matchIndex:          make(map[string]string),
		worldTokens:         make(map[string]worldTokenEntry),
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

	openedAt := s.now()
	sess := &domain.Session{
		ID:            uuid.NewString(),
		MatchID:       matchID,
		GameID:        gameID,
		ModeID:        modeID,
		Profile:       &snap,
		Players:       players,
		Participants:  participantsFromTeams(players, teamByPlayer, openedAt),
		TeamByPlayer:  teamByPlayer,
		ServerID:      server.ID,
		ServerAddress: server.Address,
		Origin:        domain.OriginMatch,
		State:         domain.SessionOpen,
		OpenedAt:      openedAt,
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

// participantsFromTeams monta os participantes iniciais de uma sessão de match
// (spec 025): papel vazio em FFA, "team:<id>" quando o jogador tem time. Todos
// entram no mesmo instante — a criação da sessão é atômica (diferente do
// host_world, onde phantoms entram em momentos distintos via JoinWorld).
func participantsFromTeams(players []string, teamByPlayer map[string]string, at time.Time) []domain.Participant {
	out := make([]domain.Participant, 0, len(players))
	for _, p := range players {
		role := ""
		if team, ok := teamByPlayer[p]; ok {
			role = "team:" + team
		}
		out = append(out, domain.Participant{PlayerID: p, Role: role, JoinedAt: at})
	}
	return out
}

// upsertParticipant grava/atualiza um participante do mundo host_world (spec
// 025): dedupe por PlayerID — mesmo papel na re-entrada, mantém a entrada
// original (sem duplicar); papel diferente, atualiza para refletir a
// ocorrência mais recente (o jogador trocou de papel, ex.: saiu como coop e
// voltou como invader).
func upsertParticipant(participants []domain.Participant, playerID, role string, joinedAt time.Time) []domain.Participant {
	for i, p := range participants {
		if p.PlayerID == playerID {
			if p.Role != role {
				participants[i].Role = role
				participants[i].JoinedAt = joinedAt
				// troca de papel = nova ocorrência de participação (spec 027):
				// o último evento/outcome conhecido não se aplica mais.
				participants[i].LastEvent = ""
				participants[i].LastOutcome = ""
			}
			return participants
		}
	}
	return append(participants, domain.Participant{PlayerID: playerID, Role: role, JoinedAt: joinedAt})
}

// setParticipantOutcome grava o último evento/outcome de participação
// conhecido (spec 027) no participante correspondente, por PlayerID. No-op
// se o participante não existe (evento reportado sem JoinWorld/OpenWorld
// prévio — não é papel deste evento criar participante).
func setParticipantOutcome(participants []domain.Participant, playerID, event, outcome string) {
	for i, p := range participants {
		if p.PlayerID == playerID {
			participants[i].LastEvent = event
			participants[i].LastOutcome = outcome
			return
		}
	}
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
	alreadyReported := !ok && s.resultReportedLocked(matchID)
	s.mu.Unlock()
	if alreadyReported {
		// reenvio de um resultado já aceito (spec 033): a resposta anterior se
		// perdeu ou o release falhou depois do publish. Sucesso, sem republicar.
		return nil
	}
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

// resultReportedLocked diz se o match está no histórico em memória fechado por
// resultado (spec 033). Exige s.mu travado.
func (s *Service) resultReportedLocked(matchID string) bool {
	for i := len(s.history) - 1; i >= 0; i-- {
		if h := s.history[i]; h.MatchID == matchID {
			return h.CloseReason == domain.CloseResult
		}
	}
	return false
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
	// State é escrito sob o lock (closeSession): a leitura também
	open := ok && sess.State == domain.SessionOpen && sess.Profile != nil
	s.mu.Unlock()
	if !open {
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

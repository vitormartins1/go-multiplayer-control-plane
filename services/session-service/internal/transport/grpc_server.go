// Package transport traduz gRPC para o domínio (unary) e mapeia os erros de
// domínio para gRPC status codes. Sem lógica de negócio. Ver ADR-0002 e ADR-0003.
package transport

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/gen/session/v1"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/domain"
)

// Server implementa o serviço gRPC Session.
type Server struct {
	pb.UnimplementedSessionServer
	svc  domain.SessionService
	auth *auth // nil = autenticação desligada (testes antigos, dev pontual)
}

// ServerOption configura o Server.
type ServerOption func(*Server)

// WithAuth liga as checagens por mundo dos handlers (spec 030) no mesmo modo
// do interceptor — o interceptor identifica, os handlers conferem o host.
func WithAuth(cfg AuthConfig) ServerOption {
	return func(s *Server) { s.auth = &auth{cfg: cfg} }
}

// NewServer cria o server a partir do SessionService (Port de entrada).
func NewServer(svc domain.SessionService, opts ...ServerOption) *Server {
	s := &Server{svc: svc}
	for _, o := range opts {
		o(s)
	}
	return s
}

// deny registra uma permissão negada no nível do handler conforme o modo
// (nil em warn/off ou sem auth configurada).
func (s *Server) deny(ctx context.Context, method, reason string) error {
	if s.auth == nil || s.auth.cfg.Mode == AuthOff {
		return nil
	}
	return s.auth.reject(ctx, method, reason, codes.PermissionDenied)
}

// hostCheck avalia uma ação que só o host pode fazer, conforme o modo: mundo
// inexistente vira NotFound sempre; "não é o host" vira PermissionDenied em
// enforce e só é registrado em warn.
func (s *Server) hostCheck(ctx context.Context, method, sessionID, playerID string) error {
	if s.auth == nil || s.auth.cfg.Mode == AuthOff {
		return nil
	}
	err := s.svc.AuthorizeWorldHost(ctx, sessionID, playerID)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrNotWorldHost):
		return s.auth.reject(ctx, method, "not_world_host", codes.PermissionDenied)
	default:
		return worldError(err, "authorize")
	}
}

// worldScope confere que o host de mundo p2p (world token, spec 031) só toca o
// próprio mundo. Outros chamadores passam direto.
func (s *Server) worldScope(ctx context.Context, method, sessionID string) error {
	if c := callerFrom(ctx); c.Kind == CallerWorldHost && c.SessionID != sessionID {
		return s.deny(ctx, method, "other_world")
	}
	return nil
}

// normalizeMode aplica a retrocompat do ADR-0024 na borda: mode_id ausente/
// vazio = "default". Domain/service sempre recebem um modo não-vazio.
func normalizeMode(modeID string) string {
	if modeID == "" {
		return domain.DefaultModeID
	}
	return modeID
}

// CreateSession aloca um server, emite os connect tokens e registra a sessão.
func (s *Server) CreateSession(ctx context.Context, req *pb.CreateSessionRequest) (*pb.CreateSessionResponse, error) {
	sess, grants, err := s.svc.Create(ctx, req.GetMatchId(), req.GetGameId(), normalizeMode(req.GetModeId()), toTeamAssignments(req))
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNoServer):
			return nil, status.Error(codes.ResourceExhausted, "no server available")
		case errors.Is(err, domain.ErrInvalidTeams):
			return nil, status.Error(codes.InvalidArgument, "invalid team composition")
		case errors.Is(err, domain.ErrProfileUnavailable):
			return nil, status.Error(codes.FailedPrecondition, "mode profile unavailable")
		case errors.Is(err, domain.ErrFleetUnavailable):
			return nil, status.Error(codes.FailedPrecondition, "fleet not configured")
		default:
			return nil, status.Error(codes.Internal, "create session")
		}
	}

	pbGrants := make([]*pb.ConnectGrant, len(grants))
	for i, g := range grants {
		pbGrants[i] = &pb.ConnectGrant{PlayerId: g.PlayerID, ConnectToken: g.Token}
	}
	return &pb.CreateSessionResponse{
		SessionId:     sess.ID,
		ServerAddress: sess.ServerAddress,
		Grants:        pbGrants,
	}, nil
}

// toTeamAssignments traduz o request para a composição do domínio. Com times
// (queue_arena/teams) usa req.teams; sem times (FFA) devolve um único
// assignment sem TeamID com todos os req.players.
func toTeamAssignments(req *pb.CreateSessionRequest) []domain.TeamAssignment {
	if teams := req.GetTeams(); len(teams) > 0 {
		out := make([]domain.TeamAssignment, len(teams))
		for i, t := range teams {
			out[i] = domain.TeamAssignment{TeamID: t.GetTeamId(), Players: t.GetPlayers()}
		}
		return out
	}
	return []domain.TeamAssignment{{Players: req.GetPlayers()}}
}

// ReportResult publica o resultado, libera o server e encerra a sessão. Traduz
// o outcome polimórfico (oneof) para o domínio (ADR-0023).
func (s *Server) ReportResult(ctx context.Context, req *pb.ReportResultRequest) (*pb.ReportResultResponse, error) {
	outcome := toMatchOutcome(req)

	if err := s.svc.ReportResult(ctx, req.GetMatchId(), outcome); err != nil {
		switch {
		case errors.Is(err, domain.ErrSessionNotFound):
			return nil, status.Error(codes.NotFound, "session not found")
		case errors.Is(err, domain.ErrUnsupportedOutcome):
			return nil, status.Error(codes.InvalidArgument, "unsupported outcome kind")
		case errors.Is(err, domain.ErrInvalidOutcome):
			return nil, status.Error(codes.InvalidArgument, "invalid outcome")
		case errors.Is(err, domain.ErrOutcomeMismatch):
			// spec 017: a FORMA do resultado não bate com o arquétipo do modo.
			return nil, status.Error(codes.FailedPrecondition, "outcome kind does not match session archetype")
		default:
			return nil, status.Error(codes.Internal, "report result")
		}
	}
	return &pb.ReportResultResponse{}, nil
}

// GetSessionConfig devolve o session manifest do snapshot (ADR-0024, spec 017).
func (s *Server) GetSessionConfig(ctx context.Context, req *pb.GetSessionConfigRequest) (*pb.GetSessionConfigResponse, error) {
	if err := s.worldScope(ctx, "/session.v1.Session/GetSessionConfig", req.GetRefId()); err != nil {
		return nil, err
	}
	m, err := s.svc.GetSessionConfig(ctx, req.GetRefId())
	if err != nil {
		if errors.Is(err, domain.ErrSessionNotFound) {
			return nil, status.Error(codes.NotFound, "session not found")
		}
		return nil, status.Error(codes.Internal, "get session config")
	}
	return &pb.GetSessionConfigResponse{
		SessionId:   m.SessionID,
		MatchId:     m.MatchID,
		GameId:      m.GameID,
		ModeId:      m.ModeID,
		Archetype:   m.Archetype,
		Topology:    m.Topology,
		ParamsJson:  string(m.ParamsJSON),
		ProfileHash: m.ProfileHash,
	}, nil
}

// OpenWorld abre um mundo persistente para o host (host_world_dropin, spec 011).
// Com o jogador autenticado (spec 030), o host é o sub do token: o
// host_player_id pode vir vazio; se vier diferente, é tentativa de abrir mundo
// em nome de outro.
func (s *Server) OpenWorld(ctx context.Context, req *pb.OpenWorldRequest) (*pb.OpenWorldResponse, error) {
	host := req.GetHostPlayerId()
	if c := callerFrom(ctx); c.Kind == CallerPlayer {
		switch {
		case host == "":
			host = c.PlayerID
		case host != c.PlayerID:
			if err := s.deny(ctx, "/session.v1.Session/OpenWorld", "host_mismatch"); err != nil {
				return nil, err
			}
		}
	}
	attrs := domain.WorldAttributes{
		SoulLevel: int(req.GetHostSoulLevel()), WeaponTier: int(req.GetWeaponTier()),
		Password: req.GetPassword(), HostConnection: req.GetHostConnection(),
	}
	sess, grant, worldToken, err := s.svc.OpenWorld(ctx, host, req.GetGameId(), normalizeMode(req.GetModeId()), attrs)
	if err != nil {
		return nil, worldError(err, "open world")
	}
	return &pb.OpenWorldResponse{
		SessionId:      sess.ID,
		ServerAddress:  sess.ServerAddress,
		HostGrant:      &pb.ConnectGrant{PlayerId: grant.PlayerID, ConnectToken: grant.Token},
		HostConnection: sess.HostConnection,
		WorldToken:     worldToken,
	}, nil
}

// JoinWorld injeta um phantom no mundo com um papel e devolve o connect token.
// summoner_player_id (spec 030), quando vem, precisa ser o host do mundo —
// sempre, em qualquer modo: é o matchmaking (que já autentica o jogador) quem
// preenche o campo no SummonSign.
func (s *Server) JoinWorld(ctx context.Context, req *pb.JoinWorldRequest) (*pb.JoinWorldResponse, error) {
	if summoner := req.GetSummonerPlayerId(); summoner != "" {
		if err := s.svc.AuthorizeWorldHost(ctx, req.GetSessionId(), summoner); err != nil {
			return nil, worldError(err, "join world")
		}
	}
	grant, endpoint, err := s.svc.JoinWorld(ctx, req.GetSessionId(), req.GetPlayerId(), req.GetRole())
	if err != nil {
		return nil, worldError(err, "join world")
	}
	return &pb.JoinWorldResponse{
		Grant:          &pb.ConnectGrant{PlayerId: grant.PlayerID, ConnectToken: grant.Token},
		ServerAddress:  endpoint.ServerAddress,
		HostConnection: endpoint.HostConnection,
	}, nil
}

// ListInvadableWorlds devolve o diretório de mundos com vaga de invasor (spec
// 012). Tradução pura; sem erro de domínio novo (vazio = lista vazia). O
// mode_id NÃO é normalizado para "default": vazio significa "todos os modos"
// (compat com chamadores anteriores à spec matchmaking/012).
func (s *Server) ListInvadableWorlds(ctx context.Context, req *pb.ListInvadableWorldsRequest) (*pb.ListInvadableWorldsResponse, error) {
	worlds, err := s.svc.ListInvadableWorlds(ctx, req.GetGameId(), req.GetModeId())
	if err != nil {
		return nil, status.Error(codes.Internal, "list invadable worlds")
	}
	pbWorlds := make([]*pb.InvadableWorld, len(worlds))
	for i, w := range worlds {
		pbWorlds[i] = &pb.InvadableWorld{
			SessionId:        w.SessionID,
			HostSoulLevel:    int32(w.HostSoulLevel),
			WeaponTier:       int32(w.WeaponTier),
			FreeInvaderSlots: int32(w.FreeInvaderSlots),
			PasswordHash:     w.PasswordHash,
		}
	}
	return &pb.ListInvadableWorldsResponse{Worlds: pbWorlds}, nil
}

// LeaveWorld libera o slot do jogador (no-op idempotente; host encerra o mundo).
// Jogador autenticado sai a si mesmo (player_id vazio = ele); tirar outro
// jogador exige ser o host (spec 030). O Dedicated Server pode tirar qualquer um.
func (s *Server) LeaveWorld(ctx context.Context, req *pb.LeaveWorldRequest) (*pb.LeaveWorldResponse, error) {
	player := req.GetPlayerId()
	if c := callerFrom(ctx); c.Kind == CallerPlayer {
		if player == "" {
			player = c.PlayerID
		}
		if player != c.PlayerID {
			if err := s.hostCheck(ctx, "/session.v1.Session/LeaveWorld", req.GetSessionId(), c.PlayerID); err != nil {
				return nil, err
			}
		}
	}
	if err := s.worldScope(ctx, "/session.v1.Session/LeaveWorld", req.GetSessionId()); err != nil {
		return nil, err
	}
	if err := s.svc.LeaveWorld(ctx, req.GetSessionId(), player); err != nil {
		return nil, worldError(err, "leave world")
	}
	return &pb.LeaveWorldResponse{}, nil
}

// CloseWorld encerra o mundo (host saiu) e libera o server.
// Jogador só fecha o próprio mundo (spec 030); o Dedicated Server fecha qualquer um.
func (s *Server) CloseWorld(ctx context.Context, req *pb.CloseWorldRequest) (*pb.CloseWorldResponse, error) {
	if c := callerFrom(ctx); c.Kind == CallerPlayer {
		if err := s.hostCheck(ctx, "/session.v1.Session/CloseWorld", req.GetSessionId(), c.PlayerID); err != nil {
			return nil, err
		}
	}
	if err := s.worldScope(ctx, "/session.v1.Session/CloseWorld", req.GetSessionId()); err != nil {
		return nil, err
	}
	if err := s.svc.CloseWorld(ctx, req.GetSessionId()); err != nil {
		return nil, worldError(err, "close world")
	}
	return &pb.CloseWorldResponse{}, nil
}

// worldError mapeia os erros de domínio do host_world_dropin para gRPC codes.
func worldError(err error, op string) error {
	switch {
	case errors.Is(err, domain.ErrUnsupportedArchetype):
		return status.Error(codes.FailedPrecondition, "game is not host_world_dropin")
	case errors.Is(err, domain.ErrRoleNotEnabled):
		return status.Error(codes.InvalidArgument, "role not enabled")
	case errors.Is(err, domain.ErrRoleFull), errors.Is(err, domain.ErrNoServer):
		return status.Error(codes.ResourceExhausted, "no capacity")
	case errors.Is(err, domain.ErrWorldNotFound):
		return status.Error(codes.NotFound, "world not found")
	case errors.Is(err, domain.ErrNotWorldHost):
		return status.Error(codes.PermissionDenied, "not the world host")
	case errors.Is(err, domain.ErrAlreadyJoined):
		return status.Error(codes.FailedPrecondition, "player already joined")
	case errors.Is(err, domain.ErrInvalidParticipation):
		return status.Error(codes.InvalidArgument, "invalid participation event")
	case errors.Is(err, domain.ErrFleetUnavailable):
		return status.Error(codes.FailedPrecondition, "fleet not configured")
	case errors.Is(err, domain.ErrHostConnectionRequired):
		return status.Error(codes.InvalidArgument, "host_connection required for p2p world")
	default:
		return status.Error(codes.Internal, op)
	}
}

// ReportParticipationEvent recebe um evento por participante do server do mundo
// (spec 013) e o traduz para o domínio.
func (s *Server) ReportParticipationEvent(ctx context.Context, req *pb.ReportParticipationEventRequest) (*pb.ReportParticipationEventResponse, error) {
	if err := s.worldScope(ctx, "/session.v1.Session/ReportParticipationEvent", req.GetSessionId()); err != nil {
		return nil, err
	}
	evt := domain.ParticipationEvent{
		PlayerID: req.GetPlayerId(),
		Role:     req.GetRole(),
		Event:    req.GetEvent(),
		Outcome:  req.GetOutcome(),
		EventID:  req.GetEventId(),
		Stats:    req.GetStats(),
	}
	if err := s.svc.ReportParticipationEvent(ctx, req.GetSessionId(), evt); err != nil {
		return nil, worldError(err, "report participation event")
	}
	return &pb.ReportParticipationEventResponse{}, nil
}

// toMatchOutcome mapeia o oneof do request para o domínio. Kind vazio (nenhum
// campo do oneof setado) cai como outcome não suportado na validação do service.
func toMatchOutcome(req *pb.ReportResultRequest) domain.MatchOutcome {
	if wld := req.GetWinLossDraw(); wld != nil {
		players := make([]domain.PlayerOutcome, len(wld.GetPlayers()))
		for i, p := range wld.GetPlayers() {
			players[i] = domain.PlayerOutcome{
				PlayerID: p.GetPlayerId(),
				TeamID:   p.GetTeamId(),
				Outcome:  p.GetOutcome(),
				Score:    p.GetScore(),
				Stats:    p.GetStats(),
			}
		}
		return domain.MatchOutcome{Kind: domain.OutcomeWinLossDraw, WinLossDraw: &domain.WinLossDrawResult{Players: players}}
	}
	if pl := req.GetPlacement(); pl != nil {
		entries := make([]domain.PlacementEntry, len(pl.GetEntries()))
		for i, e := range pl.GetEntries() {
			entries[i] = domain.PlacementEntry{
				PlayerID:   e.GetPlayerId(),
				TeamID:     e.GetTeamId(),
				Placement:  int(e.GetPlacement()), // int32 (proto) -> int (domínio)
				SurvivedMs: e.GetSurvivedMs(),
				Stats:      e.GetStats(),
			}
		}
		return domain.MatchOutcome{Kind: domain.OutcomePlacement, Placement: &domain.PlacementResult{Entries: entries}}
	}
	if c := req.GetCollective(); c != nil {
		players := make([]domain.CollectivePlayer, len(c.GetPlayers()))
		for i, p := range c.GetPlayers() {
			players[i] = domain.CollectivePlayer{
				PlayerID: p.GetPlayerId(),
				TeamID:   p.GetTeamId(),
				Stats:    p.GetStats(),
			}
		}
		return domain.MatchOutcome{Kind: domain.OutcomeCollective, Collective: &domain.CollectiveResult{Outcome: c.GetOutcome(), Players: players}}
	}
	return domain.MatchOutcome{} // Kind vazio -> ErrUnsupportedOutcome
}

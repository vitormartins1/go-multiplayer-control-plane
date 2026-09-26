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
	svc domain.SessionService
}

// NewServer cria o server a partir do SessionService (Port de entrada).
func NewServer(svc domain.SessionService) *Server {
	return &Server{svc: svc}
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
func (s *Server) OpenWorld(ctx context.Context, req *pb.OpenWorldRequest) (*pb.OpenWorldResponse, error) {
	attrs := domain.WorldAttributes{SoulLevel: int(req.GetHostSoulLevel()), WeaponTier: int(req.GetWeaponTier()), Password: req.GetPassword()}
	sess, grant, err := s.svc.OpenWorld(ctx, req.GetHostPlayerId(), req.GetGameId(), normalizeMode(req.GetModeId()), attrs)
	if err != nil {
		return nil, worldError(err, "open world")
	}
	return &pb.OpenWorldResponse{
		SessionId:     sess.ID,
		ServerAddress: sess.ServerAddress,
		HostGrant:     &pb.ConnectGrant{PlayerId: grant.PlayerID, ConnectToken: grant.Token},
	}, nil
}

// JoinWorld injeta um phantom no mundo com um papel e devolve o connect token.
func (s *Server) JoinWorld(ctx context.Context, req *pb.JoinWorldRequest) (*pb.JoinWorldResponse, error) {
	grant, serverAddr, err := s.svc.JoinWorld(ctx, req.GetSessionId(), req.GetPlayerId(), req.GetRole())
	if err != nil {
		return nil, worldError(err, "join world")
	}
	return &pb.JoinWorldResponse{
		Grant:         &pb.ConnectGrant{PlayerId: grant.PlayerID, ConnectToken: grant.Token},
		ServerAddress: serverAddr,
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
func (s *Server) LeaveWorld(ctx context.Context, req *pb.LeaveWorldRequest) (*pb.LeaveWorldResponse, error) {
	if err := s.svc.LeaveWorld(ctx, req.GetSessionId(), req.GetPlayerId()); err != nil {
		return nil, worldError(err, "leave world")
	}
	return &pb.LeaveWorldResponse{}, nil
}

// CloseWorld encerra o mundo (host saiu) e libera o server.
func (s *Server) CloseWorld(ctx context.Context, req *pb.CloseWorldRequest) (*pb.CloseWorldResponse, error) {
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
	case errors.Is(err, domain.ErrAlreadyJoined):
		return status.Error(codes.FailedPrecondition, "player already joined")
	case errors.Is(err, domain.ErrInvalidParticipation):
		return status.Error(codes.InvalidArgument, "invalid participation event")
	case errors.Is(err, domain.ErrFleetUnavailable):
		return status.Error(codes.FailedPrecondition, "fleet not configured")
	default:
		return status.Error(codes.Internal, op)
	}
}

// ReportParticipationEvent recebe um evento por participante do server do mundo
// (spec 013) e o traduz para o domínio.
func (s *Server) ReportParticipationEvent(ctx context.Context, req *pb.ReportParticipationEventRequest) (*pb.ReportParticipationEventResponse, error) {
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

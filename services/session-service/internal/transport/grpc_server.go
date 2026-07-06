// Package transport traduz gRPC para o domínio (unary) e mapeia os erros de
// domínio para gRPC status codes. Sem lógica de negócio. Ver ADR-0002 e ADR-0003.
package transport

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/vitormartins1/go-game-backend/services/session-service/gen/session/v1"
	"github.com/vitormartins1/go-game-backend/services/session-service/internal/domain"
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

// CreateSession aloca um server, emite os connect tokens e registra a sessão.
func (s *Server) CreateSession(ctx context.Context, req *pb.CreateSessionRequest) (*pb.CreateSessionResponse, error) {
	sess, grants, err := s.svc.Create(ctx, req.GetMatchId(), req.GetGameId(), req.GetPlayers())
	if err != nil {
		if errors.Is(err, domain.ErrNoServer) {
			return nil, status.Error(codes.ResourceExhausted, "no server available")
		}
		return nil, status.Error(codes.Internal, "create session")
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
		default:
			return nil, status.Error(codes.Internal, "report result")
		}
	}
	return &pb.ReportResultResponse{}, nil
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
	return domain.MatchOutcome{} // Kind vazio -> ErrUnsupportedOutcome
}

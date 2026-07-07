// Package transport traduz gRPC para o domínio (server-streaming) e valida o
// JWT via interceptor. Sem lógica de negócio. Ver ADR-0002 e ADR-0003.
package transport

import (
	"context"
	"errors"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "github.com/vitormartins1/go-game-backend/services/matchmaking-service/gen/matchmaking/v1"
	"github.com/vitormartins1/go-game-backend/services/matchmaking-service/internal/domain"
)

// Server implementa o serviço gRPC Matchmaking.
type Server struct {
	pb.UnimplementedMatchmakingServer
	mm  domain.Matchmaker
	inv domain.Invader
}

// NewServer cria o server a partir do matchmaker (queue-form) e do invader
// (host_world_dropin, spec 006), as duas Ports de entrada.
func NewServer(mm domain.Matchmaker, inv domain.Invader) *Server {
	return &Server{mm: mm, inv: inv}
}

// FindMatch enfileira o jogador no jogo e transmite o status do ticket até o
// match. O game_id vem do request (contexto do cliente, ADR-0017).
func (s *Server) FindMatch(req *pb.FindMatchRequest, stream pb.Matchmaking_FindMatchServer) error {
	gameID := req.GetGameId()
	if gameID == "" {
		return status.Error(codes.InvalidArgument, "game_id é obrigatório")
	}
	playerID := playerIDFromContext(stream.Context())
	ch, err := s.mm.Join(stream.Context(), gameID, playerID)
	if err != nil {
		if errors.Is(err, domain.ErrProfileUnavailable) {
			// jogo sem perfil de multiplayer utilizável (ADR-0023): não dá para
			// derivar o tamanho do match, então o ticket não entra na fila.
			return status.Error(codes.FailedPrecondition, "jogo sem perfil de multiplayer configurado")
		}
		return status.Error(codes.Internal, "join")
	}
	if err := stream.Send(&pb.TicketUpdate{Status: pb.Status_STATUS_SEARCHING}); err != nil {
		return err
	}
	select {
	case res := <-ch:
		if res.Failed {
			return stream.Send(&pb.TicketUpdate{
				Status:  pb.Status_STATUS_FAILED,
				MatchId: res.MatchID,
				Players: res.Players,
				Reason:  res.Reason,
			})
		}
		return stream.Send(&pb.TicketUpdate{
			Status:        pb.Status_STATUS_MATCHED,
			MatchId:       res.MatchID,
			Players:       res.Players,
			ServerAddress: res.ServerAddress,
			ConnectToken:  res.ConnectToken,
		})
	case <-stream.Context().Done():
		return stream.Context().Err() // cliente cancelou -> ticket removido pelo matchmaker
	}
}

// Invade descobre um mundo elegível e injeta o invasor (host_world_dropin, spec
// 006). O playerID vem do JWT (UnaryAuthInterceptor), não do request. Mapeia os
// erros de domínio para gRPC codes.
func (s *Server) Invade(ctx context.Context, req *pb.InvadeRequest) (*pb.InvadeResponse, error) {
	gameID := req.GetGameId()
	if gameID == "" {
		return nil, status.Error(codes.InvalidArgument, "game_id é obrigatório")
	}
	playerID := playerIDFromContext(ctx)
	crit := domain.InvaderCriteria{SoulLevel: int(req.GetSoulLevel()), WeaponTier: int(req.GetWeaponTier())}

	invaded, err := s.inv.Invade(ctx, gameID, playerID, crit)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvasionNotSupported):
			return nil, status.Error(codes.FailedPrecondition, "jogo não aceita invasão")
		case errors.Is(err, domain.ErrNoEligibleWorld):
			return nil, status.Error(codes.NotFound, "nenhum mundo elegível")
		case errors.Is(err, domain.ErrProfileUnavailable):
			return nil, status.Error(codes.FailedPrecondition, "jogo sem perfil de multiplayer configurado")
		default:
			return nil, status.Error(codes.Internal, "invade")
		}
	}
	return &pb.InvadeResponse{
		SessionId:     invaded.SessionID,
		ServerAddress: invaded.ServerAddress,
		ConnectToken:  invaded.ConnectToken,
	}, nil
}

// --- auth interceptor ---

type ctxKey int

const playerIDKey ctxKey = iota

func playerIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(playerIDKey).(string)
	return id
}

// StreamAuthInterceptor valida o Bearer JWT do metadata e injeta o playerID no
// context do stream. Falha vira codes.Unauthenticated.
func StreamAuthInterceptor(verifier domain.TokenVerifier) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		token := bearerFromMetadata(ss.Context())
		if token == "" {
			return status.Error(codes.Unauthenticated, "token ausente")
		}
		playerID, err := verifier.Verify(ss.Context(), token)
		if err != nil {
			return status.Error(codes.Unauthenticated, "token inválido")
		}
		ctx := context.WithValue(ss.Context(), playerIDKey, playerID)
		return handler(srv, &wrappedStream{ServerStream: ss, ctx: ctx})
	}
}

// UnaryAuthInterceptor valida o Bearer JWT e injeta o playerID no context da
// chamada unary (espelho do StreamAuthInterceptor — o Invade é unary, spec 006).
// Falha vira codes.Unauthenticated.
func UnaryAuthInterceptor(verifier domain.TokenVerifier) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		token := bearerFromMetadata(ctx)
		if token == "" {
			return nil, status.Error(codes.Unauthenticated, "token ausente")
		}
		playerID, err := verifier.Verify(ctx, token)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "token inválido")
		}
		return handler(context.WithValue(ctx, playerIDKey, playerID), req)
	}
}

// wrappedStream sobrescreve o Context() para carregar o playerID (o ServerStream
// não permite trocar o context diretamente).
type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context { return w.ctx }

func bearerFromMetadata(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	vals := md.Get("authorization")
	if len(vals) == 0 {
		return ""
	}
	const prefix = "Bearer "
	if h := vals[0]; len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return h[len(prefix):]
	}
	return ""
}

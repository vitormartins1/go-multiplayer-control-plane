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
	sum domain.Summoner
}

// NewServer cria o server a partir das três Ports de entrada: matchmaker
// (queue-form), invader (invasão, spec 006) e summoner (co-op, spec 007).
func NewServer(mm domain.Matchmaker, inv domain.Invader, sum domain.Summoner) *Server {
	return &Server{mm: mm, inv: inv, sum: sum}
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

// PostSummonSign é o aliado anunciando um sinal de co-op e aguardando ser
// invocado (host_world_dropin, spec 007). Server-streaming: envia POSTED e depois
// bloqueia até SUMMONED (invocado) ou EXPIRED (TTL/cancelado). O playerID vem do
// JWT (StreamAuthInterceptor). Fechar o stream cancela o sinal.
func (s *Server) PostSummonSign(req *pb.PostSummonSignRequest, stream pb.Matchmaking_PostSummonSignServer) error {
	gameID := req.GetGameId()
	if gameID == "" {
		return status.Error(codes.InvalidArgument, "game_id é obrigatório")
	}
	playerID := playerIDFromContext(stream.Context())
	crit := domain.CoopCriteria{SoulLevel: int(req.GetSoulLevel()), WeaponTier: int(req.GetWeaponTier())}

	signID, ch, err := s.sum.PostSign(stream.Context(), gameID, playerID, crit)
	if err != nil {
		return coopError(err, "post summon sign")
	}
	if err := stream.Send(&pb.SummonUpdate{Status: pb.SummonStatus_SUMMON_STATUS_POSTED, SignId: signID}); err != nil {
		s.sum.CancelSign(signID)
		return err
	}

	select {
	case <-stream.Context().Done():
		s.sum.CancelSign(signID) // aliado fechou o stream -> tira o sinal do quadro
		return stream.Context().Err()
	case summoned, ok := <-ch:
		if !ok {
			// canal fechado sem valor = TTL venceu (o sweeper fechou).
			return stream.Send(&pb.SummonUpdate{Status: pb.SummonStatus_SUMMON_STATUS_EXPIRED})
		}
		return stream.Send(&pb.SummonUpdate{
			Status:        pb.SummonStatus_SUMMON_STATUS_SUMMONED,
			SessionId:     summoned.SessionID,
			ServerAddress: summoned.ServerAddress,
			ConnectToken:  summoned.ConnectToken,
		})
	}
}

// ListSigns devolve os sinais compatíveis com o mundo do host (spec 007).
func (s *Server) ListSigns(ctx context.Context, req *pb.ListSignsRequest) (*pb.ListSignsResponse, error) {
	gameID := req.GetGameId()
	if gameID == "" {
		return nil, status.Error(codes.InvalidArgument, "game_id é obrigatório")
	}
	host := domain.CoopCriteria{SoulLevel: int(req.GetHostSoulLevel()), WeaponTier: int(req.GetHostWeaponTier())}
	signs, err := s.sum.ListSigns(ctx, gameID, host)
	if err != nil {
		return nil, coopError(err, "list signs")
	}
	pbSigns := make([]*pb.SummonSign, len(signs))
	for i, sg := range signs {
		pbSigns[i] = &pb.SummonSign{
			SignId:     sg.SignID,
			PlayerId:   sg.PlayerID,
			SoulLevel:  int32(sg.SoulLevel),
			WeaponTier: int32(sg.WeaponTier),
		}
	}
	return &pb.ListSignsResponse{Signs: pbSigns}, nil
}

// SummonSign injeta o aliado do sinal no mundo do host como coop (spec 007). O
// server/token vão ao aliado pelo stream do PostSummonSign; aqui devolvemos só o
// playerID invocado.
func (s *Server) SummonSign(ctx context.Context, req *pb.SummonSignRequest) (*pb.SummonSignResponse, error) {
	if req.GetSessionId() == "" || req.GetSignId() == "" {
		return nil, status.Error(codes.InvalidArgument, "session_id e sign_id são obrigatórios")
	}
	allyID, err := s.sum.Summon(ctx, req.GetSessionId(), req.GetSignId())
	if err != nil {
		return nil, coopError(err, "summon sign")
	}
	return &pb.SummonSignResponse{PlayerId: allyID}, nil
}

// coopError mapeia os erros de domínio do co-op para gRPC codes (spec 007).
func coopError(err error, op string) error {
	switch {
	case errors.Is(err, domain.ErrCoopNotSupported), errors.Is(err, domain.ErrProfileUnavailable):
		return status.Error(codes.FailedPrecondition, "jogo não aceita co-op")
	case errors.Is(err, domain.ErrSignConsumed):
		return status.Error(codes.FailedPrecondition, "sinal já consumido")
	case errors.Is(err, domain.ErrSignNotFound), errors.Is(err, domain.ErrWorldNotFound):
		return status.Error(codes.NotFound, "sinal ou mundo não encontrado")
	case errors.Is(err, domain.ErrRoleFull):
		return status.Error(codes.ResourceExhausted, "sem vaga de co-op")
	default:
		return status.Error(codes.Internal, op)
	}
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

package transport

import (
	"context"
	"net"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/vitormartins1/go-game-backend/services/session-service/gen/session/v1"
	"github.com/vitormartins1/go-game-backend/services/session-service/internal/allocator"
	"github.com/vitormartins1/go-game-backend/services/session-service/internal/domain"
	"github.com/vitormartins1/go-game-backend/services/session-service/internal/publisher"
	"github.com/vitormartins1/go-game-backend/services/session-service/internal/service"
	"github.com/vitormartins1/go-game-backend/services/session-service/internal/token"
)

// stubProfile é um ProfileProvider fixo para os testes de host_world: só o jogo
// "hw" tem perfil host_world_dropin; qualquer outro é indisponível.
type stubProfile struct{ profile domain.WorldProfile }

func (s stubProfile) WorldProfile(_ context.Context, gameID string) (domain.WorldProfile, error) {
	if gameID == "hw" {
		return s.profile, nil
	}
	return domain.WorldProfile{}, domain.ErrProfileUnavailable
}

// setupWorld sobe o server gRPC com um ProfileProvider host_world_dropin.
func setupWorld(t *testing.T, addresses []string) (pb.SessionClient, *token.ConnectIssuer) {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	iss, err := token.Generate()
	require.NoError(t, err)
	prof := stubProfile{profile: domain.WorldProfile{
		Archetype: "host_world_dropin", EnabledRoles: []string{domain.RoleCoop, domain.RoleInvader},
		CoopSlots: 2, InvaderSlots: 1,
	}}
	svc := service.New(allocator.New(addresses), iss, publisher.New(slogDiscard()), service.WithProfileProvider(prof))

	srv := grpc.NewServer()
	pb.RegisterSessionServer(srv, NewServer(svc))
	go func() { _ = srv.Serve(lis) }()

	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close(); srv.Stop() })
	return pb.NewSessionClient(conn), iss
}

// claimsOf decodifica os claims de um connect token com a chave do issuer.
func claimsOf(t *testing.T, iss *token.ConnectIssuer, tok string) jwt.MapClaims {
	t.Helper()
	parsed, err := jwt.Parse(tok, func(*jwt.Token) (any, error) { return iss.PublicKey(), nil },
		jwt.WithValidMethods([]string{"EdDSA"}))
	require.NoError(t, err)
	return parsed.Claims.(jwt.MapClaims)
}

// Fluxo completo: abrir mundo → co-op entra → invasor entra → invasor além da
// capacidade (ResourceExhausted) → co-op sai → host encerra.
func TestHostWorld_FluxoCompleto(t *testing.T) {
	client, iss := setupWorld(t, []string{"10.0.0.1:7777", "10.0.0.2:7777"})
	ctx := context.Background()

	open, err := client.OpenWorld(ctx, &pb.OpenWorldRequest{HostPlayerId: "host-1", GameId: "hw"})
	require.NoError(t, err)
	require.NotEmpty(t, open.GetSessionId())
	assert.Equal(t, "host", claimsOf(t, iss, open.GetHostGrant().GetConnectToken())["role"])
	sid := open.GetSessionId()

	coop, err := client.JoinWorld(ctx, &pb.JoinWorldRequest{SessionId: sid, PlayerId: "coop-1", Role: "coop"})
	require.NoError(t, err)
	assert.Equal(t, "coop", claimsOf(t, iss, coop.GetGrant().GetConnectToken())["role"])

	_, err = client.JoinWorld(ctx, &pb.JoinWorldRequest{SessionId: sid, PlayerId: "inv-1", Role: "invader"})
	require.NoError(t, err)

	// slot de invasor (capacidade 1) esgotado → ResourceExhausted.
	_, err = client.JoinWorld(ctx, &pb.JoinWorldRequest{SessionId: sid, PlayerId: "inv-2", Role: "invader"})
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))

	_, err = client.LeaveWorld(ctx, &pb.LeaveWorldRequest{SessionId: sid, PlayerId: "coop-1"})
	require.NoError(t, err)

	_, err = client.CloseWorld(ctx, &pb.CloseWorldRequest{SessionId: sid})
	require.NoError(t, err)
}

func TestOpenWorld_JogoNaoHostWorld_FailedPrecondition(t *testing.T) {
	client, _ := setupWorld(t, []string{"10.0.0.1:7777"})
	_, err := client.OpenWorld(context.Background(), &pb.OpenWorldRequest{HostPlayerId: "h", GameId: "queue-arena-game"})
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestJoinWorld_PapelNaoHabilitado_InvalidArgument(t *testing.T) {
	client, _ := setupWorld(t, []string{"10.0.0.1:7777"})
	open, err := client.OpenWorld(context.Background(), &pb.OpenWorldRequest{HostPlayerId: "h", GameId: "hw"})
	require.NoError(t, err)

	_, err = client.JoinWorld(context.Background(), &pb.JoinWorldRequest{SessionId: open.GetSessionId(), PlayerId: "p", Role: "spectator"})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestJoinWorld_MundoInexistente_NotFound(t *testing.T) {
	client, _ := setupWorld(t, []string{"10.0.0.1:7777"})
	_, err := client.JoinWorld(context.Background(), &pb.JoinWorldRequest{SessionId: "nope", PlayerId: "p", Role: "coop"})
	assert.Equal(t, codes.NotFound, status.Code(err))
}

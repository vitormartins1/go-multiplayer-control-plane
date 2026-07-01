package transport

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/vitormartins1/go-game-backend/services/matchmaking-service/gen/matchmaking/v1"
	"github.com/vitormartins1/go-game-backend/services/matchmaking-service/internal/domain"
	"github.com/vitormartins1/go-game-backend/services/matchmaking-service/internal/service"
)

// fakeVerifier devolve o próprio token como playerID (cada cliente usa um token
// distinto para representar jogadores distintos).
type fakeVerifier struct{}

func (fakeVerifier) Verify(_ context.Context, token string) (string, error) {
	if token == "" {
		return "", errors.New("sem token")
	}
	return token, nil
}

// provisionerFunc adapta uma func à Port domain.SessionProvisioner.
type provisionerFunc func(matchID string, players []string) (domain.Provisioned, error)

func (f provisionerFunc) Provision(_ context.Context, matchID string, players []string, _ string) (domain.Provisioned, error) {
	return f(matchID, players)
}

// okProvisioner devolve server fixo e um token por jogador (tok-<player>).
var okProvisioner = provisionerFunc(func(_ string, players []string) (domain.Provisioned, error) {
	tokens := make(map[string]string, len(players))
	for _, p := range players {
		tokens[p] = "tok-" + p
	}
	return domain.Provisioned{ServerAddress: "10.0.0.1:7777", Tokens: tokens}, nil
})

func setup(t *testing.T, prov domain.SessionProvisioner) pb.MatchmakingClient {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	mm := service.New(2, 10*time.Millisecond, prov)
	ctx, cancel := context.WithCancel(context.Background())
	go mm.Run(ctx)

	srv := grpc.NewServer(grpc.StreamInterceptor(StreamAuthInterceptor(fakeVerifier{})))
	pb.RegisterMatchmakingServer(srv, NewServer(mm))
	go func() { _ = srv.Serve(lis) }()

	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = conn.Close()
		srv.Stop()
		cancel()
	})
	return pb.NewMatchmakingClient(conn)
}

func authCtx(token string) context.Context {
	return metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token)
}

func TestFindMatch_SemToken_Unauthenticated(t *testing.T) {
	client := setup(t, okProvisioner)
	stream, err := client.FindMatch(context.Background(), &pb.FindMatchRequest{})
	require.NoError(t, err)

	_, err = stream.Recv()
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestFindMatch_DoisClientes_MatchedComServerEToken(t *testing.T) {
	client := setup(t, okProvisioner)

	s1, err := client.FindMatch(authCtx("p1"), &pb.FindMatchRequest{GameId: "game-1"})
	require.NoError(t, err)
	s2, err := client.FindMatch(authCtx("p2"), &pb.FindMatchRequest{GameId: "game-1"})
	require.NoError(t, err)

	// primeiro update: SEARCHING
	u1, err := s1.Recv()
	require.NoError(t, err)
	assert.Equal(t, pb.Status_STATUS_SEARCHING, u1.Status)
	u2, err := s2.Recv()
	require.NoError(t, err)
	assert.Equal(t, pb.Status_STATUS_SEARCHING, u2.Status)

	// segundo update: MATCHED com mesmo matchId, server e token do PRÓPRIO jogador
	m1, err := s1.Recv()
	require.NoError(t, err)
	assert.Equal(t, pb.Status_STATUS_MATCHED, m1.Status)
	assert.NotEmpty(t, m1.MatchId)
	assert.Equal(t, "10.0.0.1:7777", m1.ServerAddress)
	assert.Equal(t, "tok-p1", m1.ConnectToken)
	m2, err := s2.Recv()
	require.NoError(t, err)
	assert.Equal(t, m1.MatchId, m2.MatchId)
	assert.Equal(t, "10.0.0.1:7777", m2.ServerAddress)
	assert.Equal(t, "tok-p2", m2.ConnectToken)
	assert.ElementsMatch(t, []string{"p1", "p2"}, m1.Players)
}

func TestFindMatch_SemServer_Failed(t *testing.T) {
	noServer := provisionerFunc(func(string, []string) (domain.Provisioned, error) {
		return domain.Provisioned{}, domain.ErrNoServer
	})
	client := setup(t, noServer)

	s1, err := client.FindMatch(authCtx("p1"), &pb.FindMatchRequest{GameId: "game-1"})
	require.NoError(t, err)
	s2, err := client.FindMatch(authCtx("p2"), &pb.FindMatchRequest{GameId: "game-1"})
	require.NoError(t, err)

	require.NoError(t, recvSearching(t, s1))
	require.NoError(t, recvSearching(t, s2))

	f1, err := s1.Recv()
	require.NoError(t, err)
	assert.Equal(t, pb.Status_STATUS_FAILED, f1.Status)
	assert.Equal(t, "no game server available", f1.Reason)
	assert.Empty(t, f1.ConnectToken)
}

func recvSearching(t *testing.T, s pb.Matchmaking_FindMatchClient) error {
	t.Helper()
	u, err := s.Recv()
	if err != nil {
		return err
	}
	assert.Equal(t, pb.Status_STATUS_SEARCHING, u.Status)
	return nil
}

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

	pb "github.com/c130n/game-backend/services/matchmaking-service/gen/matchmaking/v1"
	"github.com/c130n/game-backend/services/matchmaking-service/internal/service"
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

func setup(t *testing.T) pb.MatchmakingClient {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	mm := service.New(2, 10*time.Millisecond)
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
	client := setup(t)
	stream, err := client.FindMatch(context.Background(), &pb.FindMatchRequest{})
	require.NoError(t, err)

	_, err = stream.Recv()
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestFindMatch_DoisClientes_MesmoMatch(t *testing.T) {
	client := setup(t)

	s1, err := client.FindMatch(authCtx("p1"), &pb.FindMatchRequest{})
	require.NoError(t, err)
	s2, err := client.FindMatch(authCtx("p2"), &pb.FindMatchRequest{})
	require.NoError(t, err)

	// primeiro update: SEARCHING
	u1, err := s1.Recv()
	require.NoError(t, err)
	assert.Equal(t, pb.Status_STATUS_SEARCHING, u1.Status)
	u2, err := s2.Recv()
	require.NoError(t, err)
	assert.Equal(t, pb.Status_STATUS_SEARCHING, u2.Status)

	// segundo update: MATCHED com o mesmo matchId
	m1, err := s1.Recv()
	require.NoError(t, err)
	assert.Equal(t, pb.Status_STATUS_MATCHED, m1.Status)
	assert.NotEmpty(t, m1.MatchId)
	m2, err := s2.Recv()
	require.NoError(t, err)
	assert.Equal(t, m1.MatchId, m2.MatchId)
	assert.ElementsMatch(t, []string{"p1", "p2"}, m1.Players)
}

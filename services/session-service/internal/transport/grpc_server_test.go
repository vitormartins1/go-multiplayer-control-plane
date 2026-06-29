package transport

import (
	"context"
	"io"
	"log/slog"
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

	pb "github.com/c130n/game-backend/services/session-service/gen/session/v1"
	"github.com/c130n/game-backend/services/session-service/internal/allocator"
	"github.com/c130n/game-backend/services/session-service/internal/publisher"
	"github.com/c130n/game-backend/services/session-service/internal/service"
	"github.com/c130n/game-backend/services/session-service/internal/token"
)

func setup(t *testing.T, addresses []string) (pb.SessionClient, *token.ConnectIssuer) {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	iss, err := token.Generate()
	require.NoError(t, err)
	svc := service.New(allocator.New(addresses), iss, publisher.New(slogDiscard()))

	srv := grpc.NewServer()
	pb.RegisterSessionServer(srv, NewServer(svc))
	go func() { _ = srv.Serve(lis) }()

	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = conn.Close()
		srv.Stop()
	})
	return pb.NewSessionClient(conn), iss
}

func TestCreateSession_RetornaGrantsEServer_TokensVerificaveis(t *testing.T) {
	client, iss := setup(t, []string{"10.0.0.1:7777"})

	resp, err := client.CreateSession(context.Background(), &pb.CreateSessionRequest{
		MatchId: "match-1",
		Players: []string{"p1", "p2"},
	})
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.1:7777", resp.GetServerAddress())
	assert.NotEmpty(t, resp.GetSessionId())
	require.Len(t, resp.GetGrants(), 2)

	// cada connect token é verificável com a chave pública e tem os claims certos.
	for _, g := range resp.GetGrants() {
		parsed, err := jwt.Parse(g.GetConnectToken(), func(*jwt.Token) (any, error) {
			return iss.PublicKey(), nil
		}, jwt.WithValidMethods([]string{"EdDSA"}))
		require.NoError(t, err)
		claims := parsed.Claims.(jwt.MapClaims)
		assert.Equal(t, g.GetPlayerId(), claims["sub"])
		assert.Equal(t, "match-1", claims["match_id"])
		assert.Equal(t, "10.0.0.1:7777", claims["server"])
	}
}

func TestCreateSession_PoolEsgotado_ResourceExhausted(t *testing.T) {
	client, _ := setup(t, []string{"10.0.0.1:7777"})

	_, err := client.CreateSession(context.Background(), &pb.CreateSessionRequest{MatchId: "m1", Players: []string{"p1"}})
	require.NoError(t, err)

	_, err = client.CreateSession(context.Background(), &pb.CreateSessionRequest{MatchId: "m2", Players: []string{"p2"}})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.ResourceExhausted, st.Code())
}

func TestReportResult_LiberaServer_PermiteNovaAlocacao(t *testing.T) {
	client, _ := setup(t, []string{"10.0.0.1:7777"})

	_, err := client.CreateSession(context.Background(), &pb.CreateSessionRequest{MatchId: "m1", Players: []string{"p1"}})
	require.NoError(t, err)

	_, err = client.ReportResult(context.Background(), &pb.ReportResultRequest{
		MatchId: "m1",
		Results: []*pb.PlayerResult{{PlayerId: "p1", Score: 10, Won: true}},
	})
	require.NoError(t, err)

	// server liberado -> nova sessão consegue alocar de novo.
	_, err = client.CreateSession(context.Background(), &pb.CreateSessionRequest{MatchId: "m2", Players: []string{"p2"}})
	assert.NoError(t, err)
}

func TestReportResult_MatchInexistente_NotFound(t *testing.T) {
	client, _ := setup(t, []string{"10.0.0.1:7777"})

	_, err := client.ReportResult(context.Background(), &pb.ReportResultRequest{MatchId: "nao-existe"})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
}

func slogDiscard() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

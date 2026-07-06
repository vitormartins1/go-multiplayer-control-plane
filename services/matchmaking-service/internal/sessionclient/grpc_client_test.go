package sessionclient

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/vitormartins1/go-game-backend/services/matchmaking-service/internal/domain"
	sessionv1 "github.com/vitormartins1/go-game-backend/services/session-service/gen/session/v1"
)

// fakeSessionServer implementa o contrato gRPC gerado do session. Não dá para
// reusar o session real (Go proíbe importar o internal/ de outro serviço), então
// validamos o adapter contra o contrato: grants -> tokens e ResourceExhausted ->
// ErrNoServer.
type fakeSessionServer struct {
	sessionv1.UnimplementedSessionServer
	full       bool // true: pool esgotado -> ResourceExhausted
	gotGameID  string
	gotPlayers []string
	gotTeams   []*sessionv1.Team
}

func (f *fakeSessionServer) CreateSession(_ context.Context, req *sessionv1.CreateSessionRequest) (*sessionv1.CreateSessionResponse, error) {
	f.gotGameID = req.GetGameId()
	f.gotPlayers = req.GetPlayers()
	f.gotTeams = req.GetTeams()
	if f.full {
		return nil, status.Error(codes.ResourceExhausted, "no server available")
	}
	grants := make([]*sessionv1.ConnectGrant, 0, len(req.GetPlayers()))
	for _, p := range req.GetPlayers() {
		grants = append(grants, &sessionv1.ConnectGrant{PlayerId: p, ConnectToken: "tok-" + p})
	}
	return &sessionv1.CreateSessionResponse{SessionId: "s1", ServerAddress: "10.0.0.1:7777", Grants: grants}, nil
}

func setup(t *testing.T, srvImpl sessionv1.SessionServer) *Client {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	sessionv1.RegisterSessionServer(srv, srvImpl)
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
	return New(sessionv1.NewSessionClient(conn))
}

func TestProvision_Sucesso_TokenPorJogador(t *testing.T) {
	fake := &fakeSessionServer{}
	c := setup(t, fake)

	got, err := c.Provision(context.Background(), "match-1", ffa("p1", "p2"), "game-1")
	require.NoError(t, err)

	assert.Equal(t, "game-1", fake.gotGameID, "game_id repassado ao CreateSession")
	assert.Equal(t, "10.0.0.1:7777", got.ServerAddress)
	require.Len(t, got.Tokens, 2)
	assert.Equal(t, "tok-p1", got.Tokens["p1"])
	assert.Equal(t, "tok-p2", got.Tokens["p2"])
	assert.Empty(t, fake.gotTeams, "FFA não envia teams")
}

// ffa monta uma composição FFA (um lado sem TeamID) para os testes.
func ffa(players ...string) []domain.TeamAssignment {
	return []domain.TeamAssignment{{Players: players}}
}

// Com times, o request leva players (união) e o repeated Team (ADR-0023).
func TestProvision_ComTimes_EnviaPlayersETeams(t *testing.T) {
	fake := &fakeSessionServer{}
	c := setup(t, fake)

	teams := []domain.TeamAssignment{
		{TeamID: "team-1", Players: []string{"p1", "p2"}},
		{TeamID: "team-2", Players: []string{"p3", "p4"}},
	}
	_, err := c.Provision(context.Background(), "match-1", teams, "game-1")
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"p1", "p2", "p3", "p4"}, fake.gotPlayers, "players = união dos times")
	require.Len(t, fake.gotTeams, 2)
	assert.Equal(t, "team-1", fake.gotTeams[0].GetTeamId())
	assert.Equal(t, []string{"p3", "p4"}, fake.gotTeams[1].GetPlayers())
}

func TestProvision_PoolEsgotado_ErrNoServer(t *testing.T) {
	c := setup(t, &fakeSessionServer{full: true})

	_, err := c.Provision(context.Background(), "m1", ffa("p1"), "game-1")
	assert.ErrorIs(t, err, domain.ErrNoServer)
}

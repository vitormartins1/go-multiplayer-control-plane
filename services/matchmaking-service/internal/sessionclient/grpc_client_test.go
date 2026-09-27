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
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/matchmaking-service/internal/domain"
	sessionv1 "github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/gen/session/v1"
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

func setup(t *testing.T, srvImpl sessionv1.SessionServer, dialOpts ...grpc.DialOption) *Client {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	sessionv1.RegisterSessionServer(srv, srvImpl)
	go func() { _ = srv.Serve(lis) }()

	opts := append([]grpc.DialOption{
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}, dialOpts...)
	conn, err := grpc.NewClient("passthrough://bufnet", opts...)
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

	got, err := c.Provision(context.Background(), "match-1", ffa("p1", "p2"), "game-1", "default")
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
	_, err := c.Provision(context.Background(), "match-1", teams, "game-1", "default")
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"p1", "p2", "p3", "p4"}, fake.gotPlayers, "players = união dos times")
	require.Len(t, fake.gotTeams, 2)
	assert.Equal(t, "team-1", fake.gotTeams[0].GetTeamId())
	assert.Equal(t, []string{"p3", "p4"}, fake.gotTeams[1].GetPlayers())
}

func TestProvision_PoolEsgotado_ErrNoServer(t *testing.T) {
	c := setup(t, &fakeSessionServer{full: true})

	_, err := c.Provision(context.Background(), "m1", ffa("p1"), "game-1", "default")
	assert.ErrorIs(t, err, domain.ErrNoServer)
}

// --- host_world_dropin (spec 006): diretório + injeção ---

// worldFake estende o contrato do session com os RPCs do diretório/injeção.
// joinCode != OK faz o JoinWorld falhar com esse código (corrida / mundo fechou).
type worldFake struct {
	sessionv1.UnimplementedSessionServer
	worlds      []*sessionv1.InvadableWorld
	gotListGame string
	gotListMode string
	gotJoin     *sessionv1.JoinWorldRequest
	gotKey      []string
	joinCode    codes.Code // OK = sucesso
	joinAddress string
	joinHost    string // session/031: mundo p2p
}

func (f *worldFake) ListInvadableWorlds(_ context.Context, req *sessionv1.ListInvadableWorldsRequest) (*sessionv1.ListInvadableWorldsResponse, error) {
	f.gotListGame = req.GetGameId()
	f.gotListMode = req.GetModeId()
	return &sessionv1.ListInvadableWorldsResponse{Worlds: f.worlds}, nil
}

func (f *worldFake) JoinWorld(ctx context.Context, req *sessionv1.JoinWorldRequest) (*sessionv1.JoinWorldResponse, error) {
	f.gotJoin = req
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		f.gotKey = md.Get("x-internal-key")
	}
	if f.joinCode != codes.OK {
		return nil, status.Error(f.joinCode, "join failed")
	}
	return &sessionv1.JoinWorldResponse{
		Grant:          &sessionv1.ConnectGrant{PlayerId: req.GetPlayerId(), ConnectToken: "tok-" + req.GetPlayerId()},
		ServerAddress:  f.joinAddress,
		HostConnection: f.joinHost,
	}, nil
}

func TestListInvadableWorlds_TraduzMundos(t *testing.T) {
	fake := &worldFake{worlds: []*sessionv1.InvadableWorld{
		{SessionId: "w1", HostSoulLevel: 100, WeaponTier: 5, FreeInvaderSlots: 1},
		{SessionId: "w2", HostSoulLevel: 200, WeaponTier: 8, FreeInvaderSlots: 2},
	}}
	c := setup(t, fake)

	worlds, err := c.ListInvadableWorlds(context.Background(), "game-1", "roda-aberta")
	require.NoError(t, err)
	assert.Equal(t, "game-1", fake.gotListGame)
	assert.Equal(t, "roda-aberta", fake.gotListMode, "spec 012: o modo vai para o diretório do session")
	require.Len(t, worlds, 2)
	assert.Equal(t, domain.InvadableWorld{SessionID: "w1", HostSoulLevel: 100, WeaponTier: 5, FreeInvaderSlots: 1}, worlds[0])
	assert.Equal(t, 2, worlds[1].FreeInvaderSlots)
}

func TestInject_Sucesso_DevolveServerEToken(t *testing.T) {
	fake := &worldFake{joinCode: codes.OK, joinAddress: "10.0.0.7:7777"}
	c := setup(t, fake)

	got, err := c.Inject(context.Background(), "w1", "invader-1", "invader", "")
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.7:7777", got.ServerAddress)
	assert.Equal(t, "tok-invader-1", got.ConnectToken)
	assert.Equal(t, "invader", fake.gotJoin.GetRole(), "papel invader repassado ao JoinWorld")
}

func TestInject_MundoP2P_DevolveHostConnection(t *testing.T) {
	c := setup(t, &worldFake{joinCode: codes.OK, joinHost: "steam:765"})
	got, err := c.Inject(context.Background(), "w1", "invader-1", "invader", "")
	require.NoError(t, err)
	assert.Empty(t, got.ServerAddress)
	assert.Equal(t, "steam:765", got.HostConnection)
}

func TestInject_SlotCheio_ErrRoleFull(t *testing.T) {
	c := setup(t, &worldFake{joinCode: codes.ResourceExhausted})
	_, err := c.Inject(context.Background(), "w1", "invader-1", "invader", "")
	assert.ErrorIs(t, err, domain.ErrRoleFull)
}

func TestInject_MundoFechado_ErrWorldNotFound(t *testing.T) {
	c := setup(t, &worldFake{joinCode: codes.NotFound})
	_, err := c.Inject(context.Background(), "w1", "invader-1", "invader", "")
	assert.ErrorIs(t, err, domain.ErrWorldNotFound)
}

// session/030: o SummonSign manda quem invocou; PermissionDenied do session
// (não é o host) vira ErrNotWorldHost.
func TestInject_Summoner_RepassaEMapeiaPermissionDenied(t *testing.T) {
	fake := &worldFake{joinCode: codes.OK}
	c := setup(t, fake)
	_, err := c.Inject(context.Background(), "w1", "aliado", "coop", "host-1")
	require.NoError(t, err)
	assert.Equal(t, "host-1", fake.gotJoin.GetSummonerPlayerId())

	c = setup(t, &worldFake{joinCode: codes.PermissionDenied})
	_, err = c.Inject(context.Background(), "w1", "aliado", "coop", "intruso")
	assert.ErrorIs(t, err, domain.ErrNotWorldHost)
}

// session/030: com as credenciais por RPC, toda chamada leva a internal key.
func TestInternalKeyCredentials_MandaAChaveNaMetadata(t *testing.T) {
	fake := &worldFake{joinCode: codes.OK}
	c := setup(t, fake, grpc.WithPerRPCCredentials(InternalKeyCredentials("k-123")))
	_, err := c.Inject(context.Background(), "w1", "invasor", "invader", "")
	require.NoError(t, err)
	assert.Equal(t, []string{"k-123"}, fake.gotKey)
}

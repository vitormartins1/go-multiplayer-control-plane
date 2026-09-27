package transport

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/gen/session/v1"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/allocator"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/domain"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/publisher"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/service"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/token"
)

// fakePlayers aceita "tok-<id>" como JWT válido do jogador <id>.
type fakePlayers struct{}

func (fakePlayers) Verify(_ context.Context, tok string) (string, error) {
	if len(tok) > 4 && tok[:4] == "tok-" {
		return tok[4:], nil
	}
	return "", errors.New("token inválido")
}

// countingRejections guarda as rejeições por "method|reason".
type countingRejections struct {
	mu sync.Mutex
	n  map[string]int
}

func (c *countingRejections) Inc(method, reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n[method+"|"+reason]++
}

func (c *countingRejections) get(method, reason string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n[method+"|"+reason]
}

const (
	testInternalKey = "chave-interna"
	testServerKey   = "chave-servidor"
)

// setupAuth sobe o server com interceptor + checagens por mundo no modo dado.
func setupAuth(t *testing.T, mode AuthMode) (pb.SessionClient, *countingRejections) {
	t.Helper()
	return setupAuthTopology(t, mode, "dedicated")
}

// setupAuthTopology é o setupAuth com a topologia do modo escolhida (spec 031).
func setupAuthTopology(t *testing.T, mode AuthMode, topology string) (pb.SessionClient, *countingRejections) {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	iss, err := token.Generate()
	require.NoError(t, err)
	prof := stubProfile{profile: domain.ProfileSnapshot{
		Archetype: "host_world_dropin", Topology: topology, ParamsJSON: []byte(`{}`), Hash: "h",
		EnabledRoles: []string{domain.RoleCoop, domain.RoleInvader}, CoopSlots: 2, InvaderSlots: 1,
	}}
	pub := publisher.New(slogDiscard())
	svc := service.New(allocator.New([]string{"10.0.0.1:7777", "10.0.0.2:7777"}), iss, pub,
		service.WithProfileProvider(prof), service.WithParticipationPublisher(pub))

	rej := &countingRejections{n: map[string]int{}}
	cfg := AuthConfig{Mode: mode, Players: fakePlayers{}, InternalKey: testInternalKey, GameServerKey: testServerKey, WorldTokens: svc, Rejections: rej}
	srv := grpc.NewServer(grpc.UnaryInterceptor(UnaryAuthInterceptor(cfg)))
	pb.RegisterSessionServer(srv, NewServer(svc, WithAuth(cfg)))
	go func() { _ = srv.Serve(lis) }()

	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close(); srv.Stop() })
	return pb.NewSessionClient(conn), rej
}

func asPlayer(id string) context.Context {
	return metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer tok-"+id)
}

func asInternal() context.Context {
	return metadata.AppendToOutgoingContext(context.Background(), "x-internal-key", testInternalKey)
}

func asServer() context.Context {
	return metadata.AppendToOutgoingContext(context.Background(), "x-game-server-key", testServerKey)
}

func TestAuth_Enforce_OpenWorld_HostVemDoToken(t *testing.T) {
	c, _ := setupAuth(t, AuthEnforce)

	resp, err := c.OpenWorld(asPlayer("host-1"), &pb.OpenWorldRequest{GameId: "hw"}) // sem host_player_id
	require.NoError(t, err)
	assert.Equal(t, "host-1", resp.GetHostGrant().GetPlayerId())
}

func TestAuth_Enforce_OpenWorld_EmNomeDeOutro_PermissionDenied(t *testing.T) {
	c, _ := setupAuth(t, AuthEnforce)

	_, err := c.OpenWorld(asPlayer("host-1"), &pb.OpenWorldRequest{GameId: "hw", HostPlayerId: "vitima"})
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestAuth_Enforce_SemCredencialOuCredencialErrada_Unauthenticated(t *testing.T) {
	c, rej := setupAuth(t, AuthEnforce)

	_, err := c.OpenWorld(context.Background(), &pb.OpenWorldRequest{GameId: "hw", HostPlayerId: "h"})
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	assert.Equal(t, 1, rej.get("/session.v1.Session/OpenWorld", "missing_credential"))

	bad := metadata.AppendToOutgoingContext(context.Background(), "x-game-server-key", "errada")
	_, err = c.ReportParticipationEvent(bad, &pb.ReportParticipationEventRequest{SessionId: "s"})
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	assert.Equal(t, 1, rej.get("/session.v1.Session/ReportParticipationEvent", "invalid_credential"))
}

// Cada tipo de chamador só alcança os RPCs da tabela (spec 030).
func TestAuth_Enforce_TabelaDePermissoes(t *testing.T) {
	c, _ := setupAuth(t, AuthEnforce)
	denied := func(err error) bool { return status.Code(err) == codes.PermissionDenied }

	_, err := c.ListInvadableWorlds(asPlayer("p1"), &pb.ListInvadableWorldsRequest{GameId: "hw"})
	assert.True(t, denied(err), "jogador não lista mundos direto (é o matchmaking)")
	_, err = c.JoinWorld(asPlayer("p1"), &pb.JoinWorldRequest{SessionId: "s", PlayerId: "p1", Role: "coop"})
	assert.True(t, denied(err), "jogador não se injeta direto")
	_, err = c.ReportParticipationEvent(asPlayer("p1"), &pb.ReportParticipationEventRequest{SessionId: "s"})
	assert.True(t, denied(err), "jogador não reporta evento")
	_, err = c.OpenWorld(asServer(), &pb.OpenWorldRequest{GameId: "hw", HostPlayerId: "h"})
	assert.True(t, denied(err), "game server não abre mundo")

	_, err = c.ListInvadableWorlds(asInternal(), &pb.ListInvadableWorldsRequest{GameId: "hw"})
	assert.NoError(t, err, "matchmaking lista mundos")
}

func TestAuth_Enforce_CloseWorld_SoOHostOuOServer(t *testing.T) {
	c, _ := setupAuth(t, AuthEnforce)
	w1, err := c.OpenWorld(asPlayer("host-1"), &pb.OpenWorldRequest{GameId: "hw"})
	require.NoError(t, err)

	_, err = c.CloseWorld(asPlayer("intruso"), &pb.CloseWorldRequest{SessionId: w1.GetSessionId()})
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	_, err = c.CloseWorld(asPlayer("host-1"), &pb.CloseWorldRequest{SessionId: w1.GetSessionId()})
	assert.NoError(t, err)

	w2, err := c.OpenWorld(asPlayer("host-2"), &pb.OpenWorldRequest{GameId: "hw"})
	require.NoError(t, err)
	_, err = c.CloseWorld(asServer(), &pb.CloseWorldRequest{SessionId: w2.GetSessionId()})
	assert.NoError(t, err, "o Dedicated Server fecha o mundo que hospeda")
}

func TestAuth_Enforce_LeaveWorld_SaiASiMesmoOuHostTiraOutro(t *testing.T) {
	c, _ := setupAuth(t, AuthEnforce)
	w, err := c.OpenWorld(asPlayer("host-1"), &pb.OpenWorldRequest{GameId: "hw"})
	require.NoError(t, err)
	sid := w.GetSessionId()
	for _, p := range []string{"coop-1", "coop-2"} {
		_, err = c.JoinWorld(asInternal(), &pb.JoinWorldRequest{SessionId: sid, PlayerId: p, Role: "coop"})
		require.NoError(t, err)
	}

	_, err = c.LeaveWorld(asPlayer("coop-1"), &pb.LeaveWorldRequest{SessionId: sid}) // player_id vazio = ele mesmo
	assert.NoError(t, err)
	_, err = c.LeaveWorld(asPlayer("coop-1"), &pb.LeaveWorldRequest{SessionId: sid, PlayerId: "coop-2"})
	assert.Equal(t, codes.PermissionDenied, status.Code(err), "phantom não tira outro phantom")
	_, err = c.LeaveWorld(asPlayer("host-1"), &pb.LeaveWorldRequest{SessionId: sid, PlayerId: "coop-2"})
	assert.NoError(t, err, "o host tira um phantom")
}

// SummonSign: o summoner precisa ser o host — em qualquer modo.
func TestAuth_JoinWorld_SummonerQueNaoEHost_PermissionDeniedMesmoEmWarn(t *testing.T) {
	c, _ := setupAuth(t, AuthWarn)
	w, err := c.OpenWorld(asPlayer("host-1"), &pb.OpenWorldRequest{GameId: "hw"})
	require.NoError(t, err)

	_, err = c.JoinWorld(asInternal(), &pb.JoinWorldRequest{SessionId: w.GetSessionId(), PlayerId: "aliado", Role: "coop", SummonerPlayerId: "intruso"})
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	_, err = c.JoinWorld(asInternal(), &pb.JoinWorldRequest{SessionId: w.GetSessionId(), PlayerId: "aliado", Role: "coop", SummonerPlayerId: "host-1"})
	assert.NoError(t, err)
}

// warn: a falha é contada, mas a chamada segue com a semântica anterior.
func TestAuth_Warn_ContaMasNaoBloqueia(t *testing.T) {
	c, rej := setupAuth(t, AuthWarn)

	w, err := c.OpenWorld(context.Background(), &pb.OpenWorldRequest{GameId: "hw", HostPlayerId: "host-legado"})
	require.NoError(t, err, "cliente antigo sem bearer continua abrindo mundo")
	assert.Equal(t, "host-legado", w.GetHostGrant().GetPlayerId())
	assert.Equal(t, 1, rej.get("/session.v1.Session/OpenWorld", "missing_credential"))

	_, err = c.CloseWorld(asPlayer("intruso"), &pb.CloseWorldRequest{SessionId: w.GetSessionId()})
	assert.NoError(t, err)
	assert.Equal(t, 1, rej.get("/session.v1.Session/CloseWorld", "not_world_host"))
}

func TestParseAuthMode_DesconhecidoViraWarn(t *testing.T) {
	assert.Equal(t, AuthEnforce, ParseAuthMode(" ENFORCE "))
	assert.Equal(t, AuthOff, ParseAuthMode("off"))
	assert.Equal(t, AuthWarn, ParseAuthMode(""))
	assert.Equal(t, AuthWarn, ParseAuthMode("sim"))
}

// --- spec 031: host de mundo p2p com world token ---

func asWorldHost(token string) context.Context {
	return metadata.AppendToOutgoingContext(context.Background(), "x-world-token", token)
}

func TestAuth_P2P_OpenWorld_DevolveHostConnectionEWorldToken(t *testing.T) {
	c, _ := setupAuthTopology(t, AuthEnforce, "p2p")

	w, err := c.OpenWorld(asPlayer("host-1"), &pb.OpenWorldRequest{GameId: "hw", HostConnection: "steam:765"})
	require.NoError(t, err)
	assert.Empty(t, w.GetServerAddress(), "p2p não tem game server")
	assert.Equal(t, "steam:765", w.GetHostConnection())
	assert.NotEmpty(t, w.GetWorldToken())

	j, err := c.JoinWorld(asInternal(), &pb.JoinWorldRequest{SessionId: w.GetSessionId(), PlayerId: "invasor", Role: "invader"})
	require.NoError(t, err)
	assert.Equal(t, "steam:765", j.GetHostConnection())
	assert.Empty(t, j.GetServerAddress())
}

func TestAuth_P2P_OpenWorldSemHostConnection_InvalidArgument(t *testing.T) {
	c, _ := setupAuthTopology(t, AuthEnforce, "p2p")
	_, err := c.OpenWorld(asPlayer("host-1"), &pb.OpenWorldRequest{GameId: "hw"})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestAuth_P2P_WorldToken_SoValeNoProprioMundo(t *testing.T) {
	c, rej := setupAuthTopology(t, AuthEnforce, "p2p")
	w1, err := c.OpenWorld(asPlayer("host-1"), &pb.OpenWorldRequest{GameId: "hw", HostConnection: "steam:1"})
	require.NoError(t, err)
	w2, err := c.OpenWorld(asPlayer("host-2"), &pb.OpenWorldRequest{GameId: "hw", HostConnection: "steam:2"})
	require.NoError(t, err)
	tok := w1.GetWorldToken()

	evt := func(sid string) *pb.ReportParticipationEventRequest {
		return &pb.ReportParticipationEventRequest{SessionId: sid, PlayerId: "host-1", Role: "host", Event: "BOSS_DEFEATED", Outcome: "SUCCESS"}
	}
	_, err = c.ReportParticipationEvent(asWorldHost(tok), evt(w1.GetSessionId()))
	assert.NoError(t, err, "o host reporta no próprio mundo")

	_, err = c.ReportParticipationEvent(asWorldHost(tok), evt(w2.GetSessionId()))
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.Equal(t, 1, rej.get("/session.v1.Session/ReportParticipationEvent", "other_world"))
	_, err = c.CloseWorld(asWorldHost(tok), &pb.CloseWorldRequest{SessionId: w2.GetSessionId()})
	assert.Equal(t, codes.PermissionDenied, status.Code(err))

	_, err = c.OpenWorld(asWorldHost(tok), &pb.OpenWorldRequest{GameId: "hw", HostConnection: "steam:1"})
	assert.Equal(t, codes.PermissionDenied, status.Code(err), "world token não abre mundo")

	_, err = c.GetSessionConfig(asWorldHost(tok), &pb.GetSessionConfigRequest{RefId: w1.GetSessionId()})
	assert.NoError(t, err, "o host lê o manifest do próprio mundo")

	_, err = c.CloseWorld(asWorldHost(tok), &pb.CloseWorldRequest{SessionId: w1.GetSessionId()})
	require.NoError(t, err)
	_, err = c.ReportParticipationEvent(asWorldHost(tok), evt(w1.GetSessionId()))
	assert.Equal(t, codes.Unauthenticated, status.Code(err), "mundo fechado: o token morre junto")
}

func TestAuth_P2P_WorldTokenDesconhecido_Unauthenticated(t *testing.T) {
	c, rej := setupAuthTopology(t, AuthEnforce, "p2p")
	_, err := c.ReportParticipationEvent(asWorldHost("inventado"), &pb.ReportParticipationEventRequest{SessionId: "s"})
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	assert.Equal(t, 1, rej.get("/session.v1.Session/ReportParticipationEvent", "invalid_credential"))
}

// --- spec 032: heartbeat do mundo ---

func TestAuth_Heartbeat_HostWorldTokenEServer(t *testing.T) {
	c, _ := setupAuthTopology(t, AuthEnforce, "p2p")
	w, err := c.OpenWorld(asPlayer("host-1"), &pb.OpenWorldRequest{GameId: "hw", HostConnection: "steam:1"})
	require.NoError(t, err)
	other, err := c.OpenWorld(asPlayer("host-2"), &pb.OpenWorldRequest{GameId: "hw", HostConnection: "steam:2"})
	require.NoError(t, err)
	sid := w.GetSessionId()
	req := &pb.HeartbeatRequest{SessionId: sid}

	resp, err := c.Heartbeat(asWorldHost(w.GetWorldToken()), req)
	require.NoError(t, err)
	assert.Positive(t, resp.GetIntervalSeconds())
	_, err = c.Heartbeat(asPlayer("host-1"), req)
	assert.NoError(t, err, "o host pelo bearer")
	_, err = c.Heartbeat(asServer(), req)
	assert.NoError(t, err, "o Dedicated Server")

	_, err = c.Heartbeat(asPlayer("intruso"), req)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	_, err = c.Heartbeat(asWorldHost(other.GetWorldToken()), req)
	assert.Equal(t, codes.PermissionDenied, status.Code(err), "world token de outro mundo")
	_, err = c.Heartbeat(asInternal(), req)
	assert.Equal(t, codes.PermissionDenied, status.Code(err), "matchmaking não manda heartbeat")
	_, err = c.Heartbeat(asServer(), &pb.HeartbeatRequest{SessionId: "nao-existe"})
	assert.Equal(t, codes.NotFound, status.Code(err))
}

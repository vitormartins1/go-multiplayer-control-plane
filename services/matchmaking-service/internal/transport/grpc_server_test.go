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

	pb "github.com/vitormartins1/go-multiplayer-control-plane/services/matchmaking-service/gen/matchmaking/v1"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/matchmaking-service/internal/domain"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/matchmaking-service/internal/service"
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
type provisionerFunc func(matchID string, teams []domain.TeamAssignment) (domain.Provisioned, error)

func (f provisionerFunc) Provision(_ context.Context, matchID string, teams []domain.TeamAssignment, _, _ string) (domain.Provisioned, error) {
	return f(matchID, teams)
}

// okProvisioner devolve server fixo e um token por jogador (tok-<player>).
var okProvisioner = provisionerFunc(func(_ string, teams []domain.TeamAssignment) (domain.Provisioned, error) {
	tokens := map[string]string{}
	for _, t := range teams {
		for _, p := range t.Players {
			tokens[p] = "tok-" + p
		}
	}
	return domain.Provisioned{ServerAddress: "10.0.0.1:7777", Tokens: tokens}, nil
})

// stubProvider é um domain.ProfileProvider fixo (formação ou erro) para os testes.
type stubProvider struct {
	size int
	err  error
}

func (s stubProvider) Format(context.Context, string, string) (domain.MatchFormat, error) {
	return domain.MatchFormat{Size: s.size, Structure: domain.StructureFFA}, s.err
}

func setup(t *testing.T, prov domain.SessionProvisioner) pb.MatchmakingClient {
	return setupWith(t, prov, stubProvider{size: 2})
}

func setupWith(t *testing.T, prov domain.SessionProvisioner, profile domain.ProfileProvider) pb.MatchmakingClient {
	return setupFull(t, prov, profile, stubInvader{}, defaultSummoner())
}

// defaultSummoner é um Summoner real com stubs — usado onde o co-op não é o foco.
func defaultSummoner() domain.Summoner {
	return service.NewSummoner(coopPolicyStub{policy: coopSupported()}, injectorStub{addr: "10.0.0.9:7777"})
}

// setupFull sobe o server com os dois interceptors (stream + unary) e as três
// Ports de entrada injetáveis — os testes de cada modo passam a sua.
func setupFull(t *testing.T, prov domain.SessionProvisioner, profile domain.ProfileProvider, inv domain.Invader, sum domain.Summoner) pb.MatchmakingClient {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	mm := service.New(10*time.Millisecond, profile, prov)
	ctx, cancel := context.WithCancel(context.Background())
	go mm.Run(ctx)

	srv := grpc.NewServer(
		grpc.StreamInterceptor(StreamAuthInterceptor(fakeVerifier{})),
		grpc.UnaryInterceptor(UnaryAuthInterceptor(fakeVerifier{})),
	)
	pb.RegisterMatchmakingServer(srv, NewServer(mm, inv, sum))
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

// stubInvader adapta um resultado/erro fixo à Port domain.Invader. gotPlayerID
// captura o playerID que chegou via interceptor (prova o wire do JWT no unary).
type stubInvader struct {
	result      domain.Invaded
	err         error
	gotPlayerID *string
	gotCriteria *domain.InvaderCriteria
}

func (s stubInvader) Invade(_ context.Context, _, _, playerID string, crit domain.InvaderCriteria) (domain.Invaded, error) {
	if s.gotPlayerID != nil {
		*s.gotPlayerID = playerID
	}
	if s.gotCriteria != nil {
		*s.gotCriteria = crit
	}
	return s.result, s.err
}

// coopPolicyStub / injectorStub alimentam um Summoner real nos testes de co-op.
type coopPolicyStub struct{ policy domain.CoopPolicy }

func (c coopPolicyStub) CoopPolicy(context.Context, string, string) (domain.CoopPolicy, error) {
	return c.policy, nil
}

type injectorStub struct{ addr string }

func (i injectorStub) Inject(_ context.Context, _, playerID, _ string) (domain.Injected, error) {
	return domain.Injected{ServerAddress: i.addr, ConnectToken: "tok-" + playerID}, nil
}

// coopSupported é uma política host_world_dropin com co-op ligado, sem filtro de
// faixa (todo sinal casa) — simplifica os testes de transport.
func coopSupported() domain.CoopPolicy {
	return domain.CoopPolicy{Archetype: "host_world_dropin", CoopEnabled: true, LevelRangeEnabled: false}
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
	noServer := provisionerFunc(func(string, []domain.TeamAssignment) (domain.Provisioned, error) {
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

func TestFindMatch_SemPerfil_FailedPrecondition(t *testing.T) {
	client := setupWith(t, okProvisioner, stubProvider{err: domain.ErrProfileUnavailable})

	s, err := client.FindMatch(authCtx("p1"), &pb.FindMatchRequest{GameId: "sem-perfil"})
	require.NoError(t, err)

	_, err = s.Recv()
	st, _ := status.FromError(err)
	assert.Equal(t, codes.FailedPrecondition, st.Code(), "jogo sem perfil -> FailedPrecondition")
}

// --- Invade (host_world_dropin, spec 006) ---

func TestInvade_Sucesso_DevolveServerETokenEPlayerDoJWT(t *testing.T) {
	var gotPlayer string
	inv := stubInvader{
		result:      domain.Invaded{SessionID: "w1", ServerAddress: "10.0.0.7:7777", ConnectToken: "tok-invader"},
		gotPlayerID: &gotPlayer,
	}
	client := setupFull(t, okProvisioner, stubProvider{size: 2}, inv, defaultSummoner())

	resp, err := client.Invade(authCtx("invader-1"), &pb.InvadeRequest{GameId: "hw", SoulLevel: 100})
	require.NoError(t, err)
	assert.Equal(t, "w1", resp.GetSessionId())
	assert.Equal(t, "10.0.0.7:7777", resp.GetServerAddress())
	assert.Equal(t, "tok-invader", resp.GetConnectToken())
	assert.Equal(t, "invader-1", gotPlayer, "playerID veio do JWT via UnaryAuthInterceptor")
}

func TestInvade_SemToken_Unauthenticated(t *testing.T) {
	client := setupFull(t, okProvisioner, stubProvider{size: 2}, stubInvader{}, defaultSummoner())

	_, err := client.Invade(context.Background(), &pb.InvadeRequest{GameId: "hw", SoulLevel: 100})
	assert.Equal(t, codes.Unauthenticated, status.Code(err), "sem Bearer -> Unauthenticated")
}

func TestInvade_SemGameID_InvalidArgument(t *testing.T) {
	client := setupFull(t, okProvisioner, stubProvider{size: 2}, stubInvader{}, defaultSummoner())

	_, err := client.Invade(authCtx("invader-1"), &pb.InvadeRequest{SoulLevel: 100})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestInvade_NaoSuportado_FailedPrecondition(t *testing.T) {
	client := setupFull(t, okProvisioner, stubProvider{size: 2}, stubInvader{err: domain.ErrInvasionNotSupported}, defaultSummoner())

	_, err := client.Invade(authCtx("invader-1"), &pb.InvadeRequest{GameId: "hw", SoulLevel: 100})
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestInvade_Senha_ChegaAoServiceComoCriteria(t *testing.T) {
	var gotCrit domain.InvaderCriteria
	inv := stubInvader{result: domain.Invaded{SessionID: "w1"}, gotCriteria: &gotCrit}
	client := setupFull(t, okProvisioner, stubProvider{size: 2}, inv, defaultSummoner())

	_, err := client.Invade(authCtx("invader-1"), &pb.InvadeRequest{GameId: "hw", SoulLevel: 100, Password: "abrepalavra"})
	require.NoError(t, err)
	assert.Equal(t, "abrepalavra", gotCrit.Password)
}

func TestInvade_NenhumMundo_NotFound(t *testing.T) {
	client := setupFull(t, okProvisioner, stubProvider{size: 2}, stubInvader{err: domain.ErrNoEligibleWorld}, defaultSummoner())

	_, err := client.Invade(authCtx("invader-1"), &pb.InvadeRequest{GameId: "hw", SoulLevel: 100})
	assert.Equal(t, codes.NotFound, status.Code(err))
}

// --- co-op por sinais (host_world_dropin, spec 007) ---

// setupCoop sobe o server com um Summoner real (stubs de policy + injector),
// para dirigir post → list → summon ponta a ponta via bufconn.
func setupCoop(t *testing.T, policy domain.CoopPolicy) (pb.MatchmakingClient, *service.Summoner) {
	sum := service.NewSummoner(coopPolicyStub{policy: policy}, injectorStub{addr: "10.0.0.9:7777"})
	client := setupFull(t, okProvisioner, stubProvider{size: 2}, stubInvader{}, sum)
	return client, sum
}

// Fluxo completo: aliado anuncia (POSTED) → host lista e invoca → aliado recebe
// SUMMONED com server+token → o sinal some da lista.
func TestSummonSigns_FluxoCompleto(t *testing.T) {
	client, _ := setupCoop(t, coopSupported())

	stream, err := client.PostSummonSign(authCtx("ally-1"), &pb.PostSummonSignRequest{GameId: "hw", SoulLevel: 100})
	require.NoError(t, err)
	posted, err := stream.Recv()
	require.NoError(t, err)
	assert.Equal(t, pb.SummonStatus_SUMMON_STATUS_POSTED, posted.GetStatus())
	signID := posted.GetSignId()
	require.NotEmpty(t, signID)

	list, err := client.ListSigns(authCtx("host-1"), &pb.ListSignsRequest{GameId: "hw", HostSoulLevel: 100})
	require.NoError(t, err)
	require.Len(t, list.GetSigns(), 1)
	assert.Equal(t, "ally-1", list.GetSigns()[0].GetPlayerId())

	summon, err := client.SummonSign(authCtx("host-1"), &pb.SummonSignRequest{SessionId: "world-7", SignId: signID})
	require.NoError(t, err)
	assert.Equal(t, "ally-1", summon.GetPlayerId())

	// o aliado recebe SUMMONED pelo stream aberto.
	summoned, err := stream.Recv()
	require.NoError(t, err)
	assert.Equal(t, pb.SummonStatus_SUMMON_STATUS_SUMMONED, summoned.GetStatus())
	assert.Equal(t, "world-7", summoned.GetSessionId())
	assert.Equal(t, "10.0.0.9:7777", summoned.GetServerAddress())
	assert.Equal(t, "tok-ally-1", summoned.GetConnectToken())

	// consumido → some da lista.
	list, err = client.ListSigns(authCtx("host-1"), &pb.ListSignsRequest{GameId: "hw", HostSoulLevel: 100})
	require.NoError(t, err)
	assert.Empty(t, list.GetSigns())
}

// Senha (follow-up #1, matchmaking/011): aliado fora da faixa simétrica de
// co-op aparece pro host que informa a mesma senha.
func TestListSigns_SenhaBateForaDaFaixa_AparecePeloBufconn(t *testing.T) {
	client, _ := setupCoop(t, domain.CoopPolicy{Archetype: "host_world_dropin", CoopEnabled: true, LevelRangeEnabled: true})

	stream, err := client.PostSummonSign(authCtx("ally-1"), &pb.PostSummonSignRequest{GameId: "hw", SoulLevel: 300, Password: "abrepalavra"})
	require.NoError(t, err)
	_, err = stream.Recv() // POSTED
	require.NoError(t, err)

	list, err := client.ListSigns(authCtx("host-1"), &pb.ListSignsRequest{GameId: "hw", HostSoulLevel: 100, Password: "abrepalavra"})
	require.NoError(t, err)
	require.Len(t, list.GetSigns(), 1, "senha bate → aparece mesmo fora da faixa (aliado 300 vs host 100)")
	assert.Equal(t, "ally-1", list.GetSigns()[0].GetPlayerId())
}

func TestPostSummonSign_CoopNaoSuportado_FailedPrecondition(t *testing.T) {
	client, _ := setupCoop(t, domain.CoopPolicy{Archetype: "queue_arena"})

	stream, err := client.PostSummonSign(authCtx("ally-1"), &pb.PostSummonSignRequest{GameId: "hw", SoulLevel: 100})
	require.NoError(t, err)
	_, err = stream.Recv()
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestPostSummonSign_SemToken_Unauthenticated(t *testing.T) {
	client, _ := setupCoop(t, coopSupported())

	stream, err := client.PostSummonSign(context.Background(), &pb.PostSummonSignRequest{GameId: "hw", SoulLevel: 100})
	require.NoError(t, err)
	_, err = stream.Recv()
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestSummonSign_SinalInexistente_NotFound(t *testing.T) {
	client, _ := setupCoop(t, coopSupported())

	_, err := client.SummonSign(authCtx("host-1"), &pb.SummonSignRequest{SessionId: "world-1", SignId: "nope"})
	assert.Equal(t, codes.NotFound, status.Code(err))
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

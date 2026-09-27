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

	pb "github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/gen/session/v1"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/allocator"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/domain"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/publisher"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/service"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/token"
)

// stubProfile é um ProfileProvider fixo para os testes de host_world: só o jogo
// "hw" tem perfil host_world_dropin; qualquer outro devolve um snapshot
// queue_arena (o Create genérico precisa de snapshot desde o ADR-0024).
type stubProfile struct{ profile domain.ProfileSnapshot }

func (s stubProfile) ModeProfile(_ context.Context, gameID, modeID string) (domain.ProfileSnapshot, error) {
	if gameID == "hw" {
		p := s.profile
		p.ModeID = modeID
		return p, nil
	}
	return domain.ProfileSnapshot{
		ModeID: modeID, Archetype: domain.ArchetypeQueueArena, Topology: "dedicated",
		ParamsJSON: []byte(`{}`), Hash: "stub",
	}, nil
}

// setupWorld sobe o server gRPC com um ProfileProvider host_world_dropin.
func setupWorld(t *testing.T, addresses []string) (pb.SessionClient, *token.ConnectIssuer) {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	iss, err := token.Generate()
	require.NoError(t, err)
	prof := stubProfile{profile: domain.ProfileSnapshot{
		Archetype: "host_world_dropin", Topology: "dedicated", ParamsJSON: []byte(`{"enabledRoles":["coop","invader"],"coopSlots":2,"invaderSlots":1}`),
		Hash: "hw-hash", EnabledRoles: []string{domain.RoleCoop, domain.RoleInvader},
		CoopSlots: 2, InvaderSlots: 1,
	}}
	pub := publisher.New(slogDiscard())
	svc := service.New(allocator.New(addresses), iss, pub,
		service.WithProfileProvider(prof),
		service.WithParticipationPublisher(pub)) // Log satisfaz as duas Ports (spec 013)

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

// TestOpenWorld_FleetIndisponivel_FailedPrecondition (ADR-0026, spec 022):
// allocator recusa sem Fleet configurada — mesmo mapeamento do CreateSession.
func TestOpenWorld_FleetIndisponivel_FailedPrecondition(t *testing.T) {
	iss, err := token.Generate()
	require.NoError(t, err)
	prof := stubProfile{profile: domain.ProfileSnapshot{
		Archetype: "host_world_dropin", Topology: "dedicated", ParamsJSON: []byte(`{"enabledRoles":["coop"],"coopSlots":2}`),
		Hash: "hw-hash", EnabledRoles: []string{domain.RoleCoop}, CoopSlots: 2,
	}}
	svc := service.New(fleetlessAllocator{}, iss, publisher.New(slogDiscard()), service.WithProfileProvider(prof))
	srv := grpc.NewServer()
	pb.RegisterSessionServer(srv, NewServer(svc))
	lis := bufconn.Listen(1024 * 1024)
	go func() { _ = srv.Serve(lis) }()
	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close(); srv.Stop() })
	client := pb.NewSessionClient(conn)

	_, err = client.OpenWorld(context.Background(), &pb.OpenWorldRequest{HostPlayerId: "h", GameId: "hw"})
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

// Diretório de invadíveis (spec 012): abrir mundo com atributos → aparece na
// lista com a faixa e a vaga; encher o slot de invasor → some da lista. O join
// devolve o endereço do server (furo de contrato herdado da H0).
func TestListInvadableWorlds_RefleteAberturaEEsgotamento(t *testing.T) {
	client, _ := setupWorld(t, []string{"10.0.0.1:7777", "10.0.0.2:7777"})
	ctx := context.Background()

	open, err := client.OpenWorld(ctx, &pb.OpenWorldRequest{HostPlayerId: "host-1", GameId: "hw", HostSoulLevel: 90, WeaponTier: 4})
	require.NoError(t, err)
	sid := open.GetSessionId()

	list, err := client.ListInvadableWorlds(ctx, &pb.ListInvadableWorldsRequest{GameId: "hw"})
	require.NoError(t, err)
	require.Len(t, list.GetWorlds(), 1)
	w := list.GetWorlds()[0]
	assert.Equal(t, sid, w.GetSessionId())
	assert.Equal(t, int32(90), w.GetHostSoulLevel())
	assert.Equal(t, int32(4), w.GetWeaponTier())
	assert.Equal(t, int32(1), w.GetFreeInvaderSlots())

	// injeta um invasor (InvaderSlots=1) → slot esgota; o join devolve o addr.
	join, err := client.JoinWorld(ctx, &pb.JoinWorldRequest{SessionId: sid, PlayerId: "inv-1", Role: "invader"})
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.1:7777", join.GetServerAddress())

	list, err = client.ListInvadableWorlds(ctx, &pb.ListInvadableWorldsRequest{GameId: "hw"})
	require.NoError(t, err)
	assert.Empty(t, list.GetWorlds(), "mundo com invasor cheio sai do diretório")
}

func TestListInvadableWorlds_SemMundos_ListaVazia(t *testing.T) {
	client, _ := setupWorld(t, []string{"10.0.0.1:7777"})
	list, err := client.ListInvadableWorlds(context.Background(), &pb.ListInvadableWorldsRequest{GameId: "hw"})
	require.NoError(t, err)
	assert.Empty(t, list.GetWorlds())
}

// Senha (follow-up #1, spec 020): abrir com senha → o hash aparece no
// diretório de invadíveis; sem senha, o hash vem vazio.
func TestOpenWorld_ComSenha_HashApareceNoDiretorio(t *testing.T) {
	client, _ := setupWorld(t, []string{"10.0.0.1:7777"})
	ctx := context.Background()

	open, err := client.OpenWorld(ctx, &pb.OpenWorldRequest{HostPlayerId: "host-1", GameId: "hw", Password: "abrepalavra"})
	require.NoError(t, err)

	list, err := client.ListInvadableWorlds(ctx, &pb.ListInvadableWorldsRequest{GameId: "hw"})
	require.NoError(t, err)
	require.Len(t, list.GetWorlds(), 1)
	w := list.GetWorlds()[0]
	assert.Equal(t, open.GetSessionId(), w.GetSessionId())
	assert.NotEmpty(t, w.GetPasswordHash())
	assert.Equal(t, domain.HashWorldPassword("abrepalavra"), w.GetPasswordHash())
}

func TestOpenWorld_SemSenha_HashVazioNoDiretorio(t *testing.T) {
	client, _ := setupWorld(t, []string{"10.0.0.1:7777"})
	ctx := context.Background()

	_, err := client.OpenWorld(ctx, &pb.OpenWorldRequest{HostPlayerId: "host-1", GameId: "hw"})
	require.NoError(t, err)

	list, err := client.ListInvadableWorlds(ctx, &pb.ListInvadableWorldsRequest{GameId: "hw"})
	require.NoError(t, err)
	require.Len(t, list.GetWorlds(), 1)
	assert.Empty(t, list.GetWorlds()[0].GetPasswordHash())
}

// Eventos de participação (spec 013): LEFT libera o slot de invasor (o mundo
// reaparece no diretório); HOST_KILLED encerra o mundo; evento inválido → 400.
func TestReportParticipationEvent_FluxoDeSlotEEncerramento(t *testing.T) {
	client, _ := setupWorld(t, []string{"10.0.0.1:7777"})
	ctx := context.Background()

	open, err := client.OpenWorld(ctx, &pb.OpenWorldRequest{HostPlayerId: "host-1", GameId: "hw"})
	require.NoError(t, err)
	sid := open.GetSessionId()

	// invasor entra -> slot de invasor (cap 1) esgota -> some do diretório.
	_, err = client.JoinWorld(ctx, &pb.JoinWorldRequest{SessionId: sid, PlayerId: "inv-1", Role: "invader"})
	require.NoError(t, err)
	list, err := client.ListInvadableWorlds(ctx, &pb.ListInvadableWorldsRequest{GameId: "hw"})
	require.NoError(t, err)
	require.Empty(t, list.GetWorlds())

	// LEFT do invasor libera o slot -> o mundo volta ao diretório.
	_, err = client.ReportParticipationEvent(ctx, &pb.ReportParticipationEventRequest{
		SessionId: sid, PlayerId: "inv-1", Role: "invader", Event: "LEFT", Outcome: "NEUTRAL",
	})
	require.NoError(t, err)
	list, err = client.ListInvadableWorlds(ctx, &pb.ListInvadableWorldsRequest{GameId: "hw"})
	require.NoError(t, err)
	assert.Len(t, list.GetWorlds(), 1, "LEFT liberou a vaga de invasor")

	// HOST_KILLED encerra o mundo.
	_, err = client.ReportParticipationEvent(ctx, &pb.ReportParticipationEventRequest{
		SessionId: sid, PlayerId: "inv-2", Role: "invader", Event: "HOST_KILLED", Outcome: "SUCCESS",
	})
	require.NoError(t, err)
	_, err = client.JoinWorld(ctx, &pb.JoinWorldRequest{SessionId: sid, PlayerId: "x", Role: "coop"})
	assert.Equal(t, codes.NotFound, status.Code(err), "mundo encerrado")
}

// event_id (follow-up #4, spec 021) é passthrough puro: o gRPC aceita o campo
// sem erro; o repasse pro envelope é coberto no nível do publisher
// (sns_publisher_test.go), que tem acesso direto ao envelope serializado.
func TestReportParticipationEvent_ComEventID_Aceita(t *testing.T) {
	client, _ := setupWorld(t, []string{"10.0.0.1:7777"})
	ctx := context.Background()

	open, err := client.OpenWorld(ctx, &pb.OpenWorldRequest{HostPlayerId: "host-1", GameId: "hw"})
	require.NoError(t, err)
	_, err = client.JoinWorld(ctx, &pb.JoinWorldRequest{SessionId: open.GetSessionId(), PlayerId: "coop-1", Role: "coop"})
	require.NoError(t, err)

	_, err = client.ReportParticipationEvent(ctx, &pb.ReportParticipationEventRequest{
		SessionId: open.GetSessionId(), PlayerId: "coop-1", Role: "coop",
		Event: "BOSS_DEFEATED", Outcome: "SUCCESS", EventId: "22222222-2222-2222-2222-222222222222",
	})
	require.NoError(t, err)
}

// stats (spec 026) é passthrough puro, mesmo padrão do event_id acima: o
// gRPC aceita o campo sem erro; o repasse pro envelope é coberto no nível do
// publisher (sns_publisher_test.go).
func TestReportParticipationEvent_ComStats_Aceita(t *testing.T) {
	client, _ := setupWorld(t, []string{"10.0.0.1:7777"})
	ctx := context.Background()

	open, err := client.OpenWorld(ctx, &pb.OpenWorldRequest{HostPlayerId: "host-1", GameId: "hw"})
	require.NoError(t, err)
	_, err = client.JoinWorld(ctx, &pb.JoinWorldRequest{SessionId: open.GetSessionId(), PlayerId: "coop-1", Role: "coop"})
	require.NoError(t, err)

	_, err = client.ReportParticipationEvent(ctx, &pb.ReportParticipationEventRequest{
		SessionId: open.GetSessionId(), PlayerId: "coop-1", Role: "coop",
		Event: "BOSS_DEFEATED", Outcome: "SUCCESS", Stats: map[string]int64{"kills": 3, "revives": 1},
	})
	require.NoError(t, err)
}

func TestReportParticipationEvent_EventoInvalido_InvalidArgument(t *testing.T) {
	client, _ := setupWorld(t, []string{"10.0.0.1:7777"})
	open, err := client.OpenWorld(context.Background(), &pb.OpenWorldRequest{HostPlayerId: "host-1", GameId: "hw"})
	require.NoError(t, err)

	_, err = client.ReportParticipationEvent(context.Background(), &pb.ReportParticipationEventRequest{
		SessionId: open.GetSessionId(), PlayerId: "p", Role: "coop", Event: "FLEW_AWAY", Outcome: "NEUTRAL",
	})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// Spec matchmaking/012: mode_id no request filtra; o mundo aberto sem modo fica
// no "default" (normalizado no OpenWorld), e mode_id vazio lista todos.
func TestListInvadableWorlds_ModeID_FiltraSemNormalizar(t *testing.T) {
	client, _ := setupWorld(t, []string{"10.0.0.1:7777", "10.0.0.2:7777"})
	ctx := context.Background()
	_, err := client.OpenWorld(ctx, &pb.OpenWorldRequest{HostPlayerId: "host-1", GameId: "hw"})
	require.NoError(t, err)

	list, err := client.ListInvadableWorlds(ctx, &pb.ListInvadableWorldsRequest{GameId: "hw", ModeId: "default"})
	require.NoError(t, err)
	assert.Len(t, list.GetWorlds(), 1)

	list, err = client.ListInvadableWorlds(ctx, &pb.ListInvadableWorldsRequest{GameId: "hw", ModeId: "batizado"})
	require.NoError(t, err)
	assert.Empty(t, list.GetWorlds(), "outro modo não enxerga o mundo")

	list, err = client.ListInvadableWorlds(ctx, &pb.ListInvadableWorldsRequest{GameId: "hw"})
	require.NoError(t, err)
	assert.Len(t, list.GetWorlds(), 1, "sem mode_id: todos os modos")
}

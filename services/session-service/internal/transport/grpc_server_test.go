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

	pb "github.com/vitormartins1/go-game-backend/services/session-service/gen/session/v1"
	"github.com/vitormartins1/go-game-backend/services/session-service/internal/allocator"
	"github.com/vitormartins1/go-game-backend/services/session-service/internal/publisher"
	"github.com/vitormartins1/go-game-backend/services/session-service/internal/service"
	"github.com/vitormartins1/go-game-backend/services/session-service/internal/token"
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
		Outcome: &pb.ReportResultRequest_WinLossDraw{WinLossDraw: &pb.WinLossDrawResult{
			Players: []*pb.PlayerOutcome{{PlayerId: "p1", Outcome: "win", Score: 10, Stats: map[string]int64{"kills": 3}}},
		}},
	})
	require.NoError(t, err)

	// server liberado -> nova sessão consegue alocar de novo.
	_, err = client.CreateSession(context.Background(), &pb.CreateSessionRequest{MatchId: "m2", Players: []string{"p2"}})
	assert.NoError(t, err)
}

// winLossDrawReq monta um request válido para os testes de mapeamento de erro.
func winLossDrawReq(matchID string) *pb.ReportResultRequest {
	return &pb.ReportResultRequest{
		MatchId: matchID,
		Outcome: &pb.ReportResultRequest_WinLossDraw{WinLossDraw: &pb.WinLossDrawResult{
			Players: []*pb.PlayerOutcome{{PlayerId: "p1", Outcome: "win"}},
		}},
	}
}

func TestReportResult_MatchInexistente_NotFound(t *testing.T) {
	client, _ := setup(t, []string{"10.0.0.1:7777"})

	_, err := client.ReportResult(context.Background(), winLossDrawReq("nao-existe"))
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestReportResult_OutcomeAusente_InvalidArgument(t *testing.T) {
	client, _ := setup(t, []string{"10.0.0.1:7777"})

	// nenhum campo do oneof setado -> Kind vazio -> unsupported outcome.
	_, err := client.ReportResult(context.Background(), &pb.ReportResultRequest{MatchId: "m1"})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func slogDiscard() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

// Com times, o token de cada jogador carrega o claim team_id (ADR-0023, spec 010).
func TestCreateSession_ComTimes_TokenCarregaTeamID(t *testing.T) {
	client, iss := setup(t, []string{"10.0.0.1:7777"})

	resp, err := client.CreateSession(context.Background(), &pb.CreateSessionRequest{
		MatchId: "match-1",
		Players: []string{"p1", "p2"},
		Teams: []*pb.Team{
			{TeamId: "team-1", Players: []string{"p1"}},
			{TeamId: "team-2", Players: []string{"p2"}},
		},
	})
	require.NoError(t, err)
	require.Len(t, resp.GetGrants(), 2)

	want := map[string]string{"p1": "team-1", "p2": "team-2"}
	for _, g := range resp.GetGrants() {
		parsed, err := jwt.Parse(g.GetConnectToken(), func(*jwt.Token) (any, error) {
			return iss.PublicKey(), nil
		}, jwt.WithValidMethods([]string{"EdDSA"}))
		require.NoError(t, err)
		claims := parsed.Claims.(jwt.MapClaims)
		assert.Equal(t, want[g.GetPlayerId()], claims["team_id"])
	}
}

// toMatchOutcome mapeia o oneof placement (battle_royale, BR2) para o domínio,
// convertendo o placement int32 → int e preservando entries/survivedMs/stats.
func TestToMatchOutcome_Placement_MapeiaParaDominio(t *testing.T) {
	req := &pb.ReportResultRequest{
		MatchId: "m1",
		Outcome: &pb.ReportResultRequest_Placement{Placement: &pb.PlacementResult{
			Entries: []*pb.PlacementEntry{
				{PlayerId: "p1", TeamId: "squad-1", Placement: 1, SurvivedMs: 900000, Stats: map[string]int64{"kills": 4}},
				{PlayerId: "p3", Placement: 2, SurvivedMs: 500000},
			},
		}},
	}
	out := toMatchOutcome(req)

	assert.Equal(t, "placement", out.Kind)
	require.NotNil(t, out.Placement)
	require.Len(t, out.Placement.Entries, 2)
	assert.Equal(t, "p1", out.Placement.Entries[0].PlayerID)
	assert.Equal(t, "squad-1", out.Placement.Entries[0].TeamID)
	assert.Equal(t, 1, out.Placement.Entries[0].Placement)
	assert.Equal(t, int64(900000), out.Placement.Entries[0].SurvivedMs)
	assert.Equal(t, map[string]int64{"kills": 4}, out.Placement.Entries[0].Stats)
	assert.Equal(t, 2, out.Placement.Entries[1].Placement)
}

// oneof vazio → MatchOutcome{} (Kind vazio, cai como unsupported na validação).
func TestToMatchOutcome_OneofVazio_KindVazio(t *testing.T) {
	out := toMatchOutcome(&pb.ReportResultRequest{MatchId: "m1"})
	assert.Empty(t, out.Kind)
	assert.Nil(t, out.WinLossDraw)
	assert.Nil(t, out.Placement)
}

// Composição inconsistente (jogador em dois times) → InvalidArgument.
func TestCreateSession_TimesInconsistentes_InvalidArgument(t *testing.T) {
	client, _ := setup(t, []string{"10.0.0.1:7777"})

	_, err := client.CreateSession(context.Background(), &pb.CreateSessionRequest{
		MatchId: "match-1",
		Teams: []*pb.Team{
			{TeamId: "team-1", Players: []string{"p1"}},
			{TeamId: "team-2", Players: []string{"p1"}},
		},
	})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

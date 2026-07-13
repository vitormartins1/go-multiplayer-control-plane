package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/domain"
)

// discardLog é o logger silencioso para os testes do reconcile/sweeper.
func discardLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

type mockAllocator struct{ mock.Mock }

func (m *mockAllocator) Allocate(ctx context.Context, matchID, gameID, fleet string) (domain.Server, error) {
	args := m.Called(ctx, matchID, gameID, fleet)
	return args.Get(0).(domain.Server), args.Error(1)
}

func (m *mockAllocator) Release(ctx context.Context, serverID string) error {
	return m.Called(ctx, serverID).Error(0)
}

type mockIssuer struct{ mock.Mock }

func (m *mockIssuer) Issue(playerID, refID, serverAddr string, claims domain.ConnectClaims) (string, error) {
	args := m.Called(playerID, refID, serverAddr, claims)
	return args.String(0), args.Error(1)
}

// mockProfile é o ProfileProvider mockado para os testes de OpenWorld (spec 011).
type mockProfile struct{ mock.Mock }

func (m *mockProfile) ModeProfile(ctx context.Context, gameID, modeID string) (domain.ProfileSnapshot, error) {
	args := m.Called(ctx, gameID, modeID)
	return args.Get(0).(domain.ProfileSnapshot), args.Error(1)
}

// stubProfiles é o ProfileProvider default dos testes que não exercitam o
// perfil: devolve um snapshot queue_arena para qualquer (game, mode) — o
// Create exige snapshot desde o ADR-0024 (spec 017).
type stubProfiles struct{ archetype string }

func (s stubProfiles) ModeProfile(_ context.Context, _, modeID string) (domain.ProfileSnapshot, error) {
	a := s.archetype
	if a == "" {
		a = domain.ArchetypeQueueArena
	}
	return domain.ProfileSnapshot{
		ModeID: modeID, Archetype: a, Topology: "dedicated",
		ParamsJSON: []byte(`{"structure":"ffa","playerCount":2}`), Hash: "stub",
	}, nil
}

// ffaTeams monta uma composição FFA (um lado sem TeamID) a partir dos playerIDs.
func ffaTeams(players ...string) []domain.TeamAssignment {
	return []domain.TeamAssignment{{Players: players}}
}

func (m *mockIssuer) JWKS(ctx context.Context) ([]byte, error) {
	args := m.Called(ctx)
	return args.Get(0).([]byte), args.Error(1)
}

type mockPublisher struct{ mock.Mock }

func (m *mockPublisher) Publish(ctx context.Context, gameID, matchID, modeID string, outcome domain.MatchOutcome) error {
	return m.Called(ctx, gameID, matchID, modeID, outcome).Error(0)
}

type mockParticipation struct{ mock.Mock }

func (m *mockParticipation) PublishParticipation(ctx context.Context, gameID, sessionID, modeID string, evt domain.ParticipationEvent) error {
	return m.Called(ctx, gameID, sessionID, modeID, evt).Error(0)
}

type mockSessionClosedPublisher struct{ mock.Mock }

func (m *mockSessionClosedPublisher) PublishSessionClosed(ctx context.Context, sess *domain.Session) error {
	return m.Called(ctx, sess).Error(0)
}

// wldOutcome monta um MatchOutcome win_loss_draw com os jogadores dados.
func wldOutcome(players ...domain.PlayerOutcome) domain.MatchOutcome {
	return domain.MatchOutcome{Kind: domain.OutcomeWinLossDraw, WinLossDraw: &domain.WinLossDrawResult{Players: players}}
}

// validOutcome é um outcome válido genérico (passa a validação → chega ao estado).
func validOutcome() domain.MatchOutcome {
	return wldOutcome(domain.PlayerOutcome{PlayerID: "p1", Outcome: domain.ResultWin, Score: 10})
}

type mockRegistry struct{ mock.Mock }

func (m *mockRegistry) LiveServerIDs(ctx context.Context) (map[string]struct{}, error) {
	args := m.Called(ctx)
	if s := args.Get(0); s != nil {
		return s.(map[string]struct{}), args.Error(1)
	}
	return nil, args.Error(1)
}

// liveSet monta um conjunto de IDs vivos para o mockRegistry.
func liveSet(ids ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set
}

func TestCreate_NPlayers_GeraNGrantsComServerAddress(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	pub := &mockPublisher{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", "p1", "match-1", "10.0.0.1:7777", domain.ConnectClaims{ModeID: "default"}).Return("token-p1", nil)
	iss.On("Issue", "p2", "match-1", "10.0.0.1:7777", domain.ConnectClaims{ModeID: "default"}).Return("token-p2", nil)

	svc := New(alloc, iss, pub, WithProfileProvider(stubProfiles{}))
	sess, grants, err := svc.Create(context.Background(), "match-1", "game-1", "default", ffaTeams("p1", "p2"))

	require.NoError(t, err)
	assert.Equal(t, "10.0.0.1:7777", sess.ServerAddress)
	assert.Equal(t, "game-1", sess.GameID, "a sessão guarda o game_id")
	assert.NotEmpty(t, sess.ID)
	require.Len(t, grants, 2)
	assert.Equal(t, "p1", grants[0].PlayerID)
	assert.Equal(t, "token-p1", grants[0].Token)
	assert.Equal(t, "token-p2", grants[1].Token)
	assert.Equal(t, float64(1), svc.ActiveSessions())
	alloc.AssertExpectations(t)
	iss.AssertExpectations(t)
}

func TestCreate_PoolEsgotado_RetornaErrNoServer(t *testing.T) {
	alloc := &mockAllocator{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{}, domain.ErrNoServer)

	svc := New(alloc, &mockIssuer{}, &mockPublisher{}, WithProfileProvider(stubProfiles{}))
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", "default", ffaTeams("p1"))

	assert.ErrorIs(t, err, domain.ErrNoServer)
	assert.Equal(t, float64(0), svc.ActiveSessions())
}

// TestCreate_PropagaFleetDoSnapshotParaAllocate (ADR-0026, spec 022): a Fleet
// do snapshot (vinda do game-registry) chega ao allocator, sem lógica extra.
func TestCreate_PropagaFleetDoSnapshotParaAllocate(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	profiles := &mockProfile{}
	profiles.On("ModeProfile", mock.Anything, "game-1", "default").Return(domain.ProfileSnapshot{
		ModeID: "default", Archetype: domain.ArchetypeQueueArena, Topology: "dedicated",
		ParamsJSON: []byte(`{"structure":"ffa","playerCount":2}`), Hash: "h", Fleet: "capoeira-souls",
	}, nil)
	alloc.On("Allocate", mock.Anything, "match-1", "game-1", "capoeira-souls").Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)

	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(profiles))
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", "default", ffaTeams("p1"))

	require.NoError(t, err)
	alloc.AssertExpectations(t)
}

// TestCreate_FleetIndisponivel_NaoEmiteTokens (ADR-0026, spec 022): o erro do
// allocator propaga e a sessão não chega a emitir connect tokens.
func TestCreate_FleetIndisponivel_NaoEmiteTokens(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{}, domain.ErrFleetUnavailable)

	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(stubProfiles{}))
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", "default", ffaTeams("p1"))

	assert.ErrorIs(t, err, domain.ErrFleetUnavailable)
	iss.AssertNotCalled(t, "Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestReportResult_PublicaLiberaEncerra(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	pub := &mockPublisher{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	svc := New(alloc, iss, pub, WithProfileProvider(stubProfiles{}))
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", "default", ffaTeams("p1"))
	require.NoError(t, err)

	outcome := wldOutcome(domain.PlayerOutcome{PlayerID: "p1", Outcome: domain.ResultWin, Score: 10})
	pub.On("Publish", mock.Anything, "game-1", "match-1", "default", outcome).Return(nil) // recebe o game_id e o match_id da sessão
	alloc.On("Release", mock.Anything, "server-0").Return(nil)

	require.NoError(t, svc.ReportResult(context.Background(), "match-1", outcome))
	assert.Equal(t, float64(0), svc.ActiveSessions(), "sessão encerrada")
	pub.AssertExpectations(t)
	alloc.AssertExpectations(t)
}

// Publicação falha -> ReportResult retorna erro e NÃO encerra a sessão nem
// libera o server (ADR-0020: publica antes de fechar; o retry do Dedicated
// Server e o TTL sweeper cobrem). O server continua alocado.
func TestReportResult_PublishFalha_NaoEncerraSessao(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	pub := &mockPublisher{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	svc := New(alloc, iss, pub, WithProfileProvider(stubProfiles{}))
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", "default", ffaTeams("p1"))
	require.NoError(t, err)

	pub.On("Publish", mock.Anything, "game-1", "match-1", mock.Anything, mock.Anything).Return(errors.New("sns fora"))

	err = svc.ReportResult(context.Background(), "match-1", validOutcome())
	require.Error(t, err)
	assert.Equal(t, float64(1), svc.ActiveSessions(), "sessão continua ativa")
	alloc.AssertNotCalled(t, "Release", mock.Anything, mock.Anything)
	pub.AssertExpectations(t)
}

func TestReportResult_MatchInexistente_ErrSessionNotFound(t *testing.T) {
	svc := New(&mockAllocator{}, &mockIssuer{}, &mockPublisher{}, WithProfileProvider(stubProfiles{}))
	err := svc.ReportResult(context.Background(), "nao-existe", validOutcome())
	assert.ErrorIs(t, err, domain.ErrSessionNotFound)
}

// reportSetup cria um service com uma sessão aberta (match-1/game-1) pronta para
// o ReportResult (server-0, players p1/p2), no arquétipo default (queue_arena).
func reportSetup(t *testing.T) (*Service, *mockPublisher, *mockAllocator) {
	return reportSetupArchetype(t, domain.ArchetypeQueueArena)
}

// reportSetupArchetype é o reportSetup com o arquétipo do snapshot escolhido —
// a validação kind×arquétipo (spec 017) exige que o kind reportado case com o
// arquétipo do modo da sessão.
func reportSetupArchetype(t *testing.T, archetype string) (*Service, *mockPublisher, *mockAllocator) {
	t.Helper()
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	pub := &mockPublisher{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	svc := New(alloc, iss, pub, WithProfileProvider(stubProfiles{archetype: archetype}))
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", "default", ffaTeams("p1", "p2"))
	require.NoError(t, err)
	return svc, pub, alloc
}

func TestReportResult_Empate_Aceito(t *testing.T) {
	svc, pub, alloc := reportSetup(t)
	outcome := wldOutcome(
		domain.PlayerOutcome{PlayerID: "p1", Outcome: domain.ResultDraw, Score: 5},
		domain.PlayerOutcome{PlayerID: "p2", Outcome: domain.ResultDraw, Score: 5},
	)
	pub.On("Publish", mock.Anything, "game-1", "match-1", "default", outcome).Return(nil)
	alloc.On("Release", mock.Anything, "server-0").Return(nil)

	require.NoError(t, svc.ReportResult(context.Background(), "match-1", outcome), "draw é aceito (session não valida contra o perfil)")
	pub.AssertExpectations(t)
}

func TestReportResult_ComTimesEStats_Publica(t *testing.T) {
	svc, pub, alloc := reportSetup(t)
	outcome := wldOutcome(
		domain.PlayerOutcome{PlayerID: "p1", TeamID: "A", Outcome: domain.ResultWin, Score: 3, Stats: map[string]int64{"kills": 7}},
		domain.PlayerOutcome{PlayerID: "p2", TeamID: "B", Outcome: domain.ResultLoss, Score: 1},
	)
	pub.On("Publish", mock.Anything, "game-1", "match-1", "default", outcome).Return(nil)
	alloc.On("Release", mock.Anything, "server-0").Return(nil)

	require.NoError(t, svc.ReportResult(context.Background(), "match-1", outcome))
	pub.AssertExpectations(t)
}

func TestReportResult_KindNaoSuportado_ErrUnsupportedOutcome(t *testing.T) {
	svc, _, _ := reportSetup(t)
	// kind fora do conjunto conhecido (ex.: role_event ainda não implementado).
	err := svc.ReportResult(context.Background(), "match-1", domain.MatchOutcome{Kind: "role_event"})
	assert.ErrorIs(t, err, domain.ErrUnsupportedOutcome)
	assert.Equal(t, float64(1), svc.ActiveSessions(), "outcome inválido não encerra a sessão")
}

func TestReportResult_OutcomeInvalido_ErrInvalidOutcome(t *testing.T) {
	svc, _, _ := reportSetup(t)
	errSemPlayers := svc.ReportResult(context.Background(), "match-1", wldOutcome())
	assert.ErrorIs(t, errSemPlayers, domain.ErrInvalidOutcome, "win_loss_draw sem jogadores")

	errValor := svc.ReportResult(context.Background(), "match-1",
		wldOutcome(domain.PlayerOutcome{PlayerID: "p1", Outcome: "victory"}))
	assert.ErrorIs(t, errValor, domain.ErrInvalidOutcome, "outcome fora de win/loss/draw")
}

func TestSweep_SessaoExpirada_FechaComTTLELiberaServer(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)

	// clock injetável: cria a sessão em t0 e varre em t0+31m com TTL de 30m.
	now := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(stubProfiles{}), WithClock(func() time.Time { return now }))
	sess, _, err := svc.Create(context.Background(), "match-1", "game-1", "default", ffaTeams("p1"))
	require.NoError(t, err)
	assert.Equal(t, domain.SessionOpen, sess.State)
	assert.Equal(t, now, sess.OpenedAt)

	alloc.On("Release", mock.Anything, "server-0").Return(nil)
	now = now.Add(31 * time.Minute)

	assert.Equal(t, 1, svc.Sweep(context.Background(), 30*time.Minute))
	assert.Equal(t, float64(0), svc.ActiveSessions(), "sessão expirada saiu das ativas")
	assert.Equal(t, domain.SessionClosed, sess.State)
	assert.Equal(t, domain.CloseTTL, sess.CloseReason)
	assert.Equal(t, now, sess.ClosedAt)
	assert.Equal(t, float64(1), svc.ClosedTotal(domain.CloseTTL))
	alloc.AssertExpectations(t)
}

func TestSweep_SessaoDentroDoTTL_NaoFecha(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)

	now := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(stubProfiles{}), WithClock(func() time.Time { return now }))
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", "default", ffaTeams("p1"))
	require.NoError(t, err)

	now = now.Add(10 * time.Minute)
	assert.Equal(t, 0, svc.Sweep(context.Background(), 30*time.Minute))
	assert.Equal(t, float64(1), svc.ActiveSessions(), "sessão dentro do TTL segue ativa")
	alloc.AssertNotCalled(t, "Release", mock.Anything, mock.Anything)
}

func TestReconcile_ServerSumiuAposGrace_FechaComoAbandoned(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	reg := &mockRegistry{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)

	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(stubProfiles{}),
		WithClock(func() time.Time { return now }),
		WithServerRegistry(reg), WithOrphanGrace(time.Minute))
	sess, _, err := svc.Create(context.Background(), "match-1", "game-1", "default", ffaTeams("p1"))
	require.NoError(t, err)

	// passa o grace e o server sumiu da fleet (LIST devolve conjunto vazio).
	now = now.Add(2 * time.Minute)
	reg.On("LiveServerIDs", mock.Anything).Return(liveSet(), nil)
	alloc.On("Release", mock.Anything, "server-0").Return(nil)

	assert.Equal(t, 1, svc.reconcile(context.Background(), time.Minute, discardLog()))
	assert.Equal(t, float64(0), svc.ActiveSessions(), "órfã saiu das ativas")
	assert.Equal(t, domain.SessionClosed, sess.State)
	assert.Equal(t, domain.CloseAbandoned, sess.CloseReason)
	assert.Equal(t, float64(1), svc.ClosedTotal(domain.CloseAbandoned))
	alloc.AssertExpectations(t) // garante o Release do server abandonado
}

func TestReconcile_DentroDoGrace_NaoFecha(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	reg := &mockRegistry{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)

	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(stubProfiles{}),
		WithClock(func() time.Time { return now }),
		WithServerRegistry(reg), WithOrphanGrace(time.Minute))
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", "default", ffaTeams("p1"))
	require.NoError(t, err)

	// server já sumiu, mas a sessão tem só 30s de vida — grace protege.
	now = now.Add(30 * time.Second)
	reg.On("LiveServerIDs", mock.Anything).Return(liveSet(), nil)

	assert.Equal(t, 0, svc.reconcile(context.Background(), time.Minute, discardLog()))
	assert.Equal(t, float64(1), svc.ActiveSessions(), "sessão dentro do grace segue ativa")
	alloc.AssertNotCalled(t, "Release", mock.Anything, mock.Anything)
}

func TestReconcile_ErroNoList_NaoFechaNada(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	reg := &mockRegistry{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)

	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(stubProfiles{}),
		WithClock(func() time.Time { return now }),
		WithServerRegistry(reg), WithOrphanGrace(time.Minute))
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", "default", ffaTeams("p1"))
	require.NoError(t, err)

	now = now.Add(5 * time.Minute)
	reg.On("LiveServerIDs", mock.Anything).Return(nil, errors.New("k8s indisponível"))

	assert.Equal(t, 0, svc.reconcile(context.Background(), time.Minute, discardLog()), "falha no LIST não fecha nada")
	assert.Equal(t, float64(1), svc.ActiveSessions(), "sessão saudável preservada apesar do erro de leitura")
	alloc.AssertNotCalled(t, "Release", mock.Anything, mock.Anything)
}

func TestReconcile_ServerVivo_NaoFecha(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	reg := &mockRegistry{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)

	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(stubProfiles{}),
		WithClock(func() time.Time { return now }),
		WithServerRegistry(reg), WithOrphanGrace(time.Minute))
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", "default", ffaTeams("p1"))
	require.NoError(t, err)

	now = now.Add(5 * time.Minute)
	reg.On("LiveServerIDs", mock.Anything).Return(liveSet("server-0"), nil)

	assert.Equal(t, 0, svc.reconcile(context.Background(), time.Minute, discardLog()))
	assert.Equal(t, float64(1), svc.ActiveSessions(), "server vivo → sessão segue ativa")
	alloc.AssertNotCalled(t, "Release", mock.Anything, mock.Anything)
}

func TestReconcile_SemRegistry_NoOp(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)

	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(stubProfiles{}), WithClock(func() time.Time { return now })) // sem WithServerRegistry
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", "default", ffaTeams("p1"))
	require.NoError(t, err)

	now = now.Add(5 * time.Minute)
	assert.Equal(t, 0, svc.reconcile(context.Background(), time.Minute, discardLog()), "sem registry: reconcile é no-op")
	assert.Equal(t, float64(1), svc.ActiveSessions())
}

func TestReportResult_MarcaCloseReasonResult(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	pub := &mockPublisher{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	svc := New(alloc, iss, pub, WithProfileProvider(stubProfiles{}))
	sess, _, err := svc.Create(context.Background(), "match-1", "game-1", "default", ffaTeams("p1"))
	require.NoError(t, err)

	pub.On("Publish", mock.Anything, "game-1", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	alloc.On("Release", mock.Anything, "server-0").Return(nil)
	require.NoError(t, svc.ReportResult(context.Background(), "match-1", validOutcome()))

	assert.Equal(t, domain.SessionClosed, sess.State)
	assert.Equal(t, domain.CloseResult, sess.CloseReason)
	assert.Equal(t, float64(1), svc.ClosedTotal(domain.CloseResult))
	assert.Equal(t, float64(0), svc.ClosedTotal(domain.CloseTTL))
}

// newSvcComSessoes cria um service com allocator/issuer permissivos para os
// testes de histórico (spec 005).
func newSvcComSessoes(t *testing.T, opts ...Option) *Service {
	t.Helper()
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	alloc.On("Release", mock.Anything, mock.Anything).Return(nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	pub := &mockPublisher{}
	pub.On("Publish", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	return New(alloc, iss, pub, append([]Option{WithProfileProvider(stubProfiles{})}, opts...)...)
}

func TestSessionsByGame_AtivasEFechadasFiltradasEOrdenadas(t *testing.T) {
	now := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	svc := newSvcComSessoes(t, WithClock(func() time.Time { return now }))
	ctx := context.Background()

	_, _, err := svc.Create(ctx, "m-old", "g1", "default", ffaTeams("p1"))
	require.NoError(t, err)
	now = now.Add(time.Minute)
	_, _, err = svc.Create(ctx, "m-new", "g1", "default", ffaTeams("p2"))
	require.NoError(t, err)
	_, _, err = svc.Create(ctx, "m-outro-jogo", "g2", "default", ffaTeams("p3"))
	require.NoError(t, err)

	now = now.Add(time.Minute)
	require.NoError(t, svc.ReportResult(ctx, "m-old", validOutcome()))

	active, closed := svc.SessionsByGame("g1")
	require.Len(t, active, 1, "sessão de outro jogo não vaza")
	assert.Equal(t, "m-new", active[0].MatchID)
	require.Len(t, closed, 1)
	assert.Equal(t, "m-old", closed[0].MatchID)
	assert.Equal(t, domain.CloseResult, closed[0].CloseReason)

	activeG2, closedG2 := svc.SessionsByGame("g2")
	assert.Len(t, activeG2, 1)
	assert.Empty(t, closedG2)
}

func TestHistorico_RespeitaLimite_MaisAntigaSai(t *testing.T) {
	svc := newSvcComSessoes(t, WithHistoryLimit(2))
	ctx := context.Background()

	for _, m := range []string{"m1", "m2", "m3"} {
		_, _, err := svc.Create(ctx, m, "g1", "default", ffaTeams("p"))
		require.NoError(t, err)
		require.NoError(t, svc.ReportResult(ctx, m, validOutcome()))
	}

	_, closed := svc.SessionsByGame("g1")
	require.Len(t, closed, 2, "ring bounded")
	assert.Equal(t, "m3", closed[0].MatchID, "mais recente primeiro")
	assert.Equal(t, "m2", closed[1].MatchID, "m1 (mais antiga) saiu")
}

func TestCreate_FalhaAoEmitirToken_LiberaServer(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", "p1", "match-1", "10.0.0.1:7777", domain.ConnectClaims{ModeID: "default"}).Return("", assert.AnError)
	alloc.On("Release", mock.Anything, "server-0").Return(nil)

	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(stubProfiles{}))
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", "default", ffaTeams("p1"))

	require.Error(t, err)
	assert.Equal(t, float64(0), svc.ActiveSessions())
	alloc.AssertExpectations(t) // garante o Release de rollback
}

func TestCloseByAdmin_Ativa_FechaELiberaServer(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	alloc.On("Release", mock.Anything, "server-0").Return(nil)
	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(stubProfiles{}))
	_, _, err := svc.Create(context.Background(), "m1", "g1", "default", ffaTeams("p1"))
	require.NoError(t, err)

	sess, err := svc.CloseByAdmin(context.Background(), "m1")
	require.NoError(t, err)
	assert.Equal(t, domain.CloseAdmin, sess.CloseReason)
	assert.Equal(t, float64(0), svc.ActiveSessions(), "sai das ativas")
	alloc.AssertExpectations(t)
}

func TestCloseByAdmin_Inexistente_ErrSessionNotFound(t *testing.T) {
	svc := New(&mockAllocator{}, &mockIssuer{}, &mockPublisher{}, WithProfileProvider(stubProfiles{}))
	_, err := svc.CloseByAdmin(context.Background(), "nao-existe")
	assert.ErrorIs(t, err, domain.ErrSessionNotFound)
}

// TestCloseByAdmin_PublicaSessionClosed (spec 028): closeSession publica o
// fechamento no fan-out quando o publisher está configurado.
func TestCloseByAdmin_PublicaSessionClosed(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	scp := &mockSessionClosedPublisher{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "a"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	alloc.On("Release", mock.Anything, "server-0").Return(nil)
	scp.On("PublishSessionClosed", mock.Anything, mock.MatchedBy(func(sess *domain.Session) bool {
		return sess.GameID == "g1" && sess.CloseReason == domain.CloseAdmin
	})).Return(nil)

	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(stubProfiles{}), WithSessionClosedPublisher(scp))
	_, _, err := svc.Create(context.Background(), "m1", "g1", "default", ffaTeams("p1"))
	require.NoError(t, err)

	_, err = svc.CloseByAdmin(context.Background(), "m1")
	require.NoError(t, err)
	scp.AssertExpectations(t)
}

// TestCloseByAdmin_SemSessionClosedPublisher_NaoQuebra: sem a option
// configurada (nil), closeSession funciona normalmente.
func TestCloseByAdmin_SemSessionClosedPublisher_NaoQuebra(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "a"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	alloc.On("Release", mock.Anything, "server-0").Return(nil)

	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(stubProfiles{}))
	_, _, err := svc.Create(context.Background(), "m1", "g1", "default", ffaTeams("p1"))
	require.NoError(t, err)

	_, err = svc.CloseByAdmin(context.Background(), "m1")
	require.NoError(t, err)
}

// TestCloseByAdmin_PublishSessionClosedFalha_NaoImpedeFechamento: falha ao
// publicar não desfaz o fechamento (best-effort, mesma filosofia de
// persistHistory).
func TestCloseByAdmin_PublishSessionClosedFalha_NaoImpedeFechamento(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	scp := &mockSessionClosedPublisher{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "a"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	alloc.On("Release", mock.Anything, "server-0").Return(nil)
	scp.On("PublishSessionClosed", mock.Anything, mock.Anything).Return(assert.AnError)

	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(stubProfiles{}), WithSessionClosedPublisher(scp))
	_, _, err := svc.Create(context.Background(), "m1", "g1", "default", ffaTeams("p1"))
	require.NoError(t, err)

	sess, err := svc.CloseByAdmin(context.Background(), "m1")
	require.NoError(t, err, "publish falhar não pode impedir o fechamento")
	assert.Equal(t, domain.CloseAdmin, sess.CloseReason)
	assert.Equal(t, float64(1), svc.SessionClosedPublishFailures())
}

func TestAllSessions_JogosDistintos_VemJuntas(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "s", Address: "a"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("t", nil)
	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(stubProfiles{}))
	_, _, _ = svc.Create(context.Background(), "m1", "g1", "default", ffaTeams("p1"))
	_, _, _ = svc.Create(context.Background(), "m2", "g2", "default", ffaTeams("p2"))

	active, _ := svc.AllSessions()
	assert.Len(t, active, 2, "board global reúne sessões de jogos diferentes")
}

// --- composição de times (ADR-0023, spec 010) ---

func TestCreate_ComTimes_RegistraTimesEEmiteTokenComTeamID(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	pub := &mockPublisher{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	// cada jogador recebe o token com o team_id do seu lado.
	iss.On("Issue", "p1", "match-1", "10.0.0.1:7777", domain.ConnectClaims{TeamID: "team-1", ModeID: "default"}).Return("t1", nil)
	iss.On("Issue", "p2", "match-1", "10.0.0.1:7777", domain.ConnectClaims{TeamID: "team-2", ModeID: "default"}).Return("t2", nil)

	svc := New(alloc, iss, pub, WithProfileProvider(stubProfiles{}))
	teams := []domain.TeamAssignment{
		{TeamID: "team-1", Players: []string{"p1"}},
		{TeamID: "team-2", Players: []string{"p2"}},
	}
	sess, grants, err := svc.Create(context.Background(), "match-1", "game-1", "default", teams)

	require.NoError(t, err)
	assert.Equal(t, "team-1", sess.TeamByPlayer["p1"])
	assert.Equal(t, "team-2", sess.TeamByPlayer["p2"])
	assert.ElementsMatch(t, []string{"p1", "p2"}, sess.Players)
	require.Len(t, grants, 2)
	iss.AssertExpectations(t)
}

// Composição inconsistente (mesmo jogador em dois times) é rejeitada ANTES de
// alocar server (o allocator nem é chamado).
func TestCreate_JogadorEmDoisTimes_ErrInvalidTeams(t *testing.T) {
	alloc := &mockAllocator{}
	svc := New(alloc, &mockIssuer{}, &mockPublisher{}, WithProfileProvider(stubProfiles{}))
	teams := []domain.TeamAssignment{
		{TeamID: "team-1", Players: []string{"p1"}},
		{TeamID: "team-2", Players: []string{"p1"}},
	}
	_, _, err := svc.Create(context.Background(), "m", "g", "default", teams)

	assert.ErrorIs(t, err, domain.ErrInvalidTeams)
	alloc.AssertNotCalled(t, "Allocate", mock.Anything, mock.Anything, mock.Anything)
}

// O session é dono canônico do team_id: no ReportResult ele SOBRESCREVE o que o
// game server mandou (garbage/vazio) com o time registrado na formação.
func TestReportResult_CarimbaTeamIDDaComposicaoRegistrada(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	pub := &mockPublisher{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("t", nil)
	alloc.On("Release", mock.Anything, "server-0").Return(nil)

	svc := New(alloc, iss, pub, WithProfileProvider(stubProfiles{}))
	teams := []domain.TeamAssignment{
		{TeamID: "red", Players: []string{"p1"}},
		{TeamID: "blue", Players: []string{"p2"}},
	}
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", "default", teams)
	require.NoError(t, err)

	// o server reporta team_id errado/ausente — deve ser ignorado.
	reported := wldOutcome(
		domain.PlayerOutcome{PlayerID: "p1", TeamID: "garbage", Outcome: domain.ResultWin, Score: 1},
		domain.PlayerOutcome{PlayerID: "p2", TeamID: "", Outcome: domain.ResultLoss, Score: 0},
	)
	// o fan-out recebe o team_id canônico (red/blue).
	expected := wldOutcome(
		domain.PlayerOutcome{PlayerID: "p1", TeamID: "red", Outcome: domain.ResultWin, Score: 1},
		domain.PlayerOutcome{PlayerID: "p2", TeamID: "blue", Outcome: domain.ResultLoss, Score: 0},
	)
	pub.On("Publish", mock.Anything, "game-1", "match-1", "default", expected).Return(nil)

	require.NoError(t, svc.ReportResult(context.Background(), "match-1", reported))
	pub.AssertExpectations(t)
}

// FFA: sem times, o carimbo deixa o team_id vazio (sem regressão).
func TestReportResult_FFA_TeamIDVazio(t *testing.T) {
	svc, pub, alloc := reportSetup(t) // cria match-1 FFA com p1/p2
	alloc.On("Release", mock.Anything, "server-0").Return(nil)
	reported := wldOutcome(
		domain.PlayerOutcome{PlayerID: "p1", Outcome: domain.ResultWin, Score: 3},
		domain.PlayerOutcome{PlayerID: "p2", Outcome: domain.ResultLoss, Score: 1},
	)
	pub.On("Publish", mock.Anything, "game-1", "match-1", "default", reported).Return(nil)

	require.NoError(t, svc.ReportResult(context.Background(), "match-1", reported))
	for _, p := range reported.WinLossDraw.Players {
		assert.Empty(t, p.TeamID)
	}
	pub.AssertExpectations(t)
}

// --- outcome placement (battle_royale, ADR-0023/BR2) ---

// placementOutcome monta um MatchOutcome placement com as entradas dadas.
func placementOutcome(entries ...domain.PlacementEntry) domain.MatchOutcome {
	return domain.MatchOutcome{Kind: domain.OutcomePlacement, Placement: &domain.PlacementResult{Entries: entries}}
}

// O caminho feliz do BR2: placement válido → publica (kind=placement), libera o
// server e encerra a sessão (CloseResult) — mesmo fluxo terminal do win_loss_draw.
func TestReportResult_Placement_PublicaLiberaEncerra(t *testing.T) {
	svc, pub, alloc := reportSetupArchetype(t, domain.ArchetypeBattleRoyale) // FFA p1/p2
	outcome := placementOutcome(
		domain.PlacementEntry{PlayerID: "p1", Placement: 1, SurvivedMs: 900000},
		domain.PlacementEntry{PlayerID: "p2", Placement: 2, SurvivedMs: 600000},
	)
	pub.On("Publish", mock.Anything, "game-1", "match-1", mock.Anything, mock.MatchedBy(func(o domain.MatchOutcome) bool {
		return o.Kind == domain.OutcomePlacement && o.Placement != nil && len(o.Placement.Entries) == 2
	})).Return(nil)
	alloc.On("Release", mock.Anything, "server-0").Return(nil)

	require.NoError(t, svc.ReportResult(context.Background(), "match-1", outcome))
	assert.Equal(t, float64(0), svc.ActiveSessions(), "sessão encerrada")
	assert.Equal(t, float64(1), svc.ClosedTotal(domain.CloseResult))
	pub.AssertExpectations(t)
	alloc.AssertExpectations(t)
}

// Validação estrutural: entries vazio, placement < 1 e survivedMs < 0 → ErrInvalidOutcome.
func TestReportResult_Placement_Invalido_ErrInvalidOutcome(t *testing.T) {
	svc, _, _ := reportSetupArchetype(t, domain.ArchetypeBattleRoyale)
	errVazio := svc.ReportResult(context.Background(), "match-1", placementOutcome())
	assert.ErrorIs(t, errVazio, domain.ErrInvalidOutcome, "entries vazio")

	errColocacao := svc.ReportResult(context.Background(), "match-1",
		placementOutcome(domain.PlacementEntry{PlayerID: "p1", Placement: 0}))
	assert.ErrorIs(t, errColocacao, domain.ErrInvalidOutcome, "placement < 1")

	errSurvive := svc.ReportResult(context.Background(), "match-1",
		placementOutcome(domain.PlacementEntry{PlayerID: "p1", Placement: 1, SurvivedMs: -1}))
	assert.ErrorIs(t, errSurvive, domain.ErrInvalidOutcome, "survivedMs < 0")

	assert.Equal(t, float64(1), svc.ActiveSessions(), "outcome inválido não encerra a sessão")
}

// Empate de colocação (co-eliminação, ficha §5): dois placement=1 são aceitos —
// coerência de ranking é do data plane, não do session.
func TestReportResult_Placement_EmpateAceito(t *testing.T) {
	svc, pub, alloc := reportSetupArchetype(t, domain.ArchetypeBattleRoyale)
	outcome := placementOutcome(
		domain.PlacementEntry{PlayerID: "p1", Placement: 1, SurvivedMs: 800000},
		domain.PlacementEntry{PlayerID: "p2", Placement: 1, SurvivedMs: 800000},
	)
	pub.On("Publish", mock.Anything, "game-1", "match-1", mock.Anything, mock.Anything).Return(nil)
	alloc.On("Release", mock.Anything, "server-0").Return(nil)

	require.NoError(t, svc.ReportResult(context.Background(), "match-1", outcome),
		"empate de colocação é aceito (session não recomputa a corrida)")
	pub.AssertExpectations(t)
}

// Carimbo de squad (BR1): a entrada do jogador recebe o squad-K registrado na
// formação, sobrescrevendo o que o server ecoou; solo/FFA fica com teamId vazio.
func TestReportResult_Placement_CarimbaSquadDaFormacao(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	pub := &mockPublisher{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("t", nil)
	alloc.On("Release", mock.Anything, "server-0").Return(nil)

	svc := New(alloc, iss, pub, WithProfileProvider(stubProfiles{archetype: domain.ArchetypeBattleRoyale}))
	// formação BR: p1/p2 no squad-1; p3 sem squad (solo).
	teams := []domain.TeamAssignment{
		{TeamID: "squad-1", Players: []string{"p1", "p2"}},
		{Players: []string{"p3"}},
	}
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", "default", teams)
	require.NoError(t, err)

	// o server ecoa team_id errado — deve ser ignorado e recarimbado.
	reported := placementOutcome(
		domain.PlacementEntry{PlayerID: "p1", TeamID: "garbage", Placement: 1, SurvivedMs: 900000},
		domain.PlacementEntry{PlayerID: "p2", TeamID: "garbage", Placement: 1, SurvivedMs: 900000},
		domain.PlacementEntry{PlayerID: "p3", TeamID: "garbage", Placement: 2, SurvivedMs: 500000},
	)
	var captured domain.MatchOutcome
	pub.On("Publish", mock.Anything, "game-1", "match-1", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { captured = args.Get(4).(domain.MatchOutcome) }).Return(nil)

	require.NoError(t, svc.ReportResult(context.Background(), "match-1", reported))
	require.NotNil(t, captured.Placement)
	byPlayer := map[string]string{}
	for _, e := range captured.Placement.Entries {
		byPlayer[e.PlayerID] = e.TeamID
	}
	assert.Equal(t, "squad-1", byPlayer["p1"])
	assert.Equal(t, "squad-1", byPlayer["p2"])
	assert.Empty(t, byPlayer["p3"], "solo fica com teamId vazio")
	pub.AssertExpectations(t)
}

// placement para matchId inexistente → ErrSessionNotFound (inalterado).
func TestReportResult_Placement_MatchInexistente_ErrSessionNotFound(t *testing.T) {
	svc := New(&mockAllocator{}, &mockIssuer{}, &mockPublisher{}, WithProfileProvider(stubProfiles{archetype: domain.ArchetypeBattleRoyale}))
	err := svc.ReportResult(context.Background(), "nao-existe",
		placementOutcome(domain.PlacementEntry{PlayerID: "p1", Placement: 1}))
	assert.ErrorIs(t, err, domain.ErrSessionNotFound)
}

// --- coop_mission (C2): resultado coletivo ---

func collectiveOutcome(result string, players ...domain.CollectivePlayer) domain.MatchOutcome {
	return domain.MatchOutcome{Kind: domain.OutcomeCollective, Collective: &domain.CollectiveResult{Outcome: result, Players: players}}
}

// Caminho feliz do C2: collective válido (SUCCESS) → publica (kind=collective),
// libera o server e encerra a sessão — mesmo fluxo terminal do win_loss_draw.
func TestReportResult_Collective_PublicaLiberaEncerra(t *testing.T) {
	svc, pub, alloc := reportSetupArchetype(t, domain.ArchetypeCoopMission) // FFA p1/p2
	outcome := collectiveOutcome(domain.MissionSuccess,
		domain.CollectivePlayer{PlayerID: "p1", Stats: map[string]int64{"kills": 12}},
		domain.CollectivePlayer{PlayerID: "p2", Stats: map[string]int64{"revives": 3}},
	)
	pub.On("Publish", mock.Anything, "game-1", "match-1", mock.Anything, mock.MatchedBy(func(o domain.MatchOutcome) bool {
		return o.Kind == domain.OutcomeCollective && o.Collective != nil &&
			o.Collective.Outcome == domain.MissionSuccess && len(o.Collective.Players) == 2
	})).Return(nil)
	alloc.On("Release", mock.Anything, "server-0").Return(nil)

	require.NoError(t, svc.ReportResult(context.Background(), "match-1", outcome))
	assert.Equal(t, float64(0), svc.ActiveSessions(), "sessão encerrada")
	assert.Equal(t, float64(1), svc.ClosedTotal(domain.CloseResult))
	pub.AssertExpectations(t)
	alloc.AssertExpectations(t)
}

// FAILURE também é aceito (é um desfecho válido da sessão).
func TestReportResult_Collective_Failure_Aceito(t *testing.T) {
	svc, pub, alloc := reportSetupArchetype(t, domain.ArchetypeCoopMission)
	outcome := collectiveOutcome(domain.MissionFailure,
		domain.CollectivePlayer{PlayerID: "p1"},
		domain.CollectivePlayer{PlayerID: "p2"},
	)
	pub.On("Publish", mock.Anything, "game-1", "match-1", mock.Anything, mock.Anything).Return(nil)
	alloc.On("Release", mock.Anything, "server-0").Return(nil)

	require.NoError(t, svc.ReportResult(context.Background(), "match-1", outcome))
	pub.AssertExpectations(t)
}

// Validação estrutural: players vazio e outcome fora do vocabulário → ErrInvalidOutcome.
func TestReportResult_Collective_Invalido_ErrInvalidOutcome(t *testing.T) {
	svc, _, _ := reportSetupArchetype(t, domain.ArchetypeCoopMission)
	errVazio := svc.ReportResult(context.Background(), "match-1", collectiveOutcome(domain.MissionSuccess))
	assert.ErrorIs(t, errVazio, domain.ErrInvalidOutcome, "players vazio")

	errOutcome := svc.ReportResult(context.Background(), "match-1",
		collectiveOutcome("MISSION_ABORTED", domain.CollectivePlayer{PlayerID: "p1"}))
	assert.ErrorIs(t, errOutcome, domain.ErrInvalidOutcome, "outcome fora do vocabulário")

	assert.Equal(t, float64(1), svc.ActiveSessions(), "outcome inválido não encerra a sessão")
}

// Carimbo de time (C1): a entrada do jogador recebe o team-1 registrado na
// formação, sobrescrevendo o que o server ecoou.
func TestReportResult_Collective_CarimbaTimeDaFormacao(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	pub := &mockPublisher{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("t", nil)
	alloc.On("Release", mock.Anything, "server-0").Return(nil)

	svc := New(alloc, iss, pub, WithProfileProvider(stubProfiles{archetype: domain.ArchetypeCoopMission}))
	// formação co-op: p1/p2/p3 num único time team-1.
	teams := []domain.TeamAssignment{{TeamID: "team-1", Players: []string{"p1", "p2", "p3"}}}
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", "default", teams)
	require.NoError(t, err)

	reported := collectiveOutcome(domain.MissionSuccess,
		domain.CollectivePlayer{PlayerID: "p1", TeamID: "garbage"},
		domain.CollectivePlayer{PlayerID: "p2", TeamID: "garbage"},
		domain.CollectivePlayer{PlayerID: "p3", TeamID: "garbage"},
	)
	var captured domain.MatchOutcome
	pub.On("Publish", mock.Anything, "game-1", "match-1", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { captured = args.Get(4).(domain.MatchOutcome) }).Return(nil)

	require.NoError(t, svc.ReportResult(context.Background(), "match-1", reported))
	require.NotNil(t, captured.Collective)
	for _, p := range captured.Collective.Players {
		assert.Equal(t, "team-1", p.TeamID, "recarimbado a partir da formação")
	}
	pub.AssertExpectations(t)
}

// collective para matchId inexistente → ErrSessionNotFound (inalterado).
func TestReportResult_Collective_MatchInexistente_ErrSessionNotFound(t *testing.T) {
	svc := New(&mockAllocator{}, &mockIssuer{}, &mockPublisher{}, WithProfileProvider(stubProfiles{archetype: domain.ArchetypeCoopMission}))
	err := svc.ReportResult(context.Background(), "nao-existe",
		collectiveOutcome(domain.MissionSuccess, domain.CollectivePlayer{PlayerID: "p1"}))
	assert.ErrorIs(t, err, domain.ErrSessionNotFound)
}

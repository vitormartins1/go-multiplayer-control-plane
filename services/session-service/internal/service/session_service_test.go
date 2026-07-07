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

	"github.com/vitormartins1/go-game-backend/services/session-service/internal/domain"
)

// discardLog é o logger silencioso para os testes do reconcile/sweeper.
func discardLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

type mockAllocator struct{ mock.Mock }

func (m *mockAllocator) Allocate(ctx context.Context, matchID, gameID string) (domain.Server, error) {
	args := m.Called(ctx, matchID, gameID)
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

func (m *mockProfile) WorldProfile(ctx context.Context, gameID string) (domain.WorldProfile, error) {
	args := m.Called(ctx, gameID)
	return args.Get(0).(domain.WorldProfile), args.Error(1)
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

func (m *mockPublisher) Publish(ctx context.Context, gameID, matchID string, outcome domain.MatchOutcome) error {
	return m.Called(ctx, gameID, matchID, outcome).Error(0)
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
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", "p1", "match-1", "10.0.0.1:7777", domain.ConnectClaims{}).Return("token-p1", nil)
	iss.On("Issue", "p2", "match-1", "10.0.0.1:7777", domain.ConnectClaims{}).Return("token-p2", nil)

	svc := New(alloc, iss, pub)
	sess, grants, err := svc.Create(context.Background(), "match-1", "game-1", ffaTeams("p1", "p2"))

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
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{}, domain.ErrNoServer)

	svc := New(alloc, &mockIssuer{}, &mockPublisher{})
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", ffaTeams("p1"))

	assert.ErrorIs(t, err, domain.ErrNoServer)
	assert.Equal(t, float64(0), svc.ActiveSessions())
}

func TestReportResult_PublicaLiberaEncerra(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	pub := &mockPublisher{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	svc := New(alloc, iss, pub)
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", ffaTeams("p1"))
	require.NoError(t, err)

	outcome := wldOutcome(domain.PlayerOutcome{PlayerID: "p1", Outcome: domain.ResultWin, Score: 10})
	pub.On("Publish", mock.Anything, "game-1", "match-1", outcome).Return(nil) // recebe o game_id e o match_id da sessão
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
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	svc := New(alloc, iss, pub)
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", ffaTeams("p1"))
	require.NoError(t, err)

	pub.On("Publish", mock.Anything, "game-1", "match-1", mock.Anything).Return(errors.New("sns fora"))

	err = svc.ReportResult(context.Background(), "match-1", validOutcome())
	require.Error(t, err)
	assert.Equal(t, float64(1), svc.ActiveSessions(), "sessão continua ativa")
	alloc.AssertNotCalled(t, "Release", mock.Anything, mock.Anything)
	pub.AssertExpectations(t)
}

func TestReportResult_MatchInexistente_ErrSessionNotFound(t *testing.T) {
	svc := New(&mockAllocator{}, &mockIssuer{}, &mockPublisher{})
	err := svc.ReportResult(context.Background(), "nao-existe", validOutcome())
	assert.ErrorIs(t, err, domain.ErrSessionNotFound)
}

// reportSetup cria um service com uma sessão aberta (match-1/game-1) pronta para
// o ReportResult (server-0, players p1/p2).
func reportSetup(t *testing.T) (*Service, *mockPublisher, *mockAllocator) {
	t.Helper()
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	pub := &mockPublisher{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	svc := New(alloc, iss, pub)
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", ffaTeams("p1", "p2"))
	require.NoError(t, err)
	return svc, pub, alloc
}

func TestReportResult_Empate_Aceito(t *testing.T) {
	svc, pub, alloc := reportSetup(t)
	outcome := wldOutcome(
		domain.PlayerOutcome{PlayerID: "p1", Outcome: domain.ResultDraw, Score: 5},
		domain.PlayerOutcome{PlayerID: "p2", Outcome: domain.ResultDraw, Score: 5},
	)
	pub.On("Publish", mock.Anything, "game-1", "match-1", outcome).Return(nil)
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
	pub.On("Publish", mock.Anything, "game-1", "match-1", outcome).Return(nil)
	alloc.On("Release", mock.Anything, "server-0").Return(nil)

	require.NoError(t, svc.ReportResult(context.Background(), "match-1", outcome))
	pub.AssertExpectations(t)
}

func TestReportResult_KindNaoSuportado_ErrUnsupportedOutcome(t *testing.T) {
	svc, _, _ := reportSetup(t)
	err := svc.ReportResult(context.Background(), "match-1", domain.MatchOutcome{Kind: "placement"})
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
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)

	// clock injetável: cria a sessão em t0 e varre em t0+31m com TTL de 30m.
	now := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	svc := New(alloc, iss, &mockPublisher{}, WithClock(func() time.Time { return now }))
	sess, _, err := svc.Create(context.Background(), "match-1", "game-1", ffaTeams("p1"))
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
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)

	now := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	svc := New(alloc, iss, &mockPublisher{}, WithClock(func() time.Time { return now }))
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", ffaTeams("p1"))
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
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)

	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	svc := New(alloc, iss, &mockPublisher{},
		WithClock(func() time.Time { return now }),
		WithServerRegistry(reg), WithOrphanGrace(time.Minute))
	sess, _, err := svc.Create(context.Background(), "match-1", "game-1", ffaTeams("p1"))
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
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)

	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	svc := New(alloc, iss, &mockPublisher{},
		WithClock(func() time.Time { return now }),
		WithServerRegistry(reg), WithOrphanGrace(time.Minute))
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", ffaTeams("p1"))
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
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)

	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	svc := New(alloc, iss, &mockPublisher{},
		WithClock(func() time.Time { return now }),
		WithServerRegistry(reg), WithOrphanGrace(time.Minute))
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", ffaTeams("p1"))
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
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)

	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	svc := New(alloc, iss, &mockPublisher{},
		WithClock(func() time.Time { return now }),
		WithServerRegistry(reg), WithOrphanGrace(time.Minute))
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", ffaTeams("p1"))
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
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)

	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	svc := New(alloc, iss, &mockPublisher{}, WithClock(func() time.Time { return now })) // sem WithServerRegistry
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", ffaTeams("p1"))
	require.NoError(t, err)

	now = now.Add(5 * time.Minute)
	assert.Equal(t, 0, svc.reconcile(context.Background(), time.Minute, discardLog()), "sem registry: reconcile é no-op")
	assert.Equal(t, float64(1), svc.ActiveSessions())
}

func TestReportResult_MarcaCloseReasonResult(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	pub := &mockPublisher{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	svc := New(alloc, iss, pub)
	sess, _, err := svc.Create(context.Background(), "match-1", "game-1", ffaTeams("p1"))
	require.NoError(t, err)

	pub.On("Publish", mock.Anything, "game-1", mock.Anything, mock.Anything).Return(nil)
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
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	alloc.On("Release", mock.Anything, mock.Anything).Return(nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	pub := &mockPublisher{}
	pub.On("Publish", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	return New(alloc, iss, pub, opts...)
}

func TestSessionsByGame_AtivasEFechadasFiltradasEOrdenadas(t *testing.T) {
	now := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	svc := newSvcComSessoes(t, WithClock(func() time.Time { return now }))
	ctx := context.Background()

	_, _, err := svc.Create(ctx, "m-old", "g1", ffaTeams("p1"))
	require.NoError(t, err)
	now = now.Add(time.Minute)
	_, _, err = svc.Create(ctx, "m-new", "g1", ffaTeams("p2"))
	require.NoError(t, err)
	_, _, err = svc.Create(ctx, "m-outro-jogo", "g2", ffaTeams("p3"))
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
		_, _, err := svc.Create(ctx, m, "g1", ffaTeams("p"))
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
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", "p1", "match-1", "10.0.0.1:7777", domain.ConnectClaims{}).Return("", assert.AnError)
	alloc.On("Release", mock.Anything, "server-0").Return(nil)

	svc := New(alloc, iss, &mockPublisher{})
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", ffaTeams("p1"))

	require.Error(t, err)
	assert.Equal(t, float64(0), svc.ActiveSessions())
	alloc.AssertExpectations(t) // garante o Release de rollback
}

func TestCloseByAdmin_Ativa_FechaELiberaServer(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	alloc.On("Release", mock.Anything, "server-0").Return(nil)
	svc := New(alloc, iss, &mockPublisher{})
	_, _, err := svc.Create(context.Background(), "m1", "g1", ffaTeams("p1"))
	require.NoError(t, err)

	sess, err := svc.CloseByAdmin(context.Background(), "m1")
	require.NoError(t, err)
	assert.Equal(t, domain.CloseAdmin, sess.CloseReason)
	assert.Equal(t, float64(0), svc.ActiveSessions(), "sai das ativas")
	alloc.AssertExpectations(t)
}

func TestCloseByAdmin_Inexistente_ErrSessionNotFound(t *testing.T) {
	svc := New(&mockAllocator{}, &mockIssuer{}, &mockPublisher{})
	_, err := svc.CloseByAdmin(context.Background(), "nao-existe")
	assert.ErrorIs(t, err, domain.ErrSessionNotFound)
}

func TestAllSessions_JogosDistintos_VemJuntas(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "s", Address: "a"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("t", nil)
	svc := New(alloc, iss, &mockPublisher{})
	_, _, _ = svc.Create(context.Background(), "m1", "g1", ffaTeams("p1"))
	_, _, _ = svc.Create(context.Background(), "m2", "g2", ffaTeams("p2"))

	active, _ := svc.AllSessions()
	assert.Len(t, active, 2, "board global reúne sessões de jogos diferentes")
}

// --- composição de times (ADR-0023, spec 010) ---

func TestCreate_ComTimes_RegistraTimesEEmiteTokenComTeamID(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	pub := &mockPublisher{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	// cada jogador recebe o token com o team_id do seu lado.
	iss.On("Issue", "p1", "match-1", "10.0.0.1:7777", domain.ConnectClaims{TeamID: "team-1"}).Return("t1", nil)
	iss.On("Issue", "p2", "match-1", "10.0.0.1:7777", domain.ConnectClaims{TeamID: "team-2"}).Return("t2", nil)

	svc := New(alloc, iss, pub)
	teams := []domain.TeamAssignment{
		{TeamID: "team-1", Players: []string{"p1"}},
		{TeamID: "team-2", Players: []string{"p2"}},
	}
	sess, grants, err := svc.Create(context.Background(), "match-1", "game-1", teams)

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
	svc := New(alloc, &mockIssuer{}, &mockPublisher{})
	teams := []domain.TeamAssignment{
		{TeamID: "team-1", Players: []string{"p1"}},
		{TeamID: "team-2", Players: []string{"p1"}},
	}
	_, _, err := svc.Create(context.Background(), "m", "g", teams)

	assert.ErrorIs(t, err, domain.ErrInvalidTeams)
	alloc.AssertNotCalled(t, "Allocate", mock.Anything, mock.Anything, mock.Anything)
}

// O session é dono canônico do team_id: no ReportResult ele SOBRESCREVE o que o
// game server mandou (garbage/vazio) com o time registrado na formação.
func TestReportResult_CarimbaTeamIDDaComposicaoRegistrada(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	pub := &mockPublisher{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("t", nil)
	alloc.On("Release", mock.Anything, "server-0").Return(nil)

	svc := New(alloc, iss, pub)
	teams := []domain.TeamAssignment{
		{TeamID: "red", Players: []string{"p1"}},
		{TeamID: "blue", Players: []string{"p2"}},
	}
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", teams)
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
	pub.On("Publish", mock.Anything, "game-1", "match-1", expected).Return(nil)

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
	pub.On("Publish", mock.Anything, "game-1", "match-1", reported).Return(nil)

	require.NoError(t, svc.ReportResult(context.Background(), "match-1", reported))
	for _, p := range reported.WinLossDraw.Players {
		assert.Empty(t, p.TeamID)
	}
	pub.AssertExpectations(t)
}

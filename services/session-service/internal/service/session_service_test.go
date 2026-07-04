package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-game-backend/services/session-service/internal/domain"
)

type mockAllocator struct{ mock.Mock }

func (m *mockAllocator) Allocate(ctx context.Context, matchID, gameID string) (domain.Server, error) {
	args := m.Called(ctx, matchID, gameID)
	return args.Get(0).(domain.Server), args.Error(1)
}

func (m *mockAllocator) Release(ctx context.Context, serverID string) error {
	return m.Called(ctx, serverID).Error(0)
}

type mockIssuer struct{ mock.Mock }

func (m *mockIssuer) Issue(playerID, matchID, serverAddr string) (string, error) {
	args := m.Called(playerID, matchID, serverAddr)
	return args.String(0), args.Error(1)
}

func (m *mockIssuer) JWKS(ctx context.Context) ([]byte, error) {
	args := m.Called(ctx)
	return args.Get(0).([]byte), args.Error(1)
}

type mockPublisher struct{ mock.Mock }

func (m *mockPublisher) Publish(ctx context.Context, gameID, matchID string, results []domain.PlayerResult) error {
	return m.Called(ctx, gameID, matchID, results).Error(0)
}

func TestCreate_NPlayers_GeraNGrantsComServerAddress(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	pub := &mockPublisher{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", "p1", "match-1", "10.0.0.1:7777").Return("token-p1", nil)
	iss.On("Issue", "p2", "match-1", "10.0.0.1:7777").Return("token-p2", nil)

	svc := New(alloc, iss, pub)
	sess, grants, err := svc.Create(context.Background(), "match-1", "game-1", []string{"p1", "p2"})

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
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", []string{"p1"})

	assert.ErrorIs(t, err, domain.ErrNoServer)
	assert.Equal(t, float64(0), svc.ActiveSessions())
}

func TestReportResult_PublicaLiberaEncerra(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	pub := &mockPublisher{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	svc := New(alloc, iss, pub)
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", []string{"p1"})
	require.NoError(t, err)

	results := []domain.PlayerResult{{PlayerID: "p1", Score: 10, Won: true}}
	pub.On("Publish", mock.Anything, "game-1", "match-1", results).Return(nil) // recebe o game_id e o match_id da sessão
	alloc.On("Release", mock.Anything, "server-0").Return(nil)

	require.NoError(t, svc.ReportResult(context.Background(), "match-1", results))
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
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	svc := New(alloc, iss, pub)
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", []string{"p1"})
	require.NoError(t, err)

	pub.On("Publish", mock.Anything, "game-1", "match-1", mock.Anything).Return(errors.New("sns fora"))

	err = svc.ReportResult(context.Background(), "match-1", nil)
	require.Error(t, err)
	assert.Equal(t, float64(1), svc.ActiveSessions(), "sessão continua ativa")
	alloc.AssertNotCalled(t, "Release", mock.Anything, mock.Anything)
	pub.AssertExpectations(t)
}

func TestReportResult_MatchInexistente_ErrSessionNotFound(t *testing.T) {
	svc := New(&mockAllocator{}, &mockIssuer{}, &mockPublisher{})
	err := svc.ReportResult(context.Background(), "nao-existe", nil)
	assert.ErrorIs(t, err, domain.ErrSessionNotFound)
}

func TestSweep_SessaoExpirada_FechaComTTLELiberaServer(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything).Return("token", nil)

	// clock injetável: cria a sessão em t0 e varre em t0+31m com TTL de 30m.
	now := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	svc := New(alloc, iss, &mockPublisher{}, WithClock(func() time.Time { return now }))
	sess, _, err := svc.Create(context.Background(), "match-1", "game-1", []string{"p1"})
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
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything).Return("token", nil)

	now := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	svc := New(alloc, iss, &mockPublisher{}, WithClock(func() time.Time { return now }))
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", []string{"p1"})
	require.NoError(t, err)

	now = now.Add(10 * time.Minute)
	assert.Equal(t, 0, svc.Sweep(context.Background(), 30*time.Minute))
	assert.Equal(t, float64(1), svc.ActiveSessions(), "sessão dentro do TTL segue ativa")
	alloc.AssertNotCalled(t, "Release", mock.Anything, mock.Anything)
}

func TestReportResult_MarcaCloseReasonResult(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	pub := &mockPublisher{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	svc := New(alloc, iss, pub)
	sess, _, err := svc.Create(context.Background(), "match-1", "game-1", []string{"p1"})
	require.NoError(t, err)

	pub.On("Publish", mock.Anything, "game-1", mock.Anything, mock.Anything).Return(nil)
	alloc.On("Release", mock.Anything, "server-0").Return(nil)
	require.NoError(t, svc.ReportResult(context.Background(), "match-1", nil))

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
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	pub := &mockPublisher{}
	pub.On("Publish", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	return New(alloc, iss, pub, opts...)
}

func TestSessionsByGame_AtivasEFechadasFiltradasEOrdenadas(t *testing.T) {
	now := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	svc := newSvcComSessoes(t, WithClock(func() time.Time { return now }))
	ctx := context.Background()

	_, _, err := svc.Create(ctx, "m-old", "g1", []string{"p1"})
	require.NoError(t, err)
	now = now.Add(time.Minute)
	_, _, err = svc.Create(ctx, "m-new", "g1", []string{"p2"})
	require.NoError(t, err)
	_, _, err = svc.Create(ctx, "m-outro-jogo", "g2", []string{"p3"})
	require.NoError(t, err)

	now = now.Add(time.Minute)
	require.NoError(t, svc.ReportResult(ctx, "m-old", nil))

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
		_, _, err := svc.Create(ctx, m, "g1", []string{"p"})
		require.NoError(t, err)
		require.NoError(t, svc.ReportResult(ctx, m, nil))
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
	iss.On("Issue", "p1", "match-1", "10.0.0.1:7777").Return("", assert.AnError)
	alloc.On("Release", mock.Anything, "server-0").Return(nil)

	svc := New(alloc, iss, &mockPublisher{})
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", []string{"p1"})

	require.Error(t, err)
	assert.Equal(t, float64(0), svc.ActiveSessions())
	alloc.AssertExpectations(t) // garante o Release de rollback
}

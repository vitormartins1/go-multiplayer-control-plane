package service

import (
	"context"
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

func (m *mockPublisher) Publish(ctx context.Context, gameID string, results []domain.PlayerResult) error {
	return m.Called(ctx, gameID, results).Error(0)
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
	pub.On("Publish", mock.Anything, "game-1", results).Return(nil) // recebe o game_id da sessão
	alloc.On("Release", mock.Anything, "server-0").Return(nil)

	require.NoError(t, svc.ReportResult(context.Background(), "match-1", results))
	assert.Equal(t, float64(0), svc.ActiveSessions(), "sessão encerrada")
	pub.AssertExpectations(t)
	alloc.AssertExpectations(t)
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

	pub.On("Publish", mock.Anything, "game-1", mock.Anything).Return(nil)
	alloc.On("Release", mock.Anything, "server-0").Return(nil)
	require.NoError(t, svc.ReportResult(context.Background(), "match-1", nil))

	assert.Equal(t, domain.SessionClosed, sess.State)
	assert.Equal(t, domain.CloseResult, sess.CloseReason)
	assert.Equal(t, float64(1), svc.ClosedTotal(domain.CloseResult))
	assert.Equal(t, float64(0), svc.ClosedTotal(domain.CloseTTL))
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

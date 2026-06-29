package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/c130n/game-backend/services/session-service/internal/domain"
)

type mockAllocator struct{ mock.Mock }

func (m *mockAllocator) Allocate(ctx context.Context) (domain.Server, error) {
	args := m.Called(ctx)
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

func (m *mockPublisher) Publish(ctx context.Context, matchID string, results []domain.PlayerResult) error {
	return m.Called(ctx, matchID, results).Error(0)
}

func TestCreate_NPlayers_GeraNGrantsComServerAddress(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	pub := &mockPublisher{}
	alloc.On("Allocate", mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", "p1", "match-1", "10.0.0.1:7777").Return("token-p1", nil)
	iss.On("Issue", "p2", "match-1", "10.0.0.1:7777").Return("token-p2", nil)

	svc := New(alloc, iss, pub)
	sess, grants, err := svc.Create(context.Background(), "match-1", []string{"p1", "p2"})

	require.NoError(t, err)
	assert.Equal(t, "10.0.0.1:7777", sess.ServerAddress)
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
	alloc.On("Allocate", mock.Anything).Return(domain.Server{}, domain.ErrNoServer)

	svc := New(alloc, &mockIssuer{}, &mockPublisher{})
	_, _, err := svc.Create(context.Background(), "match-1", []string{"p1"})

	assert.ErrorIs(t, err, domain.ErrNoServer)
	assert.Equal(t, float64(0), svc.ActiveSessions())
}

func TestReportResult_PublicaLiberaEncerra(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	pub := &mockPublisher{}
	alloc.On("Allocate", mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	svc := New(alloc, iss, pub)
	_, _, err := svc.Create(context.Background(), "match-1", []string{"p1"})
	require.NoError(t, err)

	results := []domain.PlayerResult{{PlayerID: "p1", Score: 10, Won: true}}
	pub.On("Publish", mock.Anything, "match-1", results).Return(nil)
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

func TestCreate_FalhaAoEmitirToken_LiberaServer(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	alloc.On("Allocate", mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", "p1", "match-1", "10.0.0.1:7777").Return("", assert.AnError)
	alloc.On("Release", mock.Anything, "server-0").Return(nil)

	svc := New(alloc, iss, &mockPublisher{})
	_, _, err := svc.Create(context.Background(), "match-1", []string{"p1"})

	require.Error(t, err)
	assert.Equal(t, float64(0), svc.ActiveSessions())
	alloc.AssertExpectations(t) // garante o Release de rollback
}

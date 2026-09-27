package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/domain"
)

// spec 033 — reenvio do ReportResult depois de aceito.

func resultRetrySetup(t *testing.T, opts ...Option) (*Service, *mockAllocator, *mockPublisher) {
	t.Helper()
	alloc, iss, pub := &mockAllocator{}, &mockIssuer{}, &mockPublisher{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	svc := New(alloc, iss, pub, append([]Option{WithProfileProvider(stubProfiles{})}, opts...)...)
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", "default", ffaTeams("p1"))
	require.NoError(t, err)
	return svc, alloc, pub
}

func TestReportResult_ReenvioDepoisDeAceito_SucessoSemRepublicar(t *testing.T) {
	svc, alloc, pub := resultRetrySetup(t)
	outcome := wldOutcome(domain.PlayerOutcome{PlayerID: "p1", Outcome: domain.ResultWin})
	pub.On("Publish", mock.Anything, "game-1", "match-1", "default", outcome).Return(nil).Once()
	// o release falha DEPOIS do publish: o server recebe erro e reenvia
	alloc.On("Release", mock.Anything, "server-0").Return(errors.New("k8s fora"))

	require.Error(t, svc.ReportResult(context.Background(), "match-1", outcome))
	require.NoError(t, svc.ReportResult(context.Background(), "match-1", outcome), "reenvio: já aceito")
	pub.AssertNumberOfCalls(t, "Publish", 1)
}

func TestReportResult_MatchFechadoPorTTL_ContinuaNotFound(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	svc, alloc, _ := resultRetrySetup(t, WithClock(func() time.Time { return now }))
	alloc.On("Release", mock.Anything, "server-0").Return(nil)
	now = now.Add(time.Hour)
	require.Equal(t, 1, svc.Sweep(context.Background(), 30*time.Minute))

	err := svc.ReportResult(context.Background(), "match-1", wldOutcome(domain.PlayerOutcome{PlayerID: "p1", Outcome: domain.ResultWin}))
	assert.ErrorIs(t, err, domain.ErrSessionNotFound, "o resultado nunca foi aceito: o server precisa saber")
}

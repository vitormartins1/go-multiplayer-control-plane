package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/domain"
)

// spec 032 — heartbeat do mundo.

// heartbeatSetup abre um mundo dedicado com relógio controlado.
func heartbeatSetup(t *testing.T, now *time.Time) (*Service, *mockAllocator, string) {
	t.Helper()
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	prof := &mockProfile{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	alloc.On("Release", mock.Anything, "server-0").Return(nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("tok", nil)
	prof.On("ModeProfile", mock.Anything, "game-1", mock.Anything).Return(coopInvaderProfile(), nil)
	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(prof),
		WithClock(func() time.Time { return *now }), WithWorldHeartbeatTTL(90*time.Second))
	sess, _, _, err := svc.OpenWorld(context.Background(), "host-1", "game-1", "default", domain.WorldAttributes{})
	require.NoError(t, err)
	return svc, alloc, sess.ID
}

func TestHeartbeat_DevolveUmTercoDoTTL(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	svc, _, id := heartbeatSetup(t, &now)

	interval, err := svc.Heartbeat(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, interval)
	assert.Equal(t, float64(1), svc.HeartbeatsTotal())
}

func TestHeartbeat_MundoInexistente_ErrWorldNotFound(t *testing.T) {
	now := time.Now()
	svc, _, _ := heartbeatSetup(t, &now)
	_, err := svc.Heartbeat(context.Background(), "nao-existe")
	assert.ErrorIs(t, err, domain.ErrWorldNotFound)
}

// Mundo com heartbeat em dia sobrevive à idade: 2 h aberto, SESSION_TTL = 30 min.
func TestSweep_MundoComHeartbeatEmDia_NaoFechaPelaIdade(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	svc, _, id := heartbeatSetup(t, &now)

	for range 240 { // 2 h, um heartbeat a cada 30 s
		now = now.Add(30 * time.Second)
		_, err := svc.Heartbeat(context.Background(), id)
		require.NoError(t, err)
	}
	assert.Equal(t, 0, svc.Sweep(context.Background(), 30*time.Minute))
	assert.Equal(t, float64(1), svc.ActiveSessions())
}

// Host caiu: o heartbeat venceu → fecha como abandoned, mesmo com o mundo novo.
func TestSweep_HeartbeatVencido_FechaComoAbandoned(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	svc, alloc, id := heartbeatSetup(t, &now)
	_, err := svc.Heartbeat(context.Background(), id)
	require.NoError(t, err)

	now = now.Add(91 * time.Second)
	assert.Equal(t, 1, svc.Sweep(context.Background(), 30*time.Minute))
	assert.Equal(t, float64(1), svc.ClosedTotal(domain.CloseAbandoned))
	alloc.AssertCalled(t, "Release", mock.Anything, "server-0")
}

// Compatibilidade: jogo que não manda heartbeat segue a regra de idade.
func TestSweep_MundoSemHeartbeat_SegueRegraDeIdade(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	svc, _, _ := heartbeatSetup(t, &now)

	now = now.Add(10 * time.Minute) // bem além do heartbeat TTL, dentro do SESSION_TTL
	assert.Equal(t, 0, svc.Sweep(context.Background(), 30*time.Minute))

	now = now.Add(21 * time.Minute)
	assert.Equal(t, 1, svc.Sweep(context.Background(), 30*time.Minute))
	assert.Equal(t, float64(1), svc.ClosedTotal(domain.CloseTTL))
}

func TestWithWorldHeartbeatTTL_ZeroMantemPadrao(t *testing.T) {
	svc := New(&mockAllocator{}, &mockIssuer{}, &mockPublisher{}, WithWorldHeartbeatTTL(0))
	assert.Equal(t, DefaultWorldHeartbeatTTL, svc.heartbeatTTL)
}

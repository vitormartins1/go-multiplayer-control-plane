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

// spec 031 — topologia p2p no host_world_dropin.

func p2pProfile() domain.ProfileSnapshot {
	p := coopInvaderProfile()
	p.Topology = domain.TopologyP2P
	return p
}

// p2pSetup monta um service cujo modo é p2p. O allocator não tem expectativa:
// qualquer Allocate/Release faz o teste falhar (mock sem On).
func p2pSetup(t *testing.T, opts ...Option) (*Service, *mockAllocator, *mockIssuer) {
	t.Helper()
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	prof := &mockProfile{}
	prof.On("ModeProfile", mock.Anything, "game-1", mock.Anything).Return(p2pProfile(), nil)
	svc := New(alloc, iss, &mockPublisher{}, append([]Option{WithProfileProvider(prof)}, opts...)...)
	return svc, alloc, iss
}

func TestOpenWorld_P2P_NaoAlocaEGuardaHostConnection(t *testing.T) {
	svc, alloc, iss := p2pSetup(t)
	iss.On("Issue", "host-1", mock.Anything, "steam:765", mock.Anything).Return("tok-host", nil)

	sess, grant, worldToken, err := svc.OpenWorld(context.Background(), "host-1", "game-1", "default",
		domain.WorldAttributes{HostConnection: "steam:765"})
	require.NoError(t, err)

	alloc.AssertNotCalled(t, "Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	iss.AssertExpectations(t) // claim server = id de conexão do host
	assert.Equal(t, "tok-host", grant.Token)
	assert.Empty(t, sess.ServerID)
	assert.Empty(t, sess.ServerAddress)
	assert.Equal(t, "steam:765", sess.HostConnection)
	assert.NotEmpty(t, worldToken)
	assert.NotContains(t, sess.WorldTokenHash, worldToken, "o session guarda só o hash")

	got, ok := svc.ResolveWorldToken(worldToken)
	assert.True(t, ok)
	assert.Equal(t, sess.ID, got)
}

func TestOpenWorld_P2P_SemHostConnection_ErrHostConnectionRequired(t *testing.T) {
	svc, _, _ := p2pSetup(t)

	_, _, _, err := svc.OpenWorld(context.Background(), "host-1", "game-1", "default", domain.WorldAttributes{})
	assert.ErrorIs(t, err, domain.ErrHostConnectionRequired)
	assert.Equal(t, float64(0), svc.ActiveSessions())
}

func TestOpenWorld_Dedicado_SemWorldToken(t *testing.T) {
	svc, _, _, sessionID := openWorldSetup(t)
	active, _ := svc.AllSessions()
	require.Len(t, active, 1)
	assert.Empty(t, active[0].WorldTokenHash)
	_, ok := svc.ResolveWorldToken("")
	assert.False(t, ok)
	assert.NotEmpty(t, sessionID)
}

func TestJoinWorld_P2P_DevolveHostConnectionNoTokenENoEndpoint(t *testing.T) {
	svc, _, iss := p2pSetup(t)
	iss.On("Issue", mock.Anything, mock.Anything, "steam:765", mock.Anything).Return("tok", nil)
	sess, _, _, err := svc.OpenWorld(context.Background(), "host-1", "game-1", "default",
		domain.WorldAttributes{HostConnection: "steam:765"})
	require.NoError(t, err)

	_, endpoint, err := svc.JoinWorld(context.Background(), sess.ID, "invasor", domain.RoleInvader)
	require.NoError(t, err)
	assert.Equal(t, domain.WorldEndpoint{HostConnection: "steam:765"}, endpoint)
	iss.AssertNumberOfCalls(t, "Issue", 2) // host + invasor, ambos com claim server = host
}

func TestCloseWorld_P2P_SemReleaseEInvalidaOWorldToken(t *testing.T) {
	svc, alloc, iss := p2pSetup(t)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("tok", nil)
	sess, _, worldToken, err := svc.OpenWorld(context.Background(), "host-1", "game-1", "default",
		domain.WorldAttributes{HostConnection: "steam:765"})
	require.NoError(t, err)

	require.NoError(t, svc.CloseWorld(context.Background(), sess.ID))
	alloc.AssertNotCalled(t, "Release", mock.Anything, mock.Anything)
	_, ok := svc.ResolveWorldToken(worldToken)
	assert.False(t, ok, "token de mundo fechado deixa de valer")
}

// Sem GameServer, o mundo p2p nunca aparece na fonte de verdade do Agones —
// o reconcile não pode tratá-lo como órfão.
func TestReconcile_P2P_MundoNaoViraOrfao(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	reg := &mockRegistry{}
	svc, _, iss := p2pSetup(t, WithClock(func() time.Time { return now }), WithServerRegistry(reg))
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("tok", nil)
	_, _, _, err := svc.OpenWorld(context.Background(), "host-1", "game-1", "default",
		domain.WorldAttributes{HostConnection: "steam:765"})
	require.NoError(t, err)

	now = now.Add(10 * time.Minute)
	reg.On("LiveServerIDs", mock.Anything).Return(liveSet(), nil)
	assert.Equal(t, 0, svc.reconcile(context.Background(), time.Minute, discardLog()))
	assert.Equal(t, float64(1), svc.ActiveSessions())
}

func TestNewWorldToken_HashBateEUnico(t *testing.T) {
	a, hashA, err := newWorldToken()
	require.NoError(t, err)
	b, _, err := newWorldToken()
	require.NoError(t, err)
	assert.NotEqual(t, a, b)
	assert.Equal(t, hashWorldToken(a), hashA)
	assert.Len(t, hashA, 64)
}

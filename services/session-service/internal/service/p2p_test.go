package service

import (
	"context"
	"strings"
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

	got, closed, ok := svc.ResolveWorldToken(worldToken)
	assert.True(t, ok)
	assert.False(t, closed)
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
	_, _, ok := svc.ResolveWorldToken("")
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
	id, closed, ok := svc.ResolveWorldToken(worldToken)
	assert.True(t, ok, "continua resolvendo para o reenvio do fechamento (spec 029)")
	assert.True(t, closed, "mas marcado como fechado")
	assert.Equal(t, sess.ID, id)
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

// Revisão: o token de mundo fechado sai da janela junto com o histórico.
func TestWorldToken_FechadoSaiDaJanela(t *testing.T) {
	svc, _, iss := p2pSetup(t, WithHistoryLimit(1))
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("tok", nil)
	ctx := context.Background()
	open := func(host string) (string, string) {
		sess, _, tok, err := svc.OpenWorld(ctx, host, "game-1", "default", domain.WorldAttributes{HostConnection: "steam:" + host})
		require.NoError(t, err)
		return sess.ID, tok
	}
	id1, tok1 := open("h1")
	id2, tok2 := open("h2")
	require.NoError(t, svc.CloseWorld(ctx, id1))
	require.NoError(t, svc.CloseWorld(ctx, id2))

	_, _, ok := svc.ResolveWorldToken(tok1)
	assert.False(t, ok, "o mais antigo saiu (limite 1)")
	_, closed, ok := svc.ResolveWorldToken(tok2)
	assert.True(t, ok)
	assert.True(t, closed)
}

// Revisão: o host p2p não reporta evento de quem nunca esteve no mundo.
func TestReportParticipation_P2P_JogadorForaDoMundo_Rejeita(t *testing.T) {
	part := &mockParticipation{}
	svc, _, iss := p2pSetup(t, WithParticipationPublisher(part))
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("tok", nil)
	ctx := context.Background()
	sess, _, _, err := svc.OpenWorld(ctx, "host-1", "game-1", "default", domain.WorldAttributes{HostConnection: "steam:1"})
	require.NoError(t, err)

	err = svc.ReportParticipationEvent(ctx, sess.ID, domain.ParticipationEvent{PlayerID: "vitima", Role: "coop", Event: "BOSS_DEFEATED", Outcome: "SUCCESS"})
	assert.ErrorIs(t, err, domain.ErrInvalidParticipation)
	part.AssertNotCalled(t, "PublishParticipation", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)

	part.On("PublishParticipation", mock.Anything, "game-1", sess.ID, "default", mock.Anything).Return(nil)
	require.NoError(t, svc.ReportParticipationEvent(ctx, sess.ID, domain.ParticipationEvent{PlayerID: "host-1", Role: "host", Event: "BOSS_DEFEATED", Outcome: "SUCCESS"}),
		"o próprio host é participante")
}

// Revisão: um mundo p2p aberto por host; abrir outro fecha o anterior.
func TestOpenWorld_P2P_UmMundoPorHost(t *testing.T) {
	svc, _, iss := p2pSetup(t)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("tok", nil)
	ctx := context.Background()
	first, _, _, err := svc.OpenWorld(ctx, "host-1", "game-1", "default", domain.WorldAttributes{HostConnection: "steam:1"})
	require.NoError(t, err)
	_, _, _, err = svc.OpenWorld(ctx, "host-1", "game-1", "default", domain.WorldAttributes{HostConnection: "steam:1"})
	require.NoError(t, err)

	assert.Equal(t, float64(1), svc.ActiveSessions())
	assert.Equal(t, domain.SessionClosed, first.State)
	assert.Equal(t, domain.CloseHostLeft, first.CloseReason)
}

func TestValidHostConnection(t *testing.T) {
	assert.NoError(t, validHostConnection("steam:76561198000000042"))
	assert.ErrorIs(t, validHostConnection(""), domain.ErrHostConnectionRequired)
	assert.ErrorIs(t, validHostConnection(strings.Repeat("x", 129)), domain.ErrHostConnectionRequired)
	assert.ErrorIs(t, validHostConnection("steam 1"), domain.ErrHostConnectionRequired)
	assert.ErrorIs(t, validHostConnection("steam:1\n"), domain.ErrHostConnectionRequired)
}

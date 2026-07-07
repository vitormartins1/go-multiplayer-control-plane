package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-game-backend/services/session-service/internal/domain"
)

// coopInvaderProfile é um perfil host_world_dropin com ambos os papéis.
func coopInvaderProfile() domain.WorldProfile {
	return domain.WorldProfile{
		Archetype:    "host_world_dropin",
		EnabledRoles: []string{domain.RoleCoop, domain.RoleInvader},
		CoopSlots:    2,
		InvaderSlots: 1,
	}
}

// openWorldSetup monta um service com perfil host_world e abre um mundo do host.
// Devolve o service, os mocks e o sessionID do mundo aberto.
func openWorldSetup(t *testing.T) (*Service, *mockAllocator, *mockIssuer, string) {
	t.Helper()
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	prof := &mockProfile{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	prof.On("WorldProfile", mock.Anything, "game-1").Return(coopInvaderProfile(), nil)

	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(prof))
	sess, grant, err := svc.OpenWorld(context.Background(), "host-1", "game-1")
	require.NoError(t, err)
	require.Equal(t, "host-1", grant.PlayerID)
	require.Equal(t, "token", grant.Token)
	return svc, alloc, iss, sess.ID
}

func TestOpenWorld_PerfilOk_CriaMundoComSlotsEGrantDoHost(t *testing.T) {
	svc, _, _, sessionID := openWorldSetup(t)

	active, _ := svc.AllSessions()
	require.Len(t, active, 1)
	sess := active[0]
	assert.Equal(t, sessionID, sess.ID)
	assert.Equal(t, domain.OriginHostWorld, sess.Origin)
	assert.Equal(t, "host-1", sess.HostPlayerID)
	assert.Equal(t, 2, sess.RoleSlots[domain.RoleCoop].Capacity)
	assert.Equal(t, 1, sess.RoleSlots[domain.RoleInvader].Capacity)
	assert.Empty(t, sess.MatchID, "host_world não tem matchID")
}

func TestOpenWorld_SemProfileProvider_ErrUnsupportedArchetype(t *testing.T) {
	svc := New(&mockAllocator{}, &mockIssuer{}, &mockPublisher{}) // sem WithProfileProvider
	_, _, err := svc.OpenWorld(context.Background(), "host-1", "game-1")
	assert.ErrorIs(t, err, domain.ErrUnsupportedArchetype)
}

func TestOpenWorld_PerfilIndisponivel_ErrUnsupportedArchetype(t *testing.T) {
	prof := &mockProfile{}
	prof.On("WorldProfile", mock.Anything, "game-1").Return(domain.WorldProfile{}, domain.ErrProfileUnavailable)
	svc := New(&mockAllocator{}, &mockIssuer{}, &mockPublisher{}, WithProfileProvider(prof))

	_, _, err := svc.OpenWorld(context.Background(), "host-1", "game-1")
	assert.ErrorIs(t, err, domain.ErrUnsupportedArchetype)
}

func TestOpenWorld_PoolEsgotado_ErrNoServer(t *testing.T) {
	alloc := &mockAllocator{}
	prof := &mockProfile{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{}, domain.ErrNoServer)
	prof.On("WorldProfile", mock.Anything, "game-1").Return(coopInvaderProfile(), nil)
	svc := New(alloc, &mockIssuer{}, &mockPublisher{}, WithProfileProvider(prof))

	_, _, err := svc.OpenWorld(context.Background(), "host-1", "game-1")
	assert.ErrorIs(t, err, domain.ErrNoServer)
}

func TestOpenWorld_TokenFalha_LiberaServer(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	prof := &mockProfile{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", assert.AnError)
	alloc.On("Release", mock.Anything, "server-0").Return(nil)
	prof.On("WorldProfile", mock.Anything, "game-1").Return(coopInvaderProfile(), nil)
	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(prof))

	_, _, err := svc.OpenWorld(context.Background(), "host-1", "game-1")
	require.Error(t, err)
	alloc.AssertCalled(t, "Release", mock.Anything, "server-0")
	assert.Equal(t, float64(0), svc.ActiveSessions())
}

func TestJoinWorld_SlotLivre_OcupaEEmiteToken(t *testing.T) {
	svc, _, _, sessionID := openWorldSetup(t)

	grant, err := svc.JoinWorld(context.Background(), sessionID, "phantom-1", domain.RoleCoop)
	require.NoError(t, err)
	assert.Equal(t, "phantom-1", grant.PlayerID)
	assert.Equal(t, float64(1), svc.OccupiedSlots(domain.RoleCoop))
	assert.Equal(t, float64(0), svc.OccupiedSlots(domain.RoleInvader))
}

func TestJoinWorld_PapelNaoHabilitado_ErrRoleNotEnabled(t *testing.T) {
	// perfil só com coop; invader não habilitado.
	prof := &mockProfile{}
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "s0", Address: "a"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	prof.On("WorldProfile", mock.Anything, "game-1").Return(domain.WorldProfile{
		Archetype: "host_world_dropin", EnabledRoles: []string{domain.RoleCoop}, CoopSlots: 1,
	}, nil)
	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(prof))
	sess, _, err := svc.OpenWorld(context.Background(), "host-1", "game-1")
	require.NoError(t, err)

	_, err = svc.JoinWorld(context.Background(), sess.ID, "phantom-1", domain.RoleInvader)
	assert.ErrorIs(t, err, domain.ErrRoleNotEnabled)
}

func TestJoinWorld_SlotCheio_ErrRoleFull(t *testing.T) {
	svc, _, _, sessionID := openWorldSetup(t)
	_, err := svc.JoinWorld(context.Background(), sessionID, "inv-1", domain.RoleInvader)
	require.NoError(t, err) // InvaderSlots=1, agora cheio

	_, err = svc.JoinWorld(context.Background(), sessionID, "inv-2", domain.RoleInvader)
	assert.ErrorIs(t, err, domain.ErrRoleFull)
}

func TestJoinWorld_TokenFalha_ReverteSlot(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	prof := &mockProfile{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "s0", Address: "a"}, nil)
	// host abre ok; join falha ao emitir o token.
	iss.On("Issue", "host-1", mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	iss.On("Issue", "phantom-1", mock.Anything, mock.Anything, mock.Anything).Return("", assert.AnError)
	prof.On("WorldProfile", mock.Anything, "game-1").Return(coopInvaderProfile(), nil)
	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(prof))
	sess, _, err := svc.OpenWorld(context.Background(), "host-1", "game-1")
	require.NoError(t, err)

	_, err = svc.JoinWorld(context.Background(), sess.ID, "phantom-1", domain.RoleCoop)
	require.Error(t, err)
	assert.Equal(t, float64(0), svc.OccupiedSlots(domain.RoleCoop), "slot revertido após falha de token")
}

func TestJoinWorld_MundoInexistente_ErrWorldNotFound(t *testing.T) {
	svc, _, _, _ := openWorldSetup(t)
	_, err := svc.JoinWorld(context.Background(), "nao-existe", "phantom-1", domain.RoleCoop)
	assert.ErrorIs(t, err, domain.ErrWorldNotFound)
}

func TestJoinWorld_JaPresente_ErrAlreadyJoined(t *testing.T) {
	svc, _, _, sessionID := openWorldSetup(t)
	_, err := svc.JoinWorld(context.Background(), sessionID, "phantom-1", domain.RoleCoop)
	require.NoError(t, err)

	_, err = svc.JoinWorld(context.Background(), sessionID, "phantom-1", domain.RoleCoop)
	assert.ErrorIs(t, err, domain.ErrAlreadyJoined)
}

func TestJoinWorld_HostNaoPodeEntrarComoPhantom_ErrAlreadyJoined(t *testing.T) {
	svc, _, _, sessionID := openWorldSetup(t)
	_, err := svc.JoinWorld(context.Background(), sessionID, "host-1", domain.RoleCoop)
	assert.ErrorIs(t, err, domain.ErrAlreadyJoined)
}

func TestLeaveWorld_LiberaSlotMantemMundo(t *testing.T) {
	svc, _, _, sessionID := openWorldSetup(t)
	_, err := svc.JoinWorld(context.Background(), sessionID, "phantom-1", domain.RoleCoop)
	require.NoError(t, err)
	require.Equal(t, float64(1), svc.OccupiedSlots(domain.RoleCoop))

	require.NoError(t, svc.LeaveWorld(context.Background(), sessionID, "phantom-1"))
	assert.Equal(t, float64(0), svc.OccupiedSlots(domain.RoleCoop))
	assert.Equal(t, float64(1), svc.ActiveSessions(), "mundo segue aberto")
}

func TestLeaveWorld_JogadorAusente_NoOp(t *testing.T) {
	svc, _, _, sessionID := openWorldSetup(t)
	// ninguém entrou; leave é no-op idempotente.
	require.NoError(t, svc.LeaveWorld(context.Background(), sessionID, "ninguem"))
	assert.Equal(t, float64(1), svc.ActiveSessions())
}

func TestLeaveWorld_Host_EncerraMundo(t *testing.T) {
	svc, alloc, _, sessionID := openWorldSetup(t)
	alloc.On("Release", mock.Anything, "server-0").Return(nil)

	require.NoError(t, svc.LeaveWorld(context.Background(), sessionID, "host-1"))
	assert.Equal(t, float64(0), svc.ActiveSessions(), "host saiu → mundo encerrado")
	assert.Equal(t, float64(1), svc.ClosedTotal(domain.CloseHostLeft))
	alloc.AssertCalled(t, "Release", mock.Anything, "server-0")
}

func TestCloseWorld_HostLeftLiberaServer(t *testing.T) {
	svc, alloc, _, sessionID := openWorldSetup(t)
	alloc.On("Release", mock.Anything, "server-0").Return(nil)

	require.NoError(t, svc.CloseWorld(context.Background(), sessionID))
	assert.Equal(t, float64(0), svc.ActiveSessions())
	assert.Equal(t, float64(1), svc.ClosedTotal(domain.CloseHostLeft))
}

func TestCloseWorld_Inexistente_ErrWorldNotFound(t *testing.T) {
	svc, _, _, _ := openWorldSetup(t)
	err := svc.CloseWorld(context.Background(), "nao-existe")
	assert.ErrorIs(t, err, domain.ErrWorldNotFound)
}

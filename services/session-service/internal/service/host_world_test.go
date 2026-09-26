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

// coopInvaderProfile é um perfil host_world_dropin com ambos os papéis.
func coopInvaderProfile() domain.ProfileSnapshot {
	return domain.ProfileSnapshot{
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
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	prof.On("ModeProfile", mock.Anything, "game-1", mock.Anything).Return(coopInvaderProfile(), nil)

	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(prof))
	sess, grant, err := svc.OpenWorld(context.Background(), "host-1", "game-1", "default", domain.WorldAttributes{})
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
	_, _, err := svc.OpenWorld(context.Background(), "host-1", "game-1", "default", domain.WorldAttributes{})
	assert.ErrorIs(t, err, domain.ErrUnsupportedArchetype)
}

func TestOpenWorld_PerfilIndisponivel_ErrUnsupportedArchetype(t *testing.T) {
	prof := &mockProfile{}
	prof.On("ModeProfile", mock.Anything, "game-1", mock.Anything).Return(domain.ProfileSnapshot{}, domain.ErrProfileUnavailable)
	svc := New(&mockAllocator{}, &mockIssuer{}, &mockPublisher{}, WithProfileProvider(prof))

	_, _, err := svc.OpenWorld(context.Background(), "host-1", "game-1", "default", domain.WorldAttributes{})
	assert.ErrorIs(t, err, domain.ErrUnsupportedArchetype)
}

func TestOpenWorld_PoolEsgotado_ErrNoServer(t *testing.T) {
	alloc := &mockAllocator{}
	prof := &mockProfile{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{}, domain.ErrNoServer)
	prof.On("ModeProfile", mock.Anything, "game-1", mock.Anything).Return(coopInvaderProfile(), nil)
	svc := New(alloc, &mockIssuer{}, &mockPublisher{}, WithProfileProvider(prof))

	_, _, err := svc.OpenWorld(context.Background(), "host-1", "game-1", "default", domain.WorldAttributes{})
	assert.ErrorIs(t, err, domain.ErrNoServer)
}

// TestOpenWorld_PropagaFleetDoSnapshotParaAllocate (ADR-0026, spec 022).
func TestOpenWorld_PropagaFleetDoSnapshotParaAllocate(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	prof := &mockProfile{}
	snap := coopInvaderProfile()
	snap.Fleet = "capoeira-souls"
	alloc.On("Allocate", mock.Anything, mock.Anything, "game-1", "capoeira-souls").Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	prof.On("ModeProfile", mock.Anything, "game-1", mock.Anything).Return(snap, nil)
	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(prof))

	_, _, err := svc.OpenWorld(context.Background(), "host-1", "game-1", "default", domain.WorldAttributes{})
	require.NoError(t, err)
	alloc.AssertExpectations(t)
}

// TestOpenWorld_FleetIndisponivel_ErrFleetUnavailable (ADR-0026, spec 022).
func TestOpenWorld_FleetIndisponivel_ErrFleetUnavailable(t *testing.T) {
	alloc := &mockAllocator{}
	prof := &mockProfile{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{}, domain.ErrFleetUnavailable)
	prof.On("ModeProfile", mock.Anything, "game-1", mock.Anything).Return(coopInvaderProfile(), nil)
	svc := New(alloc, &mockIssuer{}, &mockPublisher{}, WithProfileProvider(prof))

	_, _, err := svc.OpenWorld(context.Background(), "host-1", "game-1", "default", domain.WorldAttributes{})
	assert.ErrorIs(t, err, domain.ErrFleetUnavailable)
}

func TestOpenWorld_TokenFalha_LiberaServer(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	prof := &mockProfile{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", assert.AnError)
	alloc.On("Release", mock.Anything, "server-0").Return(nil)
	prof.On("ModeProfile", mock.Anything, "game-1", mock.Anything).Return(coopInvaderProfile(), nil)
	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(prof))

	_, _, err := svc.OpenWorld(context.Background(), "host-1", "game-1", "default", domain.WorldAttributes{})
	require.Error(t, err)
	alloc.AssertCalled(t, "Release", mock.Anything, "server-0")
	assert.Equal(t, float64(0), svc.ActiveSessions())
}

func TestJoinWorld_SlotLivre_OcupaEEmiteToken(t *testing.T) {
	svc, _, _, sessionID := openWorldSetup(t)

	grant, addr, err := svc.JoinWorld(context.Background(), sessionID, "phantom-1", domain.RoleCoop)
	require.NoError(t, err)
	assert.Equal(t, "phantom-1", grant.PlayerID)
	assert.Equal(t, "10.0.0.1:7777", addr, "join devolve o endereço do server do mundo")
	assert.Equal(t, float64(1), svc.OccupiedSlots(domain.RoleCoop))
	assert.Equal(t, float64(0), svc.OccupiedSlots(domain.RoleInvader))
}

func TestJoinWorld_PapelNaoHabilitado_ErrRoleNotEnabled(t *testing.T) {
	// perfil só com coop; invader não habilitado.
	prof := &mockProfile{}
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "s0", Address: "a"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	prof.On("ModeProfile", mock.Anything, "game-1", mock.Anything).Return(domain.ProfileSnapshot{
		Archetype: "host_world_dropin", EnabledRoles: []string{domain.RoleCoop}, CoopSlots: 1,
	}, nil)
	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(prof))
	sess, _, err := svc.OpenWorld(context.Background(), "host-1", "game-1", "default", domain.WorldAttributes{})
	require.NoError(t, err)

	_, _, err = svc.JoinWorld(context.Background(), sess.ID, "phantom-1", domain.RoleInvader)
	assert.ErrorIs(t, err, domain.ErrRoleNotEnabled)
}

func TestJoinWorld_SlotCheio_ErrRoleFull(t *testing.T) {
	svc, _, _, sessionID := openWorldSetup(t)
	_, _, err := svc.JoinWorld(context.Background(), sessionID, "inv-1", domain.RoleInvader)
	require.NoError(t, err) // InvaderSlots=1, agora cheio

	_, _, err = svc.JoinWorld(context.Background(), sessionID, "inv-2", domain.RoleInvader)
	assert.ErrorIs(t, err, domain.ErrRoleFull)
}

func TestJoinWorld_TokenFalha_ReverteSlot(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	prof := &mockProfile{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "s0", Address: "a"}, nil)
	// host abre ok; join falha ao emitir o token.
	iss.On("Issue", "host-1", mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	iss.On("Issue", "phantom-1", mock.Anything, mock.Anything, mock.Anything).Return("", assert.AnError)
	prof.On("ModeProfile", mock.Anything, "game-1", mock.Anything).Return(coopInvaderProfile(), nil)
	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(prof))
	sess, _, err := svc.OpenWorld(context.Background(), "host-1", "game-1", "default", domain.WorldAttributes{})
	require.NoError(t, err)

	_, _, err = svc.JoinWorld(context.Background(), sess.ID, "phantom-1", domain.RoleCoop)
	require.Error(t, err)
	assert.Equal(t, float64(0), svc.OccupiedSlots(domain.RoleCoop), "slot revertido após falha de token")
	assert.Equal(t, []string{"host-1"}, worldPlayers(t, svc, sess.ID), "quem nunca entrou não vira participante")
}

func TestJoinWorld_MundoInexistente_ErrWorldNotFound(t *testing.T) {
	svc, _, _, _ := openWorldSetup(t)
	_, _, err := svc.JoinWorld(context.Background(), "nao-existe", "phantom-1", domain.RoleCoop)
	assert.ErrorIs(t, err, domain.ErrWorldNotFound)
}

func TestJoinWorld_JaPresente_ErrAlreadyJoined(t *testing.T) {
	svc, _, _, sessionID := openWorldSetup(t)
	_, _, err := svc.JoinWorld(context.Background(), sessionID, "phantom-1", domain.RoleCoop)
	require.NoError(t, err)

	_, _, err = svc.JoinWorld(context.Background(), sessionID, "phantom-1", domain.RoleCoop)
	assert.ErrorIs(t, err, domain.ErrAlreadyJoined)
}

func TestJoinWorld_HostNaoPodeEntrarComoPhantom_ErrAlreadyJoined(t *testing.T) {
	svc, _, _, sessionID := openWorldSetup(t)
	_, _, err := svc.JoinWorld(context.Background(), sessionID, "host-1", domain.RoleCoop)
	assert.ErrorIs(t, err, domain.ErrAlreadyJoined)
}

func TestLeaveWorld_LiberaSlotMantemMundo(t *testing.T) {
	svc, _, _, sessionID := openWorldSetup(t)
	_, _, err := svc.JoinWorld(context.Background(), sessionID, "phantom-1", domain.RoleCoop)
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

// multiWorldSvc monta um service capaz de abrir mundos host_world para vários
// jogos, com o perfil dado por gameID (spec 012 — diretório de invadíveis).
func multiWorldSvc(t *testing.T, profiles map[string]domain.ProfileSnapshot) *Service {
	t.Helper()
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	prof := &mockProfile{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "s0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	for g, p := range profiles {
		prof.On("ModeProfile", mock.Anything, g, mock.Anything).Return(p, nil)
	}
	return New(alloc, iss, &mockPublisher{}, WithProfileProvider(prof))
}

func TestOpenWorld_ComAtributos_GuardaSoulLevelEWeaponTier(t *testing.T) {
	svc := multiWorldSvc(t, map[string]domain.ProfileSnapshot{"game-1": coopInvaderProfile()})
	sess, _, err := svc.OpenWorld(context.Background(), "host-1", "game-1", "default", domain.WorldAttributes{SoulLevel: 120, WeaponTier: 5})
	require.NoError(t, err)
	assert.Equal(t, 120, sess.HostSoulLevel)
	assert.Equal(t, 5, sess.WeaponTier)
}

func TestOpenWorld_SemAtributos_MundoComZeros(t *testing.T) {
	svc := multiWorldSvc(t, map[string]domain.ProfileSnapshot{"game-1": coopInvaderProfile()})
	sess, _, err := svc.OpenWorld(context.Background(), "host-1", "game-1", "default", domain.WorldAttributes{})
	require.NoError(t, err)
	assert.Zero(t, sess.HostSoulLevel)
	assert.Zero(t, sess.WeaponTier)
}

func TestOpenWorld_ComSenha_GuardaHash(t *testing.T) {
	svc := multiWorldSvc(t, map[string]domain.ProfileSnapshot{"game-1": coopInvaderProfile()})
	sess, _, err := svc.OpenWorld(context.Background(), "host-1", "game-1", "default", domain.WorldAttributes{Password: "abrepalavra"})
	require.NoError(t, err)
	assert.Equal(t, domain.HashWorldPassword("abrepalavra"), sess.PasswordHash)
	assert.NotEmpty(t, sess.PasswordHash)
}

func TestOpenWorld_SemSenha_HashVazio(t *testing.T) {
	svc := multiWorldSvc(t, map[string]domain.ProfileSnapshot{"game-1": coopInvaderProfile()})
	sess, _, err := svc.OpenWorld(context.Background(), "host-1", "game-1", "default", domain.WorldAttributes{})
	require.NoError(t, err)
	assert.Empty(t, sess.PasswordHash)
}

func TestListInvadableWorlds_RefleteHashDaSenha(t *testing.T) {
	svc := multiWorldSvc(t, map[string]domain.ProfileSnapshot{"game-1": coopInvaderProfile()})
	ctx := context.Background()
	w, _, err := svc.OpenWorld(ctx, "host-1", "game-1", "default", domain.WorldAttributes{Password: "abrepalavra"})
	require.NoError(t, err)

	worlds, err := svc.ListInvadableWorlds(ctx, "game-1", "")
	require.NoError(t, err)
	require.Len(t, worlds, 1)
	assert.Equal(t, w.PasswordHash, worlds[0].PasswordHash)
	assert.Equal(t, domain.HashWorldPassword("abrepalavra"), worlds[0].PasswordHash)
}

func TestListInvadableWorlds_SoMundosAbertosComInvasorLivre(t *testing.T) {
	svc := multiWorldSvc(t, map[string]domain.ProfileSnapshot{"game-1": coopInvaderProfile()})
	ctx := context.Background()
	w, _, err := svc.OpenWorld(ctx, "host-1", "game-1", "default", domain.WorldAttributes{SoulLevel: 80, WeaponTier: 3})
	require.NoError(t, err)

	worlds, err := svc.ListInvadableWorlds(ctx, "game-1", "")
	require.NoError(t, err)
	require.Len(t, worlds, 1)
	assert.Equal(t, w.ID, worlds[0].SessionID)
	assert.Equal(t, 80, worlds[0].HostSoulLevel)
	assert.Equal(t, 3, worlds[0].WeaponTier)
	assert.Equal(t, 1, worlds[0].FreeInvaderSlots)
}

func TestListInvadableWorlds_ExcluiInvasorCheio(t *testing.T) {
	svc := multiWorldSvc(t, map[string]domain.ProfileSnapshot{"game-1": coopInvaderProfile()})
	ctx := context.Background()
	w, _, err := svc.OpenWorld(ctx, "host-1", "game-1", "default", domain.WorldAttributes{})
	require.NoError(t, err)
	_, _, err = svc.JoinWorld(ctx, w.ID, "inv-1", domain.RoleInvader) // InvaderSlots=1 → cheio
	require.NoError(t, err)

	worlds, err := svc.ListInvadableWorlds(ctx, "game-1", "")
	require.NoError(t, err)
	assert.Empty(t, worlds, "mundo com invasor cheio sai do diretório")
}

func TestListInvadableWorlds_ExcluiSemPapelInvasor(t *testing.T) {
	coopOnly := domain.ProfileSnapshot{Archetype: "host_world_dropin", EnabledRoles: []string{domain.RoleCoop}, CoopSlots: 2}
	svc := multiWorldSvc(t, map[string]domain.ProfileSnapshot{"game-1": coopOnly})
	ctx := context.Background()
	_, _, err := svc.OpenWorld(ctx, "host-1", "game-1", "default", domain.WorldAttributes{})
	require.NoError(t, err)

	worlds, err := svc.ListInvadableWorlds(ctx, "game-1", "")
	require.NoError(t, err)
	assert.Empty(t, worlds, "mundo sem papel invasor não é invadível")
}

func TestListInvadableWorlds_ExcluiOutroJogo(t *testing.T) {
	svc := multiWorldSvc(t, map[string]domain.ProfileSnapshot{
		"game-1": coopInvaderProfile(),
		"game-2": coopInvaderProfile(),
	})
	ctx := context.Background()
	_, _, err := svc.OpenWorld(ctx, "host-1", "game-1", "default", domain.WorldAttributes{})
	require.NoError(t, err)
	w2, _, err := svc.OpenWorld(ctx, "host-2", "game-2", "default", domain.WorldAttributes{})
	require.NoError(t, err)

	worlds, err := svc.ListInvadableWorlds(ctx, "game-2", "")
	require.NoError(t, err)
	require.Len(t, worlds, 1)
	assert.Equal(t, w2.ID, worlds[0].SessionID)
}

func TestListInvadableWorlds_ExcluiQueueArena(t *testing.T) {
	svc := multiWorldSvc(t, map[string]domain.ProfileSnapshot{"game-1": coopInvaderProfile()})
	ctx := context.Background()
	_, _, err := svc.Create(ctx, "match-1", "game-1", "default", []domain.TeamAssignment{{Players: []string{"p1"}}})
	require.NoError(t, err)

	worlds, err := svc.ListInvadableWorlds(ctx, "game-1", "")
	require.NoError(t, err)
	assert.Empty(t, worlds, "sessão queue_arena (Origin=match) não é invadível")
}

// TestListInvadableWorlds_OrdenaPorOpenedAtAsc prova o follow-up #11 do
// arquétipo: a iteração do mapa de sessões é aleatória, então sem ordenação
// explícita a ordem de saída variaria a cada chamada. Os mundos são abertos
// fora de ordem cronológica (C, A, B) para garantir que a saída reflita
// OpenedAt e não a ordem de criação.
func TestListInvadableWorlds_OrdenaPorOpenedAtAsc(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	prof := &mockProfile{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "s0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	prof.On("ModeProfile", mock.Anything, "game-1", mock.Anything).Return(coopInvaderProfile(), nil)

	base := time.Now()
	times := []time.Time{base.Add(3 * time.Hour), base.Add(1 * time.Hour), base.Add(2 * time.Hour)}
	i := 0
	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(prof), WithClock(func() time.Time {
		tm := times[i]
		i++
		return tm
	}))

	ctx := context.Background()
	wC, _, err := svc.OpenWorld(ctx, "host-c", "game-1", "default", domain.WorldAttributes{}) // OpenedAt = base+3h
	require.NoError(t, err)
	wA, _, err := svc.OpenWorld(ctx, "host-a", "game-1", "default", domain.WorldAttributes{}) // OpenedAt = base+1h
	require.NoError(t, err)
	wB, _, err := svc.OpenWorld(ctx, "host-b", "game-1", "default", domain.WorldAttributes{}) // OpenedAt = base+2h
	require.NoError(t, err)

	worlds, err := svc.ListInvadableWorlds(ctx, "game-1", "")
	require.NoError(t, err)
	require.Len(t, worlds, 3)
	got := []string{worlds[0].SessionID, worlds[1].SessionID, worlds[2].SessionID}
	assert.Equal(t, []string{wA.ID, wB.ID, wC.ID}, got, "mundo mais antigo (menor OpenedAt) primeiro")
}

func TestListInvadableWorlds_NenhumElegivel_ListaVaziaSemErro(t *testing.T) {
	svc := multiWorldSvc(t, map[string]domain.ProfileSnapshot{"game-1": coopInvaderProfile()})
	worlds, err := svc.ListInvadableWorlds(context.Background(), "game-1", "")
	require.NoError(t, err)
	assert.Empty(t, worlds)
}

// TestCloseByAdmin_MundoHostWorld_FechaPorSessionID cobre o bug da spec 024:
// mundos host_world_dropin não têm matchID (ver TestOpenWorld_PerfilOk...,
// "host_world não tem matchID"), então o force-close do operador precisa
// resolver pelo próprio sessionID, não só pelo matchIndex (caminho legado
// queue_arena).
func TestCloseByAdmin_MundoHostWorld_FechaPorSessionID(t *testing.T) {
	svc, alloc, _, sessionID := openWorldSetup(t)
	alloc.On("Release", mock.Anything, "server-0").Return(nil)

	sess, err := svc.CloseByAdmin(context.Background(), sessionID)
	require.NoError(t, err)
	assert.Equal(t, domain.CloseAdmin, sess.CloseReason)
	assert.Equal(t, float64(0), svc.ActiveSessions(), "sai das ativas")
	alloc.AssertExpectations(t)
}

// Spec matchmaking/012: com modo, o diretório só devolve mundos abertos nesse
// modo; sem modo, todos os modos do jogo (compat).
func TestListInvadableWorlds_FiltraPorModo(t *testing.T) {
	svc := multiWorldSvc(t, map[string]domain.ProfileSnapshot{"game-1": coopInvaderProfile()})
	ctx := context.Background()
	_, _, err := svc.OpenWorld(ctx, "host-1", "game-1", "roda-aberta", domain.WorldAttributes{})
	require.NoError(t, err)
	batizado, _, err := svc.OpenWorld(ctx, "host-2", "game-1", "batizado", domain.WorldAttributes{})
	require.NoError(t, err)

	worlds, err := svc.ListInvadableWorlds(ctx, "game-1", "batizado")
	require.NoError(t, err)
	require.Len(t, worlds, 1)
	assert.Equal(t, batizado.ID, worlds[0].SessionID)

	all, err := svc.ListInvadableWorlds(ctx, "game-1", "")
	require.NoError(t, err)
	assert.Len(t, all, 2, "sem modo: todos os mundos do jogo")

}

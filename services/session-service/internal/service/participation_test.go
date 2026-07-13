package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/domain"
)

// participationSetup abre um mundo host_world com um publisher de participação
// mockado (aceita qualquer evento). Devolve o svc, os mocks e o sessionID.
func participationSetup(t *testing.T) (*Service, *mockAllocator, *mockParticipation, string) {
	t.Helper()
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	prof := &mockProfile{}
	part := &mockParticipation{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	prof.On("ModeProfile", mock.Anything, "game-1", mock.Anything).Return(coopInvaderProfile(), nil)
	part.On("PublishParticipation", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(prof), WithParticipationPublisher(part))
	sess, _, err := svc.OpenWorld(context.Background(), "host-1", "game-1", "default", domain.WorldAttributes{})
	require.NoError(t, err)
	return svc, alloc, part, sess.ID
}

func evt(playerID, role, event, outcome string) domain.ParticipationEvent {
	return domain.ParticipationEvent{PlayerID: playerID, Role: role, Event: event, Outcome: outcome}
}

func TestReportParticipation_Joined_PublicaSemMexerNoSlot(t *testing.T) {
	svc, _, part, sid := participationSetup(t)
	_, _, err := svc.JoinWorld(context.Background(), sid, "coop-1", domain.RoleCoop)
	require.NoError(t, err)

	err = svc.ReportParticipationEvent(context.Background(), sid, evt("coop-1", domain.RoleCoop, domain.EventJoined, domain.OutcomeNeutral))
	require.NoError(t, err)
	assert.Equal(t, float64(1), svc.OccupiedSlots(domain.RoleCoop), "JOINED não mexe no slot")
	assert.Equal(t, float64(1), svc.ParticipationTotal(domain.EventJoined))
	part.AssertCalled(t, "PublishParticipation", mock.Anything, "game-1", sid, mock.Anything, mock.Anything)
}

func TestReportParticipation_BossDefeated_NaoMexeNoSlot(t *testing.T) {
	svc, _, _, sid := participationSetup(t)
	_, _, err := svc.JoinWorld(context.Background(), sid, "coop-1", domain.RoleCoop)
	require.NoError(t, err)

	err = svc.ReportParticipationEvent(context.Background(), sid, evt("coop-1", domain.RoleCoop, domain.EventBossDefeated, domain.OutcomeSuccess))
	require.NoError(t, err)
	assert.Equal(t, float64(1), svc.OccupiedSlots(domain.RoleCoop))
}

func TestReportParticipation_Died_LiberaSlot(t *testing.T) {
	svc, _, _, sid := participationSetup(t)
	_, _, err := svc.JoinWorld(context.Background(), sid, "inv-1", domain.RoleInvader)
	require.NoError(t, err)
	require.Equal(t, float64(1), svc.OccupiedSlots(domain.RoleInvader))

	err = svc.ReportParticipationEvent(context.Background(), sid, evt("inv-1", domain.RoleInvader, domain.EventDied, domain.OutcomeFail))
	require.NoError(t, err)
	assert.Equal(t, float64(0), svc.OccupiedSlots(domain.RoleInvader), "DIED libera o slot")
	assert.Equal(t, float64(1), svc.ActiveSessions(), "mundo segue aberto")
}

func TestReportParticipation_Left_LiberaSlot(t *testing.T) {
	svc, _, _, sid := participationSetup(t)
	_, _, err := svc.JoinWorld(context.Background(), sid, "coop-1", domain.RoleCoop)
	require.NoError(t, err)

	err = svc.ReportParticipationEvent(context.Background(), sid, evt("coop-1", domain.RoleCoop, domain.EventLeft, domain.OutcomeNeutral))
	require.NoError(t, err)
	assert.Equal(t, float64(0), svc.OccupiedSlots(domain.RoleCoop))
}

// worldPlayers lê o Players da sessão ativa pela superfície admin (cópia).
func worldPlayers(t *testing.T, svc *Service, sessionID string) []string {
	t.Helper()
	active, _ := svc.SessionsByGame("game-1")
	for _, s := range active {
		if s.ID == sessionID {
			return s.Players
		}
	}
	t.Fatalf("sessão %s não encontrada entre as ativas", sessionID)
	return nil
}

// Achado ao vivo (bateria 2026-07-12): o coop morria e SUMIA da sala do admin
// como se nunca tivesse entrado. DIED/LEFT liberam a CAPACIDADE (slot), mas a
// participação é histórica — Players mantém quem passou pelo mundo.
func TestReportParticipation_Died_MantemParticipanteNaSala(t *testing.T) {
	svc, _, _, sid := participationSetup(t)
	_, _, err := svc.JoinWorld(context.Background(), sid, "coop-1", domain.RoleCoop)
	require.NoError(t, err)

	err = svc.ReportParticipationEvent(context.Background(), sid, evt("coop-1", domain.RoleCoop, domain.EventDied, domain.OutcomeFail))
	require.NoError(t, err)
	assert.Equal(t, float64(0), svc.OccupiedSlots(domain.RoleCoop), "DIED libera o slot")
	assert.Equal(t, []string{"host-1", "coop-1"}, worldPlayers(t, svc, sid), "morto continua participante da sala")
}

// Re-entrada de quem já morreu neste mundo (novo sinal) ocupa slot de novo mas
// não duplica a lista de participantes.
func TestJoinWorld_ReentradaAposMorte_NaoDuplicaParticipante(t *testing.T) {
	svc, _, _, sid := participationSetup(t)
	_, _, err := svc.JoinWorld(context.Background(), sid, "coop-1", domain.RoleCoop)
	require.NoError(t, err)
	err = svc.ReportParticipationEvent(context.Background(), sid, evt("coop-1", domain.RoleCoop, domain.EventDied, domain.OutcomeFail))
	require.NoError(t, err)

	_, _, err = svc.JoinWorld(context.Background(), sid, "coop-1", domain.RoleCoop)
	require.NoError(t, err, "morto pode ser invocado de novo (slot livre)")
	assert.Equal(t, float64(1), svc.OccupiedSlots(domain.RoleCoop))
	assert.Equal(t, []string{"host-1", "coop-1"}, worldPlayers(t, svc, sid), "sem duplicata na re-entrada")
}

func TestReportParticipation_DiedAusente_NoOp(t *testing.T) {
	svc, _, _, sid := participationSetup(t)
	// ninguém entrou; DIED de um phantom ausente é no-op idempotente.
	err := svc.ReportParticipationEvent(context.Background(), sid, evt("ghost", domain.RoleCoop, domain.EventDied, domain.OutcomeFail))
	require.NoError(t, err)
	assert.Equal(t, float64(1), svc.ActiveSessions())
}

func TestReportParticipation_HostKilled_EncerraMundo(t *testing.T) {
	svc, alloc, _, sid := participationSetup(t)
	alloc.On("Release", mock.Anything, "server-0").Return(nil)

	err := svc.ReportParticipationEvent(context.Background(), sid, evt("inv-1", domain.RoleInvader, domain.EventHostKilled, domain.OutcomeSuccess))
	require.NoError(t, err)
	assert.Equal(t, float64(0), svc.ActiveSessions(), "HOST_KILLED encerra o mundo")
	assert.Equal(t, float64(1), svc.ClosedTotal(domain.CloseHostKilled))
	alloc.AssertCalled(t, "Release", mock.Anything, "server-0")
}

func TestReportParticipation_EventoInvalido_ErrInvalidParticipation(t *testing.T) {
	svc, _, part, sid := participationSetup(t)
	err := svc.ReportParticipationEvent(context.Background(), sid, evt("p", domain.RoleCoop, "TELEPORTED", domain.OutcomeNeutral))
	assert.ErrorIs(t, err, domain.ErrInvalidParticipation)
	part.AssertNotCalled(t, "PublishParticipation", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestReportParticipation_OutcomeInvalido_ErrInvalidParticipation(t *testing.T) {
	svc, _, _, sid := participationSetup(t)
	err := svc.ReportParticipationEvent(context.Background(), sid, evt("p", domain.RoleCoop, domain.EventJoined, "MAYBE"))
	assert.ErrorIs(t, err, domain.ErrInvalidParticipation)
}

func TestReportParticipation_MundoInexistente_ErrWorldNotFound(t *testing.T) {
	svc, _, _, _ := participationSetup(t)
	err := svc.ReportParticipationEvent(context.Background(), "nao-existe", evt("p", domain.RoleCoop, domain.EventJoined, domain.OutcomeNeutral))
	assert.ErrorIs(t, err, domain.ErrWorldNotFound)
}

func TestReportParticipation_PublishFalha_NaoMutaSlot(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	prof := &mockProfile{}
	part := &mockParticipation{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "a"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	prof.On("ModeProfile", mock.Anything, "game-1", mock.Anything).Return(coopInvaderProfile(), nil)
	part.On("PublishParticipation", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(assert.AnError)

	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(prof), WithParticipationPublisher(part))
	sess, _, err := svc.OpenWorld(context.Background(), "host-1", "game-1", "default", domain.WorldAttributes{})
	require.NoError(t, err)
	_, _, err = svc.JoinWorld(context.Background(), sess.ID, "inv-1", domain.RoleInvader)
	require.NoError(t, err)

	// DIED com publish falho: erro propaga e o slot NÃO é liberado.
	err = svc.ReportParticipationEvent(context.Background(), sess.ID, evt("inv-1", domain.RoleInvader, domain.EventDied, domain.OutcomeFail))
	require.Error(t, err)
	assert.Equal(t, float64(1), svc.OccupiedSlots(domain.RoleInvader), "publish falhou -> slot intacto")
}

func TestReportParticipation_SemPublisher_Erro(t *testing.T) {
	// mundo aberto sem WithParticipationPublisher.
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	prof := &mockProfile{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "s", Address: "a"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	prof.On("ModeProfile", mock.Anything, "game-1", mock.Anything).Return(coopInvaderProfile(), nil)
	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(prof))
	sess, _, err := svc.OpenWorld(context.Background(), "host-1", "game-1", "default", domain.WorldAttributes{})
	require.NoError(t, err)

	err = svc.ReportParticipationEvent(context.Background(), sess.ID, evt("p", domain.RoleCoop, domain.EventJoined, domain.OutcomeNeutral))
	assert.Error(t, err)
}

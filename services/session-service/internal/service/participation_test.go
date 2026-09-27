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
	sess, _, _, err := svc.OpenWorld(context.Background(), "host-1", "game-1", "default", domain.WorldAttributes{})
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

// TestReportParticipation_Stats_PassamSemAlteracao (spec 026): mesmo padrão
// dos outros 3 kinds — o service repassa Stats tal como chega, sem validar.
func TestReportParticipation_Stats_PassamSemAlteracao(t *testing.T) {
	svc, _, part, sid := participationSetup(t)
	_, _, err := svc.JoinWorld(context.Background(), sid, "coop-1", domain.RoleCoop)
	require.NoError(t, err)

	e := evt("coop-1", domain.RoleCoop, domain.EventBossDefeated, domain.OutcomeSuccess)
	e.Stats = map[string]int64{"kills": 3}
	require.NoError(t, svc.ReportParticipationEvent(context.Background(), sid, e))

	part.AssertCalled(t, "PublishParticipation", mock.Anything, "game-1", sid, mock.Anything,
		domain.ParticipationEvent{PlayerID: "coop-1", Role: domain.RoleCoop, Event: domain.EventBossDefeated, Outcome: domain.OutcomeSuccess, Stats: map[string]int64{"kills": 3}})
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

// Fix 2026-08-19 (bug achado pela bateria do data plane host_world_dropin):
// a infra de ANALYTICS fora do ar não pode prender o GAMEPLAY. Com o
// publisher falhando (SNS/localstack morto), o slot do phantom LIBERA mesmo
// assim; o erro ainda propaga (o server re-reporta com o MESMO event_id e o
// consumer deduplica — at-least-once preservado) e o retry é idempotente.
// Substitui o teste que defendia a ordem antiga ("publish antes: falha =>
// slot intacto"), cuja consequência real era mundo vivo INELEGÍVEL pra
// reinvasão ("nenhum mundo elegível") sempre que o localstack caía.
func TestReportParticipation_PublishFalha_SlotLiberaMesmoAssim(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	prof := &mockProfile{}
	part := &mockParticipation{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "a"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	prof.On("ModeProfile", mock.Anything, "game-1", mock.Anything).Return(coopInvaderProfile(), nil)
	part.On("PublishParticipation", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(assert.AnError)

	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(prof), WithParticipationPublisher(part))
	sess, _, _, err := svc.OpenWorld(context.Background(), "host-1", "game-1", "default", domain.WorldAttributes{})
	require.NoError(t, err)
	_, _, err = svc.JoinWorld(context.Background(), sess.ID, "inv-1", domain.RoleInvader)
	require.NoError(t, err)

	// DIED com publish falho: o erro propaga (server re-reporta), mas o slot
	// LIBERA — gameplay não é refém de analytics.
	err = svc.ReportParticipationEvent(context.Background(), sess.ID, evt("inv-1", domain.RoleInvader, domain.EventDied, domain.OutcomeFail))
	require.Error(t, err)
	assert.Equal(t, float64(0), svc.OccupiedSlots(domain.RoleInvader), "publish falhou -> slot libera mesmo assim")

	// retry do MESMO evento (mesmo event_id no fio real): idempotente — erro
	// de novo (publisher segue morto), slot segue 0, nada explode.
	err = svc.ReportParticipationEvent(context.Background(), sess.ID, evt("inv-1", domain.RoleInvader, domain.EventDied, domain.OutcomeFail))
	require.Error(t, err)
	assert.Equal(t, float64(0), svc.OccupiedSlots(domain.RoleInvader), "retry idempotente")
}

// participantOf lê o Participant de um jogador na sessão ativa pela
// superfície admin (cópia) — espelha worldPlayers, mas devolve o registro
// completo (spec 027: LastEvent/LastOutcome).
func participantOf(t *testing.T, svc *Service, sessionID, playerID string) *domain.Participant {
	t.Helper()
	active, _ := svc.SessionsByGame("game-1")
	for _, s := range active {
		if s.ID != sessionID {
			continue
		}
		for i := range s.Participants {
			if s.Participants[i].PlayerID == playerID {
				return &s.Participants[i]
			}
		}
	}
	return nil
}

func TestReportParticipation_GravaLastEventLastOutcome(t *testing.T) {
	svc, _, _, sid := participationSetup(t)
	_, _, err := svc.JoinWorld(context.Background(), sid, "coop-1", domain.RoleCoop)
	require.NoError(t, err)

	err = svc.ReportParticipationEvent(context.Background(), sid, evt("coop-1", domain.RoleCoop, domain.EventBossDefeated, domain.OutcomeSuccess))
	require.NoError(t, err)

	p := participantOf(t, svc, sid, "coop-1")
	require.NotNil(t, p)
	assert.Equal(t, domain.EventBossDefeated, p.LastEvent)
	assert.Equal(t, domain.OutcomeSuccess, p.LastOutcome)
}

// DIED libera o slot (RoleSlots) mas não apaga o participante — o outcome
// precisa sobreviver a essa liberação (achado da revisão do plan.md: as duas
// mutações acontecem em locks separados).
func TestReportParticipation_Died_GravaOutcomeMesmoLiberandoSlot(t *testing.T) {
	svc, _, _, sid := participationSetup(t)
	_, _, err := svc.JoinWorld(context.Background(), sid, "inv-1", domain.RoleInvader)
	require.NoError(t, err)

	err = svc.ReportParticipationEvent(context.Background(), sid, evt("inv-1", domain.RoleInvader, domain.EventDied, domain.OutcomeFail))
	require.NoError(t, err)

	p := participantOf(t, svc, sid, "inv-1")
	require.NotNil(t, p)
	assert.Equal(t, domain.EventDied, p.LastEvent)
	assert.Equal(t, domain.OutcomeFail, p.LastOutcome)
}

func TestReportParticipation_DiedAusente_NaoGravaOutcome(t *testing.T) {
	svc, _, _, sid := participationSetup(t)
	err := svc.ReportParticipationEvent(context.Background(), sid, evt("ghost", domain.RoleCoop, domain.EventDied, domain.OutcomeFail))
	require.NoError(t, err)
	assert.Nil(t, participantOf(t, svc, sid, "ghost"), "no-op: evento sem JoinWorld prévio não cria participante")
}

// Re-entrada com o MESMO papel não reseta o outcome (dedupe da spec 025);
// papel DIFERENTE reseta (nova ocorrência de participação, spec 027).
func TestJoinWorld_Reentrada_ResetaOutcomeSoQuandoTrocaDePapel(t *testing.T) {
	svc, _, _, sid := participationSetup(t)
	_, _, err := svc.JoinWorld(context.Background(), sid, "coop-1", domain.RoleCoop)
	require.NoError(t, err)
	err = svc.ReportParticipationEvent(context.Background(), sid, evt("coop-1", domain.RoleCoop, domain.EventDied, domain.OutcomeFail))
	require.NoError(t, err)

	// mesmo papel: outcome permanece.
	_, _, err = svc.JoinWorld(context.Background(), sid, "coop-1", domain.RoleCoop)
	require.NoError(t, err)
	p := participantOf(t, svc, sid, "coop-1")
	require.NotNil(t, p)
	assert.Equal(t, domain.EventDied, p.LastEvent, "mesmo papel não reseta")

	// libera o slot de coop antes de trocar de papel (JoinWorld recusa
	// re-entrada enquanto o jogador ainda ocupa um slot).
	err = svc.ReportParticipationEvent(context.Background(), sid, evt("coop-1", domain.RoleCoop, domain.EventLeft, domain.OutcomeNeutral))
	require.NoError(t, err)

	// papel diferente: outcome reseta.
	_, _, err = svc.JoinWorld(context.Background(), sid, "coop-1", domain.RoleInvader)
	require.NoError(t, err)
	p = participantOf(t, svc, sid, "coop-1")
	require.NotNil(t, p)
	assert.Empty(t, p.LastEvent, "papel diferente reseta o outcome")
	assert.Empty(t, p.LastOutcome)
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
	sess, _, _, err := svc.OpenWorld(context.Background(), "host-1", "game-1", "default", domain.WorldAttributes{})
	require.NoError(t, err)

	err = svc.ReportParticipationEvent(context.Background(), sess.ID, evt("p", domain.RoleCoop, domain.EventJoined, domain.OutcomeNeutral))
	assert.Error(t, err)
}

// hostKilledSetup abre um mundo com um publisher de participação controlável
// pelo teste (spec 029): cada chamada consome o próximo erro programado.
func hostKilledSetup(t *testing.T, releaseErr error) (*Service, *mockParticipation, string) {
	t.Helper()
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	prof := &mockProfile{}
	part := &mockParticipation{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	alloc.On("Release", mock.Anything, "server-0").Return(releaseErr)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	prof.On("ModeProfile", mock.Anything, "game-1", mock.Anything).Return(coopInvaderProfile(), nil)

	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(prof), WithParticipationPublisher(part))
	sess, _, _, err := svc.OpenWorld(context.Background(), "host-1", "game-1", "default", domain.WorldAttributes{})
	require.NoError(t, err)
	return svc, part, sess.ID
}

func hostKilled(eventID string) domain.ParticipationEvent {
	e := evt("inv-1", domain.RoleInvader, domain.EventHostKilled, domain.OutcomeSuccess)
	e.EventID = eventID
	return e
}

// Spec 029, critério 1: o publish do HOST_KILLED falha depois de o mundo já ter
// fechado; o reenvio da MESMA ocorrência precisa publicar (antes recebia
// NotFound e o evento se perdia para sempre).
func TestReportParticipation_HostKilled_PublishFalha_ReenvioPublica(t *testing.T) {
	svc, part, sid := hostKilledSetup(t, nil)
	part.On("PublishParticipation", mock.Anything, "game-1", sid, "default", hostKilled("ev-1")).Return(assert.AnError).Once()
	part.On("PublishParticipation", mock.Anything, "game-1", sid, "default", hostKilled("ev-1")).Return(nil).Once()

	err := svc.ReportParticipationEvent(context.Background(), sid, hostKilled("ev-1"))
	require.Error(t, err, "falha no publish propaga para o server reenviar")
	assert.Equal(t, float64(0), svc.ActiveSessions(), "o mundo fecha mesmo com o fan-out fora")

	err = svc.ReportParticipationEvent(context.Background(), sid, hostKilled("ev-1"))
	require.NoError(t, err, "reenvio da mesma ocorrência republica")
	part.AssertNumberOfCalls(t, "PublishParticipation", 2)
	assert.Equal(t, float64(1), svc.ClosedTotal(domain.CloseHostKilled), "reenvio não fecha de novo")
}

// Spec 029, critério 2: o Release do GameServer falha dentro do fechamento — o
// erro volta antes do publish; o reenvio publica.
func TestReportParticipation_HostKilled_ReleaseFalha_ReenvioPublica(t *testing.T) {
	svc, part, sid := hostKilledSetup(t, assert.AnError)
	part.On("PublishParticipation", mock.Anything, "game-1", sid, "default", hostKilled("ev-1")).Return(nil).Once()

	err := svc.ReportParticipationEvent(context.Background(), sid, hostKilled("ev-1"))
	require.Error(t, err)
	part.AssertNotCalled(t, "PublishParticipation", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)

	require.NoError(t, svc.ReportParticipationEvent(context.Background(), sid, hostKilled("ev-1")))
	part.AssertNumberOfCalls(t, "PublishParticipation", 1)
}

// Spec 029: data plane antigo (sem event_id) — a ocorrência é reconhecida por
// player_id + event.
func TestReportParticipation_HostKilled_SemEventID_ReenvioPublica(t *testing.T) {
	svc, part, sid := hostKilledSetup(t, nil)
	part.On("PublishParticipation", mock.Anything, "game-1", sid, "default", hostKilled("")).Return(assert.AnError).Once()
	part.On("PublishParticipation", mock.Anything, "game-1", sid, "default", hostKilled("")).Return(nil).Once()

	require.Error(t, svc.ReportParticipationEvent(context.Background(), sid, hostKilled("")))
	require.NoError(t, svc.ReportParticipationEvent(context.Background(), sid, hostKilled("")))
}

// Spec 029, critérios 3 e 4: só a ocorrência que fechou o mundo é aceita depois
// do fechamento; qualquer outro evento segue recebendo NotFound.
func TestReportParticipation_MundoFechado_OutroEvento_ErrWorldNotFound(t *testing.T) {
	svc, part, sid := hostKilledSetup(t, nil)
	part.On("PublishParticipation", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	require.NoError(t, svc.ReportParticipationEvent(context.Background(), sid, hostKilled("ev-1")))

	left := evt("coop-1", domain.RoleCoop, domain.EventLeft, domain.OutcomeNeutral)
	assert.ErrorIs(t, svc.ReportParticipationEvent(context.Background(), sid, left), domain.ErrWorldNotFound)
	assert.ErrorIs(t, svc.ReportParticipationEvent(context.Background(), sid, hostKilled("ev-2")), domain.ErrWorldNotFound)
	part.AssertNumberOfCalls(t, "PublishParticipation", 1)
}

// Testes da spec 025 (histórico de sessão): acumulação de Participants em
// Create/OpenWorld/JoinWorld e a persistência best-effort em closeSession.
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

type mockHistoryStore struct{ mock.Mock }

func (m *mockHistoryStore) SaveClosed(ctx context.Context, sess *domain.Session) error {
	return m.Called(ctx, sess).Error(0)
}

func (m *mockHistoryStore) ClosedByGame(ctx context.Context, gameID string, limit int, before time.Time) ([]domain.Session, error) {
	args := m.Called(ctx, gameID, limit, before)
	if s := args.Get(0); s != nil {
		return s.([]domain.Session), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockHistoryStore) ClosedByPlayer(ctx context.Context, gameID, playerID string, limit int, before time.Time) ([]domain.Session, error) {
	args := m.Called(ctx, gameID, playerID, limit, before)
	if s := args.Get(0); s != nil {
		return s.([]domain.Session), args.Error(1)
	}
	return nil, args.Error(1)
}

// worldParticipants lê o Participants da sessão ativa pela superfície admin
// (cópia) — irmão de worldPlayers (participation_test.go).
func worldParticipants(t *testing.T, svc *Service, gameID, sessionID string) []domain.Participant {
	t.Helper()
	active, _ := svc.SessionsByGame(gameID)
	for _, s := range active {
		if s.ID == sessionID {
			return s.Participants
		}
	}
	t.Fatalf("sessão %s não encontrada entre as ativas", sessionID)
	return nil
}

// --- Create (queue_arena/battle_royale/coop_mission): participantes iniciais ---

func TestCreate_FFA_ParticipantsComPapelVazio(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "s0", Address: "a"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("t", nil)
	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(stubProfiles{}))

	sess, _, err := svc.Create(context.Background(), "match-1", "game-1", "default", ffaTeams("p1", "p2"))
	require.NoError(t, err)

	require.Len(t, sess.Participants, 2)
	for _, p := range sess.Participants {
		assert.Empty(t, p.Role, "FFA: papel vazio")
		assert.Equal(t, sess.OpenedAt, p.JoinedAt, "todos entram no mesmo instante (criação atômica)")
	}
}

func TestCreate_ComTimes_ParticipantsComPapelTeam(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "s0", Address: "a"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("t", nil)
	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(stubProfiles{}))

	teams := []domain.TeamAssignment{
		{TeamID: "team-1", Players: []string{"p1"}},
		{TeamID: "team-2", Players: []string{"p2"}},
	}
	sess, _, err := svc.Create(context.Background(), "match-1", "game-1", "default", teams)
	require.NoError(t, err)

	byPlayer := map[string]string{}
	for _, p := range sess.Participants {
		byPlayer[p.PlayerID] = p.Role
	}
	assert.Equal(t, "team:team-1", byPlayer["p1"])
	assert.Equal(t, "team:team-2", byPlayer["p2"])
}

// --- OpenWorld: o host entra como participante role=host ---

func TestOpenWorld_ParticipantHostComRoleHost(t *testing.T) {
	svc, _, _, sessionID := openWorldSetup(t)
	participants := worldParticipants(t, svc, "game-1", sessionID)
	require.Len(t, participants, 1)
	assert.Equal(t, "host-1", participants[0].PlayerID)
	assert.Equal(t, domain.RoleHost, participants[0].Role)
}

// --- JoinWorld: phantom vira participante com o papel do join ---

func TestJoinWorld_ParticipantComPapelDoJoin(t *testing.T) {
	svc, _, _, sessionID := openWorldSetup(t)
	_, _, err := svc.JoinWorld(context.Background(), sessionID, "coop-1", domain.RoleCoop)
	require.NoError(t, err)

	participants := worldParticipants(t, svc, "game-1", sessionID)
	require.Len(t, participants, 2, "host + coop")
	var coop *domain.Participant
	for i := range participants {
		if participants[i].PlayerID == "coop-1" {
			coop = &participants[i]
		}
	}
	require.NotNil(t, coop, "coop-1 vira participante")
	assert.Equal(t, domain.RoleCoop, coop.Role)
}

// Achado ao vivo (2026-07-12) espelhado no domínio de Participants: DIED/LEFT
// libera o slot mas não apaga o participante — reentrada com o MESMO papel não
// duplica a entrada.
func TestJoinWorld_ReentradaMesmoPapel_NaoDuplicaParticipant(t *testing.T) {
	svc, _, _, sessionID := participationSetup(t)
	ctx := context.Background()
	_, _, err := svc.JoinWorld(ctx, sessionID, "coop-1", domain.RoleCoop)
	require.NoError(t, err)
	require.NoError(t, svc.ReportParticipationEvent(ctx, sessionID, domain.ParticipationEvent{
		PlayerID: "coop-1", Role: domain.RoleCoop, Event: domain.EventDied, Outcome: domain.OutcomeNeutral,
	}))
	_, _, err = svc.JoinWorld(ctx, sessionID, "coop-1", domain.RoleCoop) // reentra, mesmo papel
	require.NoError(t, err)

	participants := worldParticipants(t, svc, "game-1", sessionID)
	count := 0
	for _, p := range participants {
		if p.PlayerID == "coop-1" {
			count++
		}
	}
	assert.Equal(t, 1, count, "sem duplicar")
}

// Reentrada com papel DIFERENTE registra a ocorrência mais recente (spec 025).
func TestJoinWorld_ReentradaPapelDiferente_AtualizaParaOcorrenciaMaisRecente(t *testing.T) {
	svc, _, _, sessionID := participationSetup(t)
	ctx := context.Background()
	_, _, err := svc.JoinWorld(ctx, sessionID, "player-1", domain.RoleCoop)
	require.NoError(t, err)
	require.NoError(t, svc.ReportParticipationEvent(ctx, sessionID, domain.ParticipationEvent{
		PlayerID: "player-1", Role: domain.RoleCoop, Event: domain.EventLeft, Outcome: domain.OutcomeNeutral,
	}))
	_, _, err = svc.JoinWorld(ctx, sessionID, "player-1", domain.RoleInvader) // reentra como invasor
	require.NoError(t, err)

	participants := worldParticipants(t, svc, "game-1", sessionID)
	var found *domain.Participant
	for i := range participants {
		if participants[i].PlayerID == "player-1" {
			found = &participants[i]
		}
	}
	require.NotNil(t, found)
	assert.Equal(t, domain.RoleInvader, found.Role, "papel atualizado para o da ocorrência mais recente")
}

func TestJoinWorld_TokenFalha_ReverteParticipant(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	prof := &mockProfile{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "s0", Address: "a"}, nil)
	iss.On("Issue", "host-1", mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	iss.On("Issue", "phantom-1", mock.Anything, mock.Anything, mock.Anything).Return("", assert.AnError)
	prof.On("ModeProfile", mock.Anything, "game-1", mock.Anything).Return(coopInvaderProfile(), nil)
	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(prof))
	sess, _, err := svc.OpenWorld(context.Background(), "host-1", "game-1", "default", domain.WorldAttributes{})
	require.NoError(t, err)

	_, _, err = svc.JoinWorld(context.Background(), sess.ID, "phantom-1", domain.RoleCoop)
	require.Error(t, err)

	participants := worldParticipants(t, svc, "game-1", sess.ID)
	require.Len(t, participants, 1, "só o host: quem nunca entrou não vira participante")
	assert.Equal(t, "host-1", participants[0].PlayerID)
}

// --- closeSession: persistência best-effort (spec 025) ---

func TestReportResult_ComHistoryStore_ChamaSaveClosed(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	pub := &mockPublisher{}
	store := &mockHistoryStore{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	svc := New(alloc, iss, pub, WithProfileProvider(stubProfiles{}), WithHistoryStore(store))
	sess, _, err := svc.Create(context.Background(), "match-1", "game-1", "default", ffaTeams("p1"))
	require.NoError(t, err)

	pub.On("Publish", mock.Anything, "game-1", "match-1", "default", mock.Anything).Return(nil)
	alloc.On("Release", mock.Anything, "server-0").Return(nil)
	store.On("SaveClosed", mock.Anything, mock.MatchedBy(func(s *domain.Session) bool {
		return s.ID == sess.ID && s.CloseReason == domain.CloseResult
	})).Return(nil)

	require.NoError(t, svc.ReportResult(context.Background(), "match-1", validOutcome()))
	store.AssertExpectations(t)
	assert.Equal(t, float64(0), svc.HistoryPersistFailures())
}

// Falha do HistoryStore não propaga (ADR-0020: fechamento nunca depende de
// destino externo) e incrementa a métrica de falhas.
func TestReportResult_HistoryStoreFalha_NaoPropagaEIncrementaMetrica(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	pub := &mockPublisher{}
	store := &mockHistoryStore{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	svc := New(alloc, iss, pub, WithProfileProvider(stubProfiles{}), WithHistoryStore(store), WithLogger(discardLog()))
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", "default", ffaTeams("p1"))
	require.NoError(t, err)

	pub.On("Publish", mock.Anything, "game-1", "match-1", "default", mock.Anything).Return(nil)
	alloc.On("Release", mock.Anything, "server-0").Return(nil)
	store.On("SaveClosed", mock.Anything, mock.Anything).Return(errors.New("postgres fora"))

	require.NoError(t, svc.ReportResult(context.Background(), "match-1", validOutcome()), "falha de persistência não propaga")
	assert.Equal(t, float64(0), svc.ActiveSessions(), "sessão encerra normalmente")
	assert.Equal(t, float64(1), svc.HistoryPersistFailures())
}

// Sem HistoryStore configurado (dev sem Postgres): closeSession não quebra.
func TestReportResult_SemHistoryStore_NaoQuebra(t *testing.T) {
	svc, pub, alloc := reportSetup(t) // sem WithHistoryStore
	outcome := validOutcome()
	pub.On("Publish", mock.Anything, "game-1", "match-1", "default", outcome).Return(nil)
	alloc.On("Release", mock.Anything, "server-0").Return(nil)

	require.NoError(t, svc.ReportResult(context.Background(), "match-1", outcome))
	assert.Equal(t, float64(0), svc.HistoryPersistFailures())
}

// O critério 4 da spec: coop que morre num mundo host_world entra no histórico
// (via HistoryStore.SaveClosed) com role=coop preservado.
func TestCloseWorld_ComHistoryStore_ParticipantesIncluemCoopMorto(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	prof := &mockProfile{}
	part := &mockParticipation{}
	store := &mockHistoryStore{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	prof.On("ModeProfile", mock.Anything, "game-1", mock.Anything).Return(coopInvaderProfile(), nil)
	part.On("PublishParticipation", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(prof), WithParticipationPublisher(part), WithHistoryStore(store))

	ctx := context.Background()
	sess, _, err := svc.OpenWorld(ctx, "host-1", "game-1", "default", domain.WorldAttributes{})
	require.NoError(t, err)
	_, _, err = svc.JoinWorld(ctx, sess.ID, "coop-1", domain.RoleCoop)
	require.NoError(t, err)
	require.NoError(t, svc.ReportParticipationEvent(ctx, sess.ID, domain.ParticipationEvent{
		PlayerID: "coop-1", Role: domain.RoleCoop, Event: domain.EventDied, Outcome: domain.OutcomeNeutral,
	}))

	alloc.On("Release", mock.Anything, "server-0").Return(nil)
	store.On("SaveClosed", mock.Anything, mock.MatchedBy(func(s *domain.Session) bool {
		for _, p := range s.Participants {
			if p.PlayerID == "coop-1" && p.Role == domain.RoleCoop {
				return true
			}
		}
		return false
	})).Return(nil)

	require.NoError(t, svc.CloseWorld(ctx, sess.ID))
	store.AssertExpectations(t)
}

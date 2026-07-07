package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/domain"
)

// Testes do snapshot do modo + manifest + validação kind×arquétipo
// (ADR-0024, spec 017). Mocks/stubs em session_service_test.go.

func TestCreate_CongelaSnapshotDoModoEEmiteClaimModeID(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	prof := &mockProfile{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "s0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", "p1", "match-1", "10.0.0.1:7777", domain.ConnectClaims{ModeID: "warzone"}).Return("tok", nil)
	snap := domain.ProfileSnapshot{
		ModeID: "warzone", Archetype: domain.ArchetypeBattleRoyale, Topology: "dedicated",
		ParamsJSON: []byte(`{"lobbySize":60,"teamSize":3}`), Hash: "abc123",
	}
	prof.On("ModeProfile", mock.Anything, "game-1", "warzone").Return(snap, nil)

	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(prof))
	sess, _, err := svc.Create(context.Background(), "match-1", "game-1", "warzone", ffaTeams("p1"))
	require.NoError(t, err)
	assert.Equal(t, "warzone", sess.ModeID)
	require.NotNil(t, sess.Profile)
	assert.Equal(t, domain.ArchetypeBattleRoyale, sess.Profile.Archetype)
	iss.AssertExpectations(t)
	prof.AssertExpectations(t)
}

func TestCreate_PerfilIndisponivel_NadaAlocado(t *testing.T) {
	alloc := &mockAllocator{}
	prof := &mockProfile{}
	prof.On("ModeProfile", mock.Anything, "game-1", "nao-existe").
		Return(domain.ProfileSnapshot{}, domain.ErrProfileUnavailable)

	svc := New(alloc, &mockIssuer{}, &mockPublisher{}, WithProfileProvider(prof))
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", "nao-existe", ffaTeams("p1"))
	assert.ErrorIs(t, err, domain.ErrProfileUnavailable)
	alloc.AssertNotCalled(t, "Allocate", mock.Anything, mock.Anything, mock.Anything)
	assert.Equal(t, float64(0), svc.ActiveSessions())
}

// flakyProvider devolve um snapshot diferente a cada chamada — prova que a
// sessão NÃO refaz a leitura (snapshot imutável).
type flakyProvider struct{ calls int }

func (f *flakyProvider) ModeProfile(_ context.Context, _, modeID string) (domain.ProfileSnapshot, error) {
	f.calls++
	return domain.ProfileSnapshot{
		ModeID: modeID, Archetype: domain.ArchetypeQueueArena, Topology: "dedicated",
		ParamsJSON: []byte(fmt.Sprintf(`{"scoreTarget":%d}`, f.calls)), Hash: fmt.Sprintf("v%d", f.calls),
	}, nil
}

func TestSnapshot_Imutavel_MudancaNoRegistryNaoAfetaSessaoEmVoo(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "s0", Address: "a:1"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("tok", nil)
	prov := &flakyProvider{}

	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(prov))
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", "default", ffaTeams("p1"))
	require.NoError(t, err)

	m1, err := svc.GetSessionConfig(context.Background(), "match-1")
	require.NoError(t, err)
	m2, err := svc.GetSessionConfig(context.Background(), "match-1")
	require.NoError(t, err)
	assert.Equal(t, "v1", m1.ProfileHash, "manifest reflete o snapshot da criação")
	assert.Equal(t, m1.ProfileHash, m2.ProfileHash, "sem releitura — snapshot congelado")
	assert.Equal(t, 1, prov.calls, "o provider só foi consultado na criação")
}

func TestGetSessionConfig_PorMatchID_DevolveManifest(t *testing.T) {
	svc, _, _ := reportSetupArchetype(t, domain.ArchetypeQueueArena)

	m, err := svc.GetSessionConfig(context.Background(), "match-1")
	require.NoError(t, err)
	assert.Equal(t, "match-1", m.MatchID)
	assert.Equal(t, "game-1", m.GameID)
	assert.Equal(t, "default", m.ModeID)
	assert.Equal(t, domain.ArchetypeQueueArena, m.Archetype)
	assert.Equal(t, "dedicated", m.Topology)
	assert.NotEmpty(t, m.ParamsJSON)
}

func TestGetSessionConfig_PorSessionID_HostWorld(t *testing.T) {
	svc, _, _, sessionID := openWorldSetup(t) // host_world: identidade = sessionID
	m, err := svc.GetSessionConfig(context.Background(), sessionID)
	require.NoError(t, err)
	assert.Equal(t, sessionID, m.SessionID)
	assert.Empty(t, m.MatchID, "host_world não tem match")
	assert.Equal(t, "host_world_dropin", m.Archetype)
}

func TestGetSessionConfig_FechadaOuInexistente_ErrSessionNotFound(t *testing.T) {
	svc, pub, alloc := reportSetup(t)
	pub.On("Publish", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	alloc.On("Release", mock.Anything, mock.Anything).Return(nil)
	require.NoError(t, svc.ReportResult(context.Background(), "match-1", validOutcome()))

	_, err := svc.GetSessionConfig(context.Background(), "match-1")
	assert.ErrorIs(t, err, domain.ErrSessionNotFound, "sessão fechada não serve manifest")
	_, err = svc.GetSessionConfig(context.Background(), "nunca-existiu")
	assert.ErrorIs(t, err, domain.ErrSessionNotFound)
}

// Os três cruzamentos errados de kind×arquétipo: nada é publicado e a sessão
// segue aberta (o server pode corrigir e reenviar).
func TestReportResult_KindNaoCasaComArquetipo_ErrOutcomeMismatch(t *testing.T) {
	casos := []struct {
		nome      string
		archetype string
		outcome   domain.MatchOutcome
	}{
		{"queue_arena recebe placement", domain.ArchetypeQueueArena,
			placementOutcome(domain.PlacementEntry{PlayerID: "p1", Placement: 1})},
		{"battle_royale recebe win_loss_draw", domain.ArchetypeBattleRoyale, validOutcome()},
		{"coop_mission recebe placement", domain.ArchetypeCoopMission,
			placementOutcome(domain.PlacementEntry{PlayerID: "p1", Placement: 1})},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			svc, pub, _ := reportSetupArchetype(t, c.archetype)
			err := svc.ReportResult(context.Background(), "match-1", c.outcome)
			assert.ErrorIs(t, err, domain.ErrOutcomeMismatch)
			pub.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			assert.Equal(t, float64(1), svc.ActiveSessions(), "sessão segue aberta")
		})
	}
}

func TestReportResult_PublicaComModeIDDaSessao(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	pub := &mockPublisher{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "s0", Address: "a:1"}, nil)
	alloc.On("Release", mock.Anything, "s0").Return(nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("tok", nil)

	svc := New(alloc, iss, pub, WithProfileProvider(stubProfiles{}))
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", "ranked", ffaTeams("p1"))
	require.NoError(t, err)

	pub.On("Publish", mock.Anything, "game-1", "match-1", "ranked", mock.Anything).Return(nil)
	require.NoError(t, svc.ReportResult(context.Background(), "match-1", validOutcome()))
	pub.AssertExpectations(t)
}

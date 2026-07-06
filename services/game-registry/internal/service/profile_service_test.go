package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-game-backend/services/game-registry/internal/domain"
)

func validQA() domain.QueueArenaParams {
	return domain.QueueArenaParams{
		Structure: "teams", TeamCount: 2, TeamSize: 3,
		WinCondition: "first_to_score", ScoreTarget: 50,
		MatchStructure: "single", AllowDraw: true,
		ScoringStats: []string{"kills", "assists"},
	}
}

func validProfile() domain.MultiplayerProfile {
	qa := validQA()
	return domain.MultiplayerProfile{
		Archetype: domain.ArchetypeQueueArena, Topology: domain.TopologyDedicated, QueueArena: &qa,
	}
}

func mut(base domain.QueueArenaParams, f func(*domain.QueueArenaParams)) domain.QueueArenaParams {
	f(&base)
	return base
}

func TestSetMultiplayerProfile_Valido_UpsertEDevolve(t *testing.T) {
	repo := new(mockRepo)
	repo.On("UpsertMultiplayerProfile", mock.Anything, "g1", mock.Anything).Return(nil)

	p, err := New(repo).SetMultiplayerProfile(context.Background(), "g1", validProfile())
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, domain.ArchetypeQueueArena, p.Archetype)
	repo.AssertExpectations(t)
}

func TestSetMultiplayerProfile_ArchetypeNaoSuportado_Erro(t *testing.T) {
	p := validProfile()
	p.Archetype = "battle_royale"
	_, err := New(new(mockRepo)).SetMultiplayerProfile(context.Background(), "g1", p)
	assert.ErrorIs(t, err, domain.ErrUnsupportedArchetype)
}

func TestSetMultiplayerProfile_TopologiaNaoSuportada_Erro(t *testing.T) {
	p := validProfile()
	p.Topology = "p2p"
	_, err := New(new(mockRepo)).SetMultiplayerProfile(context.Background(), "g1", p)
	assert.ErrorIs(t, err, domain.ErrUnsupportedTopology)
}

func TestSetMultiplayerProfile_ParamsAusentes_ErrInvalidProfileParams(t *testing.T) {
	p := validProfile()
	p.QueueArena = nil
	_, err := New(new(mockRepo)).SetMultiplayerProfile(context.Background(), "g1", p)
	assert.ErrorIs(t, err, domain.ErrInvalidProfileParams)
}

func TestSetMultiplayerProfile_ParamsInvalidos_ErrInvalidProfileParams(t *testing.T) {
	base := validQA()
	cases := map[string]domain.QueueArenaParams{
		"structure desconhecido":         mut(base, func(q *domain.QueueArenaParams) { q.Structure = "coop" }),
		"teams com teamCount<2":          mut(base, func(q *domain.QueueArenaParams) { q.TeamCount = 1 }),
		"teams com teamSize<1":           mut(base, func(q *domain.QueueArenaParams) { q.TeamSize = 0 }),
		"teams com playerCount!=0":       mut(base, func(q *domain.QueueArenaParams) { q.PlayerCount = 4 }),
		"ffa com playerCount<2":          {Structure: "ffa", PlayerCount: 1, WinCondition: "objective", MatchStructure: "single"},
		"ffa com teamCount preenchido":   {Structure: "ffa", PlayerCount: 4, TeamCount: 2, WinCondition: "objective", MatchStructure: "single"},
		"winCondition desconhecida":      mut(base, func(q *domain.QueueArenaParams) { q.WinCondition = "domination_x" }),
		"first_to_score sem scoreTarget": mut(base, func(q *domain.QueueArenaParams) { q.ScoreTarget = 0 }),
		"highest_at_time sem duration":   {Structure: "ffa", PlayerCount: 8, WinCondition: "highest_at_time", MatchStructure: "single"},
		"matchStructure desconhecido":    mut(base, func(q *domain.QueueArenaParams) { q.MatchStructure = "ladder" }),
		"single com roundsToWin!=0":      mut(base, func(q *domain.QueueArenaParams) { q.RoundsToWin = 2 }),
		"best_of_rounds sem roundsToWin": mut(base, func(q *domain.QueueArenaParams) { q.MatchStructure = "best_of_rounds"; q.RoundsToWin = 0 }),
		"scoringStats invalido":          mut(base, func(q *domain.QueueArenaParams) { q.ScoringStats = []string{"Kills"} }),
	}
	for name, qa := range cases {
		t.Run(name, func(t *testing.T) {
			qa := qa
			p := domain.MultiplayerProfile{Archetype: domain.ArchetypeQueueArena, Topology: domain.TopologyDedicated, QueueArena: &qa}
			_, err := New(new(mockRepo)).SetMultiplayerProfile(context.Background(), "g1", p)
			assert.ErrorIs(t, err, domain.ErrInvalidProfileParams)
		})
	}
}

func TestSetMultiplayerProfile_VariantesValidas_Ok(t *testing.T) {
	cases := map[string]domain.QueueArenaParams{
		"ffa first_to_score":   {Structure: "ffa", PlayerCount: 8, WinCondition: "first_to_score", ScoreTarget: 25, MatchStructure: "single"},
		"teams best_of_rounds": {Structure: "teams", TeamCount: 2, TeamSize: 5, WinCondition: "objective", MatchStructure: "best_of_rounds", RoundsToWin: 3},
		"elimination sem alvo": {Structure: "ffa", PlayerCount: 2, WinCondition: "elimination", MatchStructure: "single"},
		"scoringStats vazio":   {Structure: "ffa", PlayerCount: 2, WinCondition: "objective", MatchStructure: "single", ScoringStats: nil},
	}
	for name, qa := range cases {
		t.Run(name, func(t *testing.T) {
			qa := qa
			repo := new(mockRepo)
			repo.On("UpsertMultiplayerProfile", mock.Anything, "g1", mock.Anything).Return(nil)
			p := domain.MultiplayerProfile{Archetype: domain.ArchetypeQueueArena, Topology: domain.TopologyDedicated, QueueArena: &qa}
			_, err := New(repo).SetMultiplayerProfile(context.Background(), "g1", p)
			assert.NoError(t, err)
		})
	}
}

func TestSetMultiplayerProfile_JogoInexistente_ErrGameNotFound(t *testing.T) {
	repo := new(mockRepo)
	repo.On("UpsertMultiplayerProfile", mock.Anything, "nao", mock.Anything).Return(domain.ErrGameNotFound)
	_, err := New(repo).SetMultiplayerProfile(context.Background(), "nao", validProfile())
	assert.ErrorIs(t, err, domain.ErrGameNotFound)
}

func TestGetMultiplayerProfile_Existe_Devolve(t *testing.T) {
	p := validProfile()
	repo := new(mockRepo)
	repo.On("FindByID", mock.Anything, "g1").Return(&domain.Game{ID: "g1", MultiplayerProfile: &p}, nil)

	got, err := New(repo).GetMultiplayerProfile(context.Background(), "g1")
	require.NoError(t, err)
	assert.Equal(t, domain.ArchetypeQueueArena, got.Archetype)
}

func TestGetMultiplayerProfile_SemPerfil_ErrProfileNotFound(t *testing.T) {
	repo := new(mockRepo)
	repo.On("FindByID", mock.Anything, "g1").Return(&domain.Game{ID: "g1"}, nil)
	_, err := New(repo).GetMultiplayerProfile(context.Background(), "g1")
	assert.ErrorIs(t, err, domain.ErrProfileNotFound)
}

func TestGetMultiplayerProfile_JogoInexistente_ErrGameNotFound(t *testing.T) {
	repo := new(mockRepo)
	repo.On("FindByID", mock.Anything, "nao").Return(nil, domain.ErrGameNotFound)
	_, err := New(repo).GetMultiplayerProfile(context.Background(), "nao")
	assert.ErrorIs(t, err, domain.ErrGameNotFound)
}

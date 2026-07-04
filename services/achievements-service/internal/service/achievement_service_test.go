package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-game-backend/services/achievements-service/internal/domain"
)

const gameID = "game-1"

type mockRepo struct{ mock.Mock }

func (m *mockRepo) ListCatalog(ctx context.Context, gameID string) ([]domain.Achievement, error) {
	args := m.Called(ctx, gameID)
	return args.Get(0).([]domain.Achievement), args.Error(1)
}

func (m *mockRepo) FindByEventType(ctx context.Context, gameID, eventType string) ([]domain.Achievement, error) {
	args := m.Called(ctx, gameID, eventType)
	return args.Get(0).([]domain.Achievement), args.Error(1)
}

func (m *mockRepo) ListPlayerProgress(ctx context.Context, gameID, playerID string) ([]domain.PlayerProgress, error) {
	args := m.Called(ctx, gameID, playerID)
	return args.Get(0).([]domain.PlayerProgress), args.Error(1)
}

func (m *mockRepo) RecordProgress(ctx context.Context, gameID, playerID, achievementID string, target int, at time.Time) (bool, error) {
	args := m.Called(ctx, gameID, playerID, achievementID, target, at)
	return args.Bool(0), args.Error(1)
}

func (m *mockRepo) ApplyMatchResult(ctx context.Context, gameID, matchID string, players []domain.PlayerEvents, at time.Time) (bool, error) {
	args := m.Called(ctx, gameID, matchID, players, at)
	return args.Bool(0), args.Error(1)
}

func (m *mockRepo) ProgressForAchievement(ctx context.Context, gameID, achievementID string) ([]domain.AchievementProgressEntry, error) {
	args := m.Called(ctx, gameID, achievementID)
	e, _ := args.Get(0).([]domain.AchievementProgressEntry)
	return e, args.Error(1)
}

func (m *mockRepo) CreateAchievement(ctx context.Context, a *domain.Achievement) error {
	return m.Called(ctx, a).Error(0)
}
func (m *mockRepo) UpdateAchievement(ctx context.Context, a *domain.Achievement) error {
	return m.Called(ctx, a).Error(0)
}
func (m *mockRepo) DeleteAchievement(ctx context.Context, gameID, id string) error {
	return m.Called(ctx, gameID, id).Error(0)
}

var (
	firstWin = domain.Achievement{GameID: gameID, ID: "first_win", Name: "Primeira Vitória", Description: "Vença uma", EventType: "match_won", Target: 1}
	tenWins  = domain.Achievement{GameID: gameID, ID: "ten_wins", Name: "Veterano", Description: "Vença 10", EventType: "match_won", Target: 10}
)

// ApplyMatchResult traduz o resultado bruto em eventos: match_played p/ todos,
// match_won só p/ quem venceu. A idempotência é do repo (tx + processed_matches);
// aqui validamos só a tradução delegada.
func TestApplyMatchResult_TraduzEventosPorVitoria(t *testing.T) {
	repo := new(mockRepo)
	svc := New(repo)

	results := []domain.MatchResult{
		{PlayerID: "p1", Won: true},
		{PlayerID: "p2", Won: false},
	}
	want := []domain.PlayerEvents{
		{PlayerID: "p1", EventTypes: []string{"match_played", "match_won"}},
		{PlayerID: "p2", EventTypes: []string{"match_played"}},
	}
	repo.On("ApplyMatchResult", mock.Anything, gameID, "match-1", want, mock.Anything).Return(false, nil)

	dup, err := svc.ApplyMatchResult(context.Background(), gameID, "match-1", results)
	require.NoError(t, err)
	assert.False(t, dup)
	repo.AssertExpectations(t)
}

// Partida já processada -> o repo devolve dup=true; o service propaga (no-op).
func TestApplyMatchResult_Duplicada_PropagaDup(t *testing.T) {
	repo := new(mockRepo)
	svc := New(repo)
	repo.On("ApplyMatchResult", mock.Anything, gameID, "match-1", mock.Anything, mock.Anything).Return(true, nil)

	dup, err := svc.ApplyMatchResult(context.Background(), gameID, "match-1", []domain.MatchResult{{PlayerID: "p1", Won: true}})
	require.NoError(t, err)
	assert.True(t, dup)
	repo.AssertExpectations(t)
}

func TestUpdateAchievement_Valido_DelegaComChave(t *testing.T) {
	repo := new(mockRepo)
	var captured *domain.Achievement
	repo.On("UpdateAchievement", mock.Anything, mock.AnythingOfType("*domain.Achievement")).
		Return(nil).
		Run(func(args mock.Arguments) { captured = args.Get(1).(*domain.Achievement) })

	a, err := New(repo).UpdateAchievement(context.Background(), gameID, "first_win",
		domain.Achievement{Name: "  Novo Nome  ", EventType: "match_won", Target: 3})
	require.NoError(t, err)
	assert.Equal(t, gameID, captured.GameID)
	assert.Equal(t, "first_win", captured.ID, "id/gameID vêm do path, não do corpo")
	assert.Equal(t, "Novo Nome", a.Name, "name trimado")
	assert.Equal(t, 3, a.Target)
}

func TestUpdateAchievement_NameVazio_ErrInvalid(t *testing.T) {
	_, err := New(new(mockRepo)).UpdateAchievement(context.Background(), gameID, "first_win",
		domain.Achievement{Name: "  ", EventType: "match_won", Target: 1})
	assert.ErrorIs(t, err, domain.ErrInvalidAchievement)
}

func TestUpdateAchievement_Inexistente_PropagaNotFound(t *testing.T) {
	repo := new(mockRepo)
	repo.On("UpdateAchievement", mock.Anything, mock.Anything).Return(domain.ErrAchievementNotFound)
	_, err := New(repo).UpdateAchievement(context.Background(), gameID, "fantasma",
		domain.Achievement{Name: "X", EventType: "e", Target: 1})
	assert.ErrorIs(t, err, domain.ErrAchievementNotFound)
}

func TestDeleteAchievement_DelegaAoRepo(t *testing.T) {
	repo := new(mockRepo)
	repo.On("DeleteAchievement", mock.Anything, gameID, "first_win").Return(nil)
	err := New(repo).DeleteAchievement(context.Background(), gameID, "first_win")
	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestRecordEvent_AtingeMeta_RetornaDesbloqueada(t *testing.T) {
	repo := &mockRepo{}
	repo.On("FindByEventType", mock.Anything, gameID, "match_won").Return([]domain.Achievement{firstWin}, nil)
	repo.On("RecordProgress", mock.Anything, gameID, "player-1", "first_win", 1, mock.Anything).Return(true, nil)

	unlocked, err := New(repo).RecordEvent(context.Background(), gameID, "player-1", domain.Event{Type: "match_won"})

	require.NoError(t, err)
	require.Len(t, unlocked, 1)
	require.Equal(t, "first_win", unlocked[0].ID)
	repo.AssertExpectations(t)
}

func TestRecordEvent_ProgrediuSemDesbloquear_ListaVazia(t *testing.T) {
	repo := &mockRepo{}
	repo.On("FindByEventType", mock.Anything, gameID, "match_won").Return([]domain.Achievement{tenWins}, nil)
	repo.On("RecordProgress", mock.Anything, gameID, "player-1", "ten_wins", 10, mock.Anything).Return(false, nil)

	unlocked, err := New(repo).RecordEvent(context.Background(), gameID, "player-1", domain.Event{Type: "match_won"})

	require.NoError(t, err)
	require.Empty(t, unlocked)
	repo.AssertExpectations(t)
}

func TestRecordEvent_SemMatch_ListaVazia(t *testing.T) {
	repo := &mockRepo{}
	repo.On("FindByEventType", mock.Anything, gameID, "evento_solto").Return([]domain.Achievement{}, nil)

	unlocked, err := New(repo).RecordEvent(context.Background(), gameID, "player-1", domain.Event{Type: "evento_solto"})

	require.NoError(t, err)
	require.Empty(t, unlocked)
	repo.AssertNotCalled(t, "RecordProgress", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestRecordEvent_SemType_ErrInvalidEvent(t *testing.T) {
	repo := &mockRepo{}

	_, err := New(repo).RecordEvent(context.Background(), gameID, "player-1", domain.Event{Type: ""})

	require.ErrorIs(t, err, domain.ErrInvalidEvent)
	repo.AssertNotCalled(t, "FindByEventType", mock.Anything, mock.Anything, mock.Anything)
}

func TestListForPlayer_RefleteProgressoEUnlock(t *testing.T) {
	at := time.Now().UTC()
	repo := &mockRepo{}
	repo.On("ListCatalog", mock.Anything, gameID).Return([]domain.Achievement{firstWin, tenWins}, nil)
	repo.On("ListPlayerProgress", mock.Anything, gameID, "player-1").Return([]domain.PlayerProgress{
		{AchievementID: "first_win", Progress: 1, UnlockedAt: at}, // desbloqueada
		{AchievementID: "ten_wins", Progress: 7},                  // em progresso 7/10
	}, nil)

	list, err := New(repo).ListForPlayer(context.Background(), gameID, "player-1")

	require.NoError(t, err)
	require.Len(t, list, 2)
	require.True(t, list[0].Unlocked)
	require.Equal(t, 1, list[0].Progress)
	require.Equal(t, 1, list[0].Target)
	require.Equal(t, at, list[0].UnlockedAt)
	require.False(t, list[1].Unlocked)
	require.Equal(t, 7, list[1].Progress)
	require.Equal(t, 10, list[1].Target)
	require.True(t, list[1].UnlockedAt.IsZero())
}

func TestRecordEvent_ErroNoRepo_Propaga(t *testing.T) {
	repo := &mockRepo{}
	repo.On("FindByEventType", mock.Anything, gameID, "match_won").Return([]domain.Achievement{}, errors.New("db down"))

	_, err := New(repo).RecordEvent(context.Background(), gameID, "player-1", domain.Event{Type: "match_won"})

	require.Error(t, err)
}

func TestCreateAchievement_Valido_PersisteComGameID(t *testing.T) {
	repo := &mockRepo{}
	var captured *domain.Achievement
	repo.On("CreateAchievement", mock.Anything, mock.AnythingOfType("*domain.Achievement")).
		Return(nil).
		Run(func(args mock.Arguments) { captured = args.Get(1).(*domain.Achievement) })

	a, err := New(repo).CreateAchievement(context.Background(), gameID, domain.Achievement{
		ID: "first_blood", Name: "  Primeiro Sangue  ", EventType: "kill", Target: 1,
	})
	require.NoError(t, err)
	assert.Equal(t, gameID, a.GameID)
	assert.Equal(t, "first_blood", a.ID)
	assert.Equal(t, "Primeiro Sangue", a.Name, "name trimado")
	assert.Equal(t, gameID, captured.GameID)
}

func TestCreateAchievement_Invalido(t *testing.T) {
	repo := &mockRepo{}
	cases := []domain.Achievement{
		{ID: "X", Name: "n", EventType: "kill"},      // slug inválido (1 char/maiúsc)
		{ID: "ok-slug", Name: "", EventType: "kill"}, // name vazio
		{ID: "ok-slug", Name: "n", EventType: ""},    // event_type vazio
	}
	for _, c := range cases {
		_, err := New(repo).CreateAchievement(context.Background(), gameID, c)
		assert.ErrorIs(t, err, domain.ErrInvalidAchievement)
	}
	repo.AssertNotCalled(t, "CreateAchievement", mock.Anything, mock.Anything)
}

func TestCreateAchievement_JaExiste_Propaga(t *testing.T) {
	repo := &mockRepo{}
	repo.On("CreateAchievement", mock.Anything, mock.Anything).Return(domain.ErrAchievementExists)

	_, err := New(repo).CreateAchievement(context.Background(), gameID, domain.Achievement{
		ID: "first_win", Name: "Primeira", EventType: "match_won", Target: 1,
	})
	assert.ErrorIs(t, err, domain.ErrAchievementExists)
}

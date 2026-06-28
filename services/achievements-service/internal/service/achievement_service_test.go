package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/c130n/game-backend/services/achievements-service/internal/domain"
)

type mockRepo struct{ mock.Mock }

func (m *mockRepo) ListCatalog(ctx context.Context) ([]domain.Achievement, error) {
	args := m.Called(ctx)
	return args.Get(0).([]domain.Achievement), args.Error(1)
}

func (m *mockRepo) FindByEventType(ctx context.Context, eventType string) ([]domain.Achievement, error) {
	args := m.Called(ctx, eventType)
	return args.Get(0).([]domain.Achievement), args.Error(1)
}

func (m *mockRepo) ListPlayerProgress(ctx context.Context, playerID string) ([]domain.PlayerProgress, error) {
	args := m.Called(ctx, playerID)
	return args.Get(0).([]domain.PlayerProgress), args.Error(1)
}

func (m *mockRepo) RecordProgress(ctx context.Context, playerID, achievementID string, target int, at time.Time) (bool, error) {
	args := m.Called(ctx, playerID, achievementID, target, at)
	return args.Bool(0), args.Error(1)
}

var (
	firstWin = domain.Achievement{ID: "first_win", Name: "Primeira Vitória", Description: "Vença uma", EventType: "match_won", Target: 1}
	tenWins  = domain.Achievement{ID: "ten_wins", Name: "Veterano", Description: "Vença 10", EventType: "match_won", Target: 10}
)

func TestRecordEvent_AtingeMeta_RetornaDesbloqueada(t *testing.T) {
	repo := &mockRepo{}
	repo.On("FindByEventType", mock.Anything, "match_won").Return([]domain.Achievement{firstWin}, nil)
	repo.On("RecordProgress", mock.Anything, "player-1", "first_win", 1, mock.Anything).Return(true, nil)

	unlocked, err := New(repo).RecordEvent(context.Background(), "player-1", domain.Event{Type: "match_won"})

	require.NoError(t, err)
	require.Len(t, unlocked, 1)
	require.Equal(t, "first_win", unlocked[0].ID)
	repo.AssertExpectations(t)
}

func TestRecordEvent_ProgrediuSemDesbloquear_ListaVazia(t *testing.T) {
	repo := &mockRepo{}
	repo.On("FindByEventType", mock.Anything, "match_won").Return([]domain.Achievement{tenWins}, nil)
	repo.On("RecordProgress", mock.Anything, "player-1", "ten_wins", 10, mock.Anything).Return(false, nil)

	unlocked, err := New(repo).RecordEvent(context.Background(), "player-1", domain.Event{Type: "match_won"})

	require.NoError(t, err)
	require.Empty(t, unlocked)
	repo.AssertExpectations(t)
}

func TestRecordEvent_SemMatch_ListaVazia(t *testing.T) {
	repo := &mockRepo{}
	repo.On("FindByEventType", mock.Anything, "evento_solto").Return([]domain.Achievement{}, nil)

	unlocked, err := New(repo).RecordEvent(context.Background(), "player-1", domain.Event{Type: "evento_solto"})

	require.NoError(t, err)
	require.Empty(t, unlocked)
	repo.AssertNotCalled(t, "RecordProgress", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestRecordEvent_SemType_ErrInvalidEvent(t *testing.T) {
	repo := &mockRepo{}

	_, err := New(repo).RecordEvent(context.Background(), "player-1", domain.Event{Type: ""})

	require.ErrorIs(t, err, domain.ErrInvalidEvent)
	repo.AssertNotCalled(t, "FindByEventType", mock.Anything, mock.Anything)
}

func TestListForPlayer_RefleteProgressoEUnlock(t *testing.T) {
	at := time.Now().UTC()
	repo := &mockRepo{}
	repo.On("ListCatalog", mock.Anything).Return([]domain.Achievement{firstWin, tenWins}, nil)
	repo.On("ListPlayerProgress", mock.Anything, "player-1").Return([]domain.PlayerProgress{
		{AchievementID: "first_win", Progress: 1, UnlockedAt: at}, // desbloqueada
		{AchievementID: "ten_wins", Progress: 7},                  // em progresso 7/10
	}, nil)

	list, err := New(repo).ListForPlayer(context.Background(), "player-1")

	require.NoError(t, err)
	require.Len(t, list, 2)
	// first_win
	require.True(t, list[0].Unlocked)
	require.Equal(t, 1, list[0].Progress)
	require.Equal(t, 1, list[0].Target)
	require.Equal(t, at, list[0].UnlockedAt)
	// ten_wins
	require.False(t, list[1].Unlocked)
	require.Equal(t, 7, list[1].Progress)
	require.Equal(t, 10, list[1].Target)
	require.True(t, list[1].UnlockedAt.IsZero())
}

func TestRecordEvent_ErroNoRepo_Propaga(t *testing.T) {
	repo := &mockRepo{}
	repo.On("FindByEventType", mock.Anything, "match_won").Return([]domain.Achievement{}, errors.New("db down"))

	_, err := New(repo).RecordEvent(context.Background(), "player-1", domain.Event{Type: "match_won"})

	require.Error(t, err)
}

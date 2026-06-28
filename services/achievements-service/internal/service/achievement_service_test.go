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

func (m *mockRepo) ListPlayerUnlocks(ctx context.Context, playerID string) ([]domain.Unlock, error) {
	args := m.Called(ctx, playerID)
	return args.Get(0).([]domain.Unlock), args.Error(1)
}

func (m *mockRepo) Unlock(ctx context.Context, playerID, achievementID string, at time.Time) (bool, error) {
	args := m.Called(ctx, playerID, achievementID, at)
	return args.Bool(0), args.Error(1)
}

var firstWin = domain.Achievement{ID: "first_win", Name: "Primeira Vitória", Description: "Vença uma partida", EventType: "match_won"}

func TestRecordEvent_EventoCasa_DesbloqueiaERetornaNova(t *testing.T) {
	repo := &mockRepo{}
	repo.On("FindByEventType", mock.Anything, "match_won").Return([]domain.Achievement{firstWin}, nil)
	repo.On("Unlock", mock.Anything, "player-1", "first_win", mock.Anything).Return(true, nil)

	unlocked, err := New(repo).RecordEvent(context.Background(), "player-1", domain.Event{Type: "match_won"})

	require.NoError(t, err)
	require.Len(t, unlocked, 1)
	require.Equal(t, "first_win", unlocked[0].ID)
	repo.AssertExpectations(t)
}

func TestRecordEvent_JaDesbloqueada_Idempotente_ListaVazia(t *testing.T) {
	repo := &mockRepo{}
	repo.On("FindByEventType", mock.Anything, "match_won").Return([]domain.Achievement{firstWin}, nil)
	repo.On("Unlock", mock.Anything, "player-1", "first_win", mock.Anything).Return(false, nil) // já existia

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
	repo.AssertNotCalled(t, "Unlock", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestRecordEvent_SemType_ErrInvalidEvent(t *testing.T) {
	repo := &mockRepo{}

	_, err := New(repo).RecordEvent(context.Background(), "player-1", domain.Event{Type: ""})

	require.ErrorIs(t, err, domain.ErrInvalidEvent)
	repo.AssertNotCalled(t, "FindByEventType", mock.Anything, mock.Anything)
}

func TestListForPlayer_RefleteUnlockedELocked(t *testing.T) {
	secondWin := domain.Achievement{ID: "ten_wins", Name: "Veterano", Description: "Vença 10", EventType: "match_won"}
	at := time.Now().UTC()
	repo := &mockRepo{}
	repo.On("ListCatalog", mock.Anything).Return([]domain.Achievement{firstWin, secondWin}, nil)
	repo.On("ListPlayerUnlocks", mock.Anything, "player-1").
		Return([]domain.Unlock{{PlayerID: "player-1", AchievementID: "first_win", UnlockedAt: at}}, nil)

	list, err := New(repo).ListForPlayer(context.Background(), "player-1")

	require.NoError(t, err)
	require.Len(t, list, 2)
	require.True(t, list[0].Unlocked)
	require.Equal(t, at, list[0].UnlockedAt)
	require.False(t, list[1].Unlocked)
	require.True(t, list[1].UnlockedAt.IsZero())
}

func TestRecordEvent_ErroNoRepo_Propaga(t *testing.T) {
	repo := &mockRepo{}
	repo.On("FindByEventType", mock.Anything, "match_won").Return([]domain.Achievement{}, errors.New("db down"))

	_, err := New(repo).RecordEvent(context.Background(), "player-1", domain.Event{Type: "match_won"})

	require.Error(t, err)
}

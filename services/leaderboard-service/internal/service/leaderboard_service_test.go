package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/c130n/game-backend/services/leaderboard-service/internal/domain"
)

type mockStore struct{ mock.Mock }

func (m *mockStore) Submit(ctx context.Context, playerID string, score int64) error {
	return m.Called(ctx, playerID, score).Error(0)
}

func (m *mockStore) Rank(ctx context.Context, playerID string) (int64, int64, error) {
	args := m.Called(ctx, playerID)
	return args.Get(0).(int64), args.Get(1).(int64), args.Error(2)
}

func (m *mockStore) Top(ctx context.Context, limit int) ([]domain.Entry, error) {
	args := m.Called(ctx, limit)
	e, _ := args.Get(0).([]domain.Entry)
	return e, args.Error(1)
}

func (m *mockStore) Range(ctx context.Context, start, stop int64) ([]domain.Entry, error) {
	args := m.Called(ctx, start, stop)
	e, _ := args.Get(0).([]domain.Entry)
	return e, args.Error(1)
}

func TestSubmit_RetornaEntryComRank1Based(t *testing.T) {
	store := &mockStore{}
	store.On("Submit", mock.Anything, "p1", int64(1500)).Return(nil)
	store.On("Rank", mock.Anything, "p1").Return(int64(2), int64(1500), nil) // 0-based rank 2

	e, err := New(store).Submit(context.Background(), "p1", 1500)

	require.NoError(t, err)
	require.Equal(t, int64(3), e.Rank) // 1-based
	require.Equal(t, int64(1500), e.Score)
	require.Equal(t, "p1", e.PlayerID)
}

func TestTop_ClampLimit(t *testing.T) {
	store := &mockStore{}
	store.On("Top", mock.Anything, maxLimit).Return([]domain.Entry{}, nil) // 999 -> 100

	_, err := New(store).Top(context.Background(), 999)

	require.NoError(t, err)
	store.AssertCalled(t, "Top", mock.Anything, maxLimit)
}

func TestTop_LimitZero_UsaDefault(t *testing.T) {
	store := &mockStore{}
	store.On("Top", mock.Anything, defaultLimit).Return([]domain.Entry{}, nil)

	_, err := New(store).Top(context.Background(), 0)

	require.NoError(t, err)
	store.AssertCalled(t, "Top", mock.Anything, defaultLimit)
}

func TestAround_CalculaJanela(t *testing.T) {
	store := &mockStore{}
	store.On("Rank", mock.Anything, "p1").Return(int64(10), int64(500), nil)
	store.On("Range", mock.Anything, int64(7), int64(13)).Return([]domain.Entry{}, nil) // 10±3

	_, err := New(store).Around(context.Background(), "p1", 3)

	require.NoError(t, err)
	store.AssertCalled(t, "Range", mock.Anything, int64(7), int64(13))
}

func TestAround_NoTopo_StartNaoNegativo(t *testing.T) {
	store := &mockStore{}
	store.On("Rank", mock.Anything, "p1").Return(int64(1), int64(900), nil)
	store.On("Range", mock.Anything, int64(0), int64(6)).Return([]domain.Entry{}, nil) // 1±5 -> start 0

	_, err := New(store).Around(context.Background(), "p1", 5)

	require.NoError(t, err)
	store.AssertCalled(t, "Range", mock.Anything, int64(0), int64(6))
}

func TestAround_NaoRankeado_Propaga(t *testing.T) {
	store := &mockStore{}
	store.On("Rank", mock.Anything, "p1").Return(int64(0), int64(0), domain.ErrNotRanked)

	_, err := New(store).Around(context.Background(), "p1", 3)

	require.ErrorIs(t, err, domain.ErrNotRanked)
	store.AssertNotCalled(t, "Range", mock.Anything, mock.Anything, mock.Anything)
}

func TestAround_ClampKMax(t *testing.T) {
	store := &mockStore{}
	store.On("Rank", mock.Anything, "p1").Return(int64(100), int64(10), nil)
	store.On("Range", mock.Anything, int64(50), int64(150)).Return([]domain.Entry{}, nil) // 100 ± 50 (clamp)

	_, err := New(store).Around(context.Background(), "p1", 999)

	require.NoError(t, err)
	store.AssertCalled(t, "Range", mock.Anything, int64(50), int64(150))
}

func TestSubmit_ErroStore_Propaga(t *testing.T) {
	store := &mockStore{}
	store.On("Submit", mock.Anything, "p1", int64(10)).Return(errors.New("redis down"))

	_, err := New(store).Submit(context.Background(), "p1", 10)

	require.Error(t, err)
	store.AssertNotCalled(t, "Rank", mock.Anything, mock.Anything)
}

func TestTop_ErroStore_Propaga(t *testing.T) {
	store := &mockStore{}
	store.On("Top", mock.Anything, defaultLimit).Return([]domain.Entry(nil), errors.New("redis down"))

	_, err := New(store).Top(context.Background(), 0)

	require.Error(t, err)
}

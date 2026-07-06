package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-game-backend/services/leaderboard-service/internal/domain"
)

const score = domain.BoardScore

type mockStore struct{ mock.Mock }

func (m *mockStore) Submit(ctx context.Context, gameID, board, playerID string, s int64) error {
	return m.Called(ctx, gameID, board, playerID, s).Error(0)
}

func (m *mockStore) Rank(ctx context.Context, gameID, board, playerID string) (int64, int64, error) {
	args := m.Called(ctx, gameID, board, playerID)
	return args.Get(0).(int64), args.Get(1).(int64), args.Error(2)
}

func (m *mockStore) Top(ctx context.Context, gameID, board string, limit int) ([]domain.Entry, error) {
	args := m.Called(ctx, gameID, board, limit)
	e, _ := args.Get(0).([]domain.Entry)
	return e, args.Error(1)
}

func (m *mockStore) ForceScore(ctx context.Context, gameID, board, playerID string, s int64) error {
	return m.Called(ctx, gameID, board, playerID, s).Error(0)
}

func (m *mockStore) Range(ctx context.Context, gameID, board string, start, stop int64) ([]domain.Entry, error) {
	args := m.Called(ctx, gameID, board, start, stop)
	e, _ := args.Get(0).([]domain.Entry)
	return e, args.Error(1)
}

func TestSubmit_RetornaEntryComRank1Based(t *testing.T) {
	store := &mockStore{}
	store.On("Submit", mock.Anything, "g1", score, "p1", int64(1500)).Return(nil)
	store.On("Rank", mock.Anything, "g1", score, "p1").Return(int64(2), int64(1500), nil) // 0-based rank 2

	e, err := New(store).Submit(context.Background(), "g1", score, "p1", 1500)

	require.NoError(t, err)
	require.Equal(t, int64(3), e.Rank) // 1-based
	require.Equal(t, int64(1500), e.Score)
	require.Equal(t, "p1", e.PlayerID)
}

func TestSubmit_BoardCustomizado_RepassaAoStore(t *testing.T) {
	store := &mockStore{}
	store.On("Submit", mock.Anything, "g1", "kills", "p1", int64(7)).Return(nil)
	store.On("Rank", mock.Anything, "g1", "kills", "p1").Return(int64(0), int64(7), nil)

	e, err := New(store).Submit(context.Background(), "g1", "kills", "p1", 7)

	require.NoError(t, err)
	require.Equal(t, int64(1), e.Rank)
	store.AssertCalled(t, "Submit", mock.Anything, "g1", "kills", "p1", int64(7))
}

func TestSubmit_BoardInvalido_ErrInvalidBoard(t *testing.T) {
	store := &mockStore{}
	_, err := New(store).Submit(context.Background(), "g1", "Kills!", "p1", 7)
	require.ErrorIs(t, err, domain.ErrInvalidBoard)
	store.AssertNotCalled(t, "Submit", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestTop_BoardInvalido_ErrInvalidBoard(t *testing.T) {
	store := &mockStore{}
	_, err := New(store).Top(context.Background(), "g1", "", 10) // board vazio é inválido no service
	require.ErrorIs(t, err, domain.ErrInvalidBoard)
	store.AssertNotCalled(t, "Top", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestForceSubmit_SubstituiERetornaEntry(t *testing.T) {
	store := &mockStore{}
	store.On("ForceScore", mock.Anything, "g1", score, "p1", int64(50)).Return(nil)
	store.On("Rank", mock.Anything, "g1", score, "p1").Return(int64(0), int64(50), nil)

	e, err := New(store).ForceSubmit(context.Background(), "g1", score, "p1", 50)

	require.NoError(t, err)
	require.Equal(t, int64(1), e.Rank)
	require.Equal(t, int64(50), e.Score)
	store.AssertCalled(t, "ForceScore", mock.Anything, "g1", score, "p1", int64(50))
}

func TestForceSubmit_BoardInvalido_ErrInvalidBoard(t *testing.T) {
	store := &mockStore{}
	_, err := New(store).ForceSubmit(context.Background(), "g1", "BAD!", "p1", 50)
	require.ErrorIs(t, err, domain.ErrInvalidBoard)
	store.AssertNotCalled(t, "ForceScore", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestAround_BoardInvalido_ErrInvalidBoard(t *testing.T) {
	store := &mockStore{}
	_, err := New(store).Around(context.Background(), "g1", "BAD!", "p1", 3)
	require.ErrorIs(t, err, domain.ErrInvalidBoard)
	store.AssertNotCalled(t, "Rank", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestTop_ClampLimit(t *testing.T) {
	store := &mockStore{}
	store.On("Top", mock.Anything, "g1", score, maxLimit).Return([]domain.Entry{}, nil) // 999 -> 100

	_, err := New(store).Top(context.Background(), "g1", score, 999)

	require.NoError(t, err)
	store.AssertCalled(t, "Top", mock.Anything, "g1", score, maxLimit)
}

func TestTop_LimitZero_UsaDefault(t *testing.T) {
	store := &mockStore{}
	store.On("Top", mock.Anything, "g1", score, defaultLimit).Return([]domain.Entry{}, nil)

	_, err := New(store).Top(context.Background(), "g1", score, 0)

	require.NoError(t, err)
	store.AssertCalled(t, "Top", mock.Anything, "g1", score, defaultLimit)
}

func TestAround_CalculaJanela(t *testing.T) {
	store := &mockStore{}
	store.On("Rank", mock.Anything, "g1", score, "p1").Return(int64(10), int64(500), nil)
	store.On("Range", mock.Anything, "g1", score, int64(7), int64(13)).Return([]domain.Entry{}, nil) // 10±3

	_, err := New(store).Around(context.Background(), "g1", score, "p1", 3)

	require.NoError(t, err)
	store.AssertCalled(t, "Range", mock.Anything, "g1", score, int64(7), int64(13))
}

func TestAround_NoTopo_StartNaoNegativo(t *testing.T) {
	store := &mockStore{}
	store.On("Rank", mock.Anything, "g1", score, "p1").Return(int64(1), int64(900), nil)
	store.On("Range", mock.Anything, "g1", score, int64(0), int64(6)).Return([]domain.Entry{}, nil) // 1±5 -> start 0

	_, err := New(store).Around(context.Background(), "g1", score, "p1", 5)

	require.NoError(t, err)
	store.AssertCalled(t, "Range", mock.Anything, "g1", score, int64(0), int64(6))
}

func TestAround_NaoRankeado_Propaga(t *testing.T) {
	store := &mockStore{}
	store.On("Rank", mock.Anything, "g1", score, "p1").Return(int64(0), int64(0), domain.ErrNotRanked)

	_, err := New(store).Around(context.Background(), "g1", score, "p1", 3)

	require.ErrorIs(t, err, domain.ErrNotRanked)
	store.AssertNotCalled(t, "Range", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestAround_ClampKMax(t *testing.T) {
	store := &mockStore{}
	store.On("Rank", mock.Anything, "g1", score, "p1").Return(int64(100), int64(10), nil)
	store.On("Range", mock.Anything, "g1", score, int64(50), int64(150)).Return([]domain.Entry{}, nil) // 100 ± 50 (clamp)

	_, err := New(store).Around(context.Background(), "g1", score, "p1", 999)

	require.NoError(t, err)
	store.AssertCalled(t, "Range", mock.Anything, "g1", score, int64(50), int64(150))
}

func TestSubmit_ErroStore_Propaga(t *testing.T) {
	store := &mockStore{}
	store.On("Submit", mock.Anything, "g1", score, "p1", int64(10)).Return(errors.New("redis down"))

	_, err := New(store).Submit(context.Background(), "g1", score, "p1", 10)

	require.Error(t, err)
	store.AssertNotCalled(t, "Rank", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestTop_ErroStore_Propaga(t *testing.T) {
	store := &mockStore{}
	store.On("Top", mock.Anything, "g1", score, defaultLimit).Return([]domain.Entry(nil), errors.New("redis down"))

	_, err := New(store).Top(context.Background(), "g1", score, 0)

	require.Error(t, err)
}

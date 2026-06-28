package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/c130n/game-backend/services/cloud-save-service/internal/domain"
)

type mockStore struct{ mock.Mock }

func (m *mockStore) Put(ctx context.Context, playerID string, snapshot domain.Snapshot) error {
	return m.Called(ctx, playerID, snapshot).Error(0)
}

func (m *mockStore) Get(ctx context.Context, playerID string) (domain.Snapshot, error) {
	args := m.Called(ctx, playerID)
	snap, _ := args.Get(0).(domain.Snapshot)
	return snap, args.Error(1)
}

func TestSave_ChamaStore(t *testing.T) {
	snap := domain.Snapshot{Blob: []byte("save-data"), ContentType: "application/octet-stream"}
	store := &mockStore{}
	store.On("Put", mock.Anything, "p1", snap).Return(nil)

	err := New(store).Save(context.Background(), "p1", snap)

	require.NoError(t, err)
	store.AssertExpectations(t)
}

func TestLoad_RetornaSnapshot(t *testing.T) {
	snap := domain.Snapshot{Blob: []byte("save-data"), ContentType: "application/json"}
	store := &mockStore{}
	store.On("Get", mock.Anything, "p1").Return(snap, nil)

	got, err := New(store).Load(context.Background(), "p1")

	require.NoError(t, err)
	require.Equal(t, snap, got)
}

func TestLoad_Inexistente_PropagaErrNotFound(t *testing.T) {
	store := &mockStore{}
	store.On("Get", mock.Anything, "p1").Return(domain.Snapshot{}, domain.ErrNotFound)

	_, err := New(store).Load(context.Background(), "p1")

	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestSave_ErroStore_Propaga(t *testing.T) {
	store := &mockStore{}
	store.On("Put", mock.Anything, "p1", mock.Anything).Return(errors.New("s3 down"))

	err := New(store).Save(context.Background(), "p1", domain.Snapshot{Blob: []byte("x")})

	require.Error(t, err)
}

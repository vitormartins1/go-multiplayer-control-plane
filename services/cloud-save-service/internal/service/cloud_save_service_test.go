package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/cloud-save-service/internal/domain"
)

type mockStore struct{ mock.Mock }

func (m *mockStore) Put(ctx context.Context, gameID, playerID string, snapshot domain.Snapshot) error {
	return m.Called(ctx, gameID, playerID, snapshot).Error(0)
}

func (m *mockStore) Get(ctx context.Context, gameID, playerID string) (domain.Snapshot, error) {
	args := m.Called(ctx, gameID, playerID)
	snap, _ := args.Get(0).(domain.Snapshot)
	return snap, args.Error(1)
}

func TestSave_ChamaStore(t *testing.T) {
	snap := domain.Snapshot{Blob: []byte("save-data"), ContentType: "application/octet-stream"}
	store := &mockStore{}
	store.On("Put", mock.Anything, "g1", "p1", snap).Return(nil)

	err := New(store).Save(context.Background(), "g1", "p1", snap)

	require.NoError(t, err)
	store.AssertExpectations(t)
}

func TestLoad_RetornaSnapshot(t *testing.T) {
	snap := domain.Snapshot{Blob: []byte("save-data"), ContentType: "application/json"}
	store := &mockStore{}
	store.On("Get", mock.Anything, "g1", "p1").Return(snap, nil)

	got, err := New(store).Load(context.Background(), "g1", "p1")

	require.NoError(t, err)
	require.Equal(t, snap, got)
}

func TestLoad_Inexistente_PropagaErrNotFound(t *testing.T) {
	store := &mockStore{}
	store.On("Get", mock.Anything, "g1", "p1").Return(domain.Snapshot{}, domain.ErrNotFound)

	_, err := New(store).Load(context.Background(), "g1", "p1")

	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestSave_ErroStore_Propaga(t *testing.T) {
	store := &mockStore{}
	store.On("Put", mock.Anything, "g1", "p1", mock.Anything).Return(errors.New("s3 down"))

	err := New(store).Save(context.Background(), "g1", "p1", domain.Snapshot{Blob: []byte("x")})

	require.Error(t, err)
}

func TestLoadFor_DelegaParaStore(t *testing.T) {
	snap := domain.Snapshot{Blob: []byte("save-data"), ContentType: "application/json"}
	store := &mockStore{}
	store.On("Get", mock.Anything, "g1", "p1").Return(snap, nil)

	got, err := New(store).LoadFor(context.Background(), "g1", "p1")

	require.NoError(t, err)
	require.Equal(t, snap, got)
	store.AssertExpectations(t)
}

func TestLoadFor_Inexistente_PropagaErrNotFound(t *testing.T) {
	store := &mockStore{}
	store.On("Get", mock.Anything, "g1", "p1").Return(domain.Snapshot{}, domain.ErrNotFound)

	_, err := New(store).LoadFor(context.Background(), "g1", "p1")

	require.ErrorIs(t, err, domain.ErrNotFound)
}

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/player-data-service/internal/domain"
)

type mockRepo struct{ mock.Mock }

func (m *mockRepo) Get(ctx context.Context, gameID, playerID, namespace, key string) ([]byte, error) {
	args := m.Called(ctx, gameID, playerID, namespace, key)
	b, _ := args.Get(0).([]byte)
	return b, args.Error(1)
}

func (m *mockRepo) GetNamespace(ctx context.Context, gameID, playerID, namespace string) (map[string][]byte, error) {
	args := m.Called(ctx, gameID, playerID, namespace)
	mp, _ := args.Get(0).(map[string][]byte)
	return mp, args.Error(1)
}

func (m *mockRepo) Put(ctx context.Context, gameID, playerID, namespace, key string, value []byte) error {
	return m.Called(ctx, gameID, playerID, namespace, key, value).Error(0)
}

func (m *mockRepo) Delete(ctx context.Context, gameID, playerID, namespace, key string) error {
	return m.Called(ctx, gameID, playerID, namespace, key).Error(0)
}

func (m *mockRepo) ListForPlayer(ctx context.Context, gameID, playerID string) ([]domain.Entry, error) {
	args := m.Called(ctx, gameID, playerID)
	e, _ := args.Get(0).([]domain.Entry)
	return e, args.Error(1)
}

func TestPut_Valido_ChamaRepo(t *testing.T) {
	repo := &mockRepo{}
	repo.On("Put", mock.Anything, "g1", "p1", "inventory", "gold", []byte(`100`)).Return(nil)

	err := New(repo).Put(context.Background(), "g1", "p1", "inventory", "gold", []byte(`100`))

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestGet_Inexistente_PropagaErrNotFound(t *testing.T) {
	repo := &mockRepo{}
	repo.On("Get", mock.Anything, "g1", "p1", "inventory", "gold").Return([]byte(nil), domain.ErrNotFound)

	_, err := New(repo).Get(context.Background(), "g1", "p1", "inventory", "gold")

	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestPut_NamespaceInvalido_ErrInvalidKey(t *testing.T) {
	repo := &mockRepo{}

	err := New(repo).Put(context.Background(), "g1", "p1", "bad namespace!", "gold", []byte(`1`))

	require.ErrorIs(t, err, domain.ErrInvalidKey)
	repo.AssertNotCalled(t, "Put", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestGet_KeyInvalida_ErrInvalidKey(t *testing.T) {
	repo := &mockRepo{}

	_, err := New(repo).Get(context.Background(), "g1", "p1", "inventory", "chave inválida")

	require.ErrorIs(t, err, domain.ErrInvalidKey)
	repo.AssertNotCalled(t, "Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestGetNamespace_RetornaPares(t *testing.T) {
	repo := &mockRepo{}
	repo.On("GetNamespace", mock.Anything, "g1", "p1", "stats").
		Return(map[string][]byte{"kills": []byte(`10`), "deaths": []byte(`2`)}, nil)

	m, err := New(repo).GetNamespace(context.Background(), "g1", "p1", "stats")

	require.NoError(t, err)
	require.Len(t, m, 2)
	require.Equal(t, []byte(`10`), m["kills"])
}

func TestDelete_Valido_ChamaRepo(t *testing.T) {
	repo := &mockRepo{}
	repo.On("Delete", mock.Anything, "g1", "p1", "inventory", "gold").Return(nil)

	err := New(repo).Delete(context.Background(), "g1", "p1", "inventory", "gold")

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestGetNamespace_NamespaceInvalido_ErrInvalidKey(t *testing.T) {
	repo := &mockRepo{}

	_, err := New(repo).GetNamespace(context.Background(), "g1", "p1", "ns inválido!")

	require.ErrorIs(t, err, domain.ErrInvalidKey)
	repo.AssertNotCalled(t, "GetNamespace", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestPut_ErroRepo_Propaga(t *testing.T) {
	repo := &mockRepo{}
	repo.On("Put", mock.Anything, "g1", "p1", "inventory", "gold", mock.Anything).Return(errors.New("db down"))

	err := New(repo).Put(context.Background(), "g1", "p1", "inventory", "gold", []byte(`1`))

	require.Error(t, err)
}

func TestDelete_ErroRepo_Propaga(t *testing.T) {
	repo := &mockRepo{}
	repo.On("Delete", mock.Anything, "g1", "p1", "inventory", "gold").Return(errors.New("db down"))

	err := New(repo).Delete(context.Background(), "g1", "p1", "inventory", "gold")

	require.Error(t, err)
}

func TestListForPlayer_RetornaEntries(t *testing.T) {
	repo := &mockRepo{}
	entries := []domain.Entry{
		{Namespace: "inventory", Key: "gold", Value: []byte(`100`)},
		{Namespace: "stats", Key: "kills", Value: []byte(`10`)},
	}
	repo.On("ListForPlayer", mock.Anything, "g1", "p1").Return(entries, nil)

	got, err := New(repo).ListForPlayer(context.Background(), "g1", "p1")

	require.NoError(t, err)
	require.Equal(t, entries, got)
	repo.AssertExpectations(t)
}

func TestListForPlayer_ErroRepo_Propaga(t *testing.T) {
	repo := &mockRepo{}
	repo.On("ListForPlayer", mock.Anything, "g1", "p1").Return([]domain.Entry(nil), errors.New("db down"))

	_, err := New(repo).ListForPlayer(context.Background(), "g1", "p1")

	require.Error(t, err)
}

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/identity-service/internal/domain"
)

func TestListPlayers_RepassaPaginacaoEDevolveTotal(t *testing.T) {
	repo := new(mockRepo)
	repo.On("ListAccounts", mock.Anything, 10, 20).Return([]domain.Account{{ID: "a", Handle: "Alice"}}, nil)
	repo.On("CountAccounts", mock.Anything).Return(42, nil)

	players, total, err := NewPlayerLister(repo).ListPlayers(context.Background(), 10, 20)
	require.NoError(t, err)
	require.Len(t, players, 1)
	assert.Equal(t, "Alice", players[0].Handle)
	assert.Equal(t, 42, total)
	repo.AssertExpectations(t)
}

func TestListPlayers_ClampDeLimit(t *testing.T) {
	cases := []struct {
		name      string
		inLimit   int
		wantLimit int
	}{
		{"acima do maximo satura", 1000, maxLimit},
		{"zero vira default", 0, defaultLimit},
		{"negativo vira default", -5, defaultLimit},
		{"valido passa", 75, 75},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := new(mockRepo)
			repo.On("ListAccounts", mock.Anything, c.wantLimit, 0).Return([]domain.Account{}, nil)
			repo.On("CountAccounts", mock.Anything).Return(0, nil)

			_, _, err := NewPlayerLister(repo).ListPlayers(context.Background(), c.inLimit, 0)
			require.NoError(t, err)
			repo.AssertExpectations(t) // garante que o repo recebeu o limit clampeado
		})
	}
}

func TestGetPlayer_Existente_DevolveConta(t *testing.T) {
	repo := new(mockRepo)
	repo.On("FindByID", mock.Anything, "acc-1").Return(&domain.Account{ID: "acc-1", Handle: "Alice"}, nil)

	acc, err := NewPlayerLister(repo).GetPlayer(context.Background(), "acc-1")
	require.NoError(t, err)
	assert.Equal(t, "Alice", acc.Handle)
	repo.AssertExpectations(t)
}

func TestGetPlayer_Inexistente_ErrAccountNotFound(t *testing.T) {
	repo := new(mockRepo)
	repo.On("FindByID", mock.Anything, "nope").Return(nil, domain.ErrAccountNotFound)

	_, err := NewPlayerLister(repo).GetPlayer(context.Background(), "nope")
	assert.ErrorIs(t, err, domain.ErrAccountNotFound)
}

func TestListPlayers_OffsetNegativo_ViraZero(t *testing.T) {
	repo := new(mockRepo)
	repo.On("ListAccounts", mock.Anything, defaultLimit, 0).Return([]domain.Account{}, nil)
	repo.On("CountAccounts", mock.Anything).Return(0, nil)

	_, _, err := NewPlayerLister(repo).ListPlayers(context.Background(), 0, -10)
	require.NoError(t, err)
	repo.AssertExpectations(t)
}

type mockExternalLister struct{ mock.Mock }

func (m *mockExternalLister) ListExternalByAccount(ctx context.Context, accountID string) ([]domain.ExternalIdentity, error) {
	args := m.Called(ctx, accountID)
	ids, _ := args.Get(0).([]domain.ExternalIdentity)
	return ids, args.Error(1)
}

// spec 006.
func TestExternalIdentities_ContaComVinculo(t *testing.T) {
	repo := new(mockRepo)
	repo.On("FindByID", mock.Anything, "id-1").Return(&domain.Account{ID: "id-1"}, nil)
	ext := new(mockExternalLister)
	ext.On("ListExternalByAccount", mock.Anything, "id-1").Return([]domain.ExternalIdentity{{Provider: "steam", ExternalID: "42"}}, nil)

	ids, err := NewPlayerLister(repo, WithExternalIdentities(ext)).ExternalIdentities(context.Background(), "id-1")
	require.NoError(t, err)
	assert.Equal(t, []domain.ExternalIdentity{{Provider: "steam", ExternalID: "42"}}, ids)
}

func TestExternalIdentities_ContaInexistente_ErrAccountNotFound(t *testing.T) {
	repo := new(mockRepo)
	repo.On("FindByID", mock.Anything, "nope").Return(nil, domain.ErrAccountNotFound)
	_, err := NewPlayerLister(repo, WithExternalIdentities(new(mockExternalLister))).ExternalIdentities(context.Background(), "nope")
	assert.ErrorIs(t, err, domain.ErrAccountNotFound)
}

func TestExternalIdentities_SemLoginFederado_ListaVazia(t *testing.T) {
	repo := new(mockRepo)
	repo.On("FindByID", mock.Anything, "id-1").Return(&domain.Account{ID: "id-1"}, nil)
	ids, err := NewPlayerLister(repo).ExternalIdentities(context.Background(), "id-1")
	require.NoError(t, err)
	assert.Empty(t, ids)
}

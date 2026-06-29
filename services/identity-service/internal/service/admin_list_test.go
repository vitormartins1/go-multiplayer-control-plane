package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/c130n/game-backend/services/identity-service/internal/domain"
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

func TestListPlayers_OffsetNegativo_ViraZero(t *testing.T) {
	repo := new(mockRepo)
	repo.On("ListAccounts", mock.Anything, defaultLimit, 0).Return([]domain.Account{}, nil)
	repo.On("CountAccounts", mock.Anything).Return(0, nil)

	_, _, err := NewPlayerLister(repo).ListPlayers(context.Background(), 0, -10)
	require.NoError(t, err)
	repo.AssertExpectations(t)
}

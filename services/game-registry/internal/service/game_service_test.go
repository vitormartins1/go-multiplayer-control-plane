package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/c130n/game-backend/services/game-registry/internal/domain"
)

type mockRepo struct{ mock.Mock }

func (m *mockRepo) Create(ctx context.Context, g *domain.Game) error {
	return m.Called(ctx, g).Error(0)
}
func (m *mockRepo) List(ctx context.Context) ([]domain.Game, error) {
	args := m.Called(ctx)
	games, _ := args.Get(0).([]domain.Game)
	return games, args.Error(1)
}
func (m *mockRepo) FindByID(ctx context.Context, id string) (*domain.Game, error) {
	args := m.Called(ctx, id)
	g, _ := args.Get(0).(*domain.Game)
	return g, args.Error(1)
}

func TestRegister_Valido_CriaComIDeCreatedAt(t *testing.T) {
	repo := new(mockRepo)
	var captured *domain.Game
	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Game")).
		Return(nil).
		Run(func(args mock.Arguments) { captured = args.Get(1).(*domain.Game) })

	g, err := New(repo).Register(context.Background(), "tetris", "  Tetris  ")
	require.NoError(t, err)
	assert.NotEmpty(t, g.ID)
	assert.Equal(t, "tetris", g.Slug)
	assert.Equal(t, "Tetris", g.Name, "name é trimado")
	assert.False(t, g.CreatedAt.IsZero())
	assert.Equal(t, g.ID, captured.ID)
}

func TestRegister_SlugInvalido_ErrInvalidSlug(t *testing.T) {
	cases := []string{"Tetris", "a", "-tetris", "tetris-", "te tris", "tétris", ""}
	for _, slug := range cases {
		t.Run(slug, func(t *testing.T) {
			_, err := New(new(mockRepo)).Register(context.Background(), slug, "Nome")
			assert.ErrorIs(t, err, domain.ErrInvalidSlug)
		})
	}
}

func TestRegister_NameInvalido_ErrInvalidName(t *testing.T) {
	_, err := New(new(mockRepo)).Register(context.Background(), "tetris", "   ")
	assert.ErrorIs(t, err, domain.ErrInvalidName)
}

func TestRegister_SlugTaken_PropagaErro(t *testing.T) {
	repo := new(mockRepo)
	repo.On("Create", mock.Anything, mock.Anything).Return(domain.ErrSlugTaken)
	_, err := New(repo).Register(context.Background(), "tetris", "Tetris")
	assert.ErrorIs(t, err, domain.ErrSlugTaken)
}

func TestGet_Inexistente_PropagaNotFound(t *testing.T) {
	repo := new(mockRepo)
	repo.On("FindByID", mock.Anything, "x").Return(nil, domain.ErrGameNotFound)
	_, err := New(repo).Get(context.Background(), "x")
	assert.ErrorIs(t, err, domain.ErrGameNotFound)
}

func TestList_DelegaAoRepo(t *testing.T) {
	repo := new(mockRepo)
	repo.On("List", mock.Anything).Return([]domain.Game{{Name: "Arkanoid"}}, nil)
	games, err := New(repo).List(context.Background())
	require.NoError(t, err)
	require.Len(t, games, 1)
	assert.Equal(t, "Arkanoid", games[0].Name)
}

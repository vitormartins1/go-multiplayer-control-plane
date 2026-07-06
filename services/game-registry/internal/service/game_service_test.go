package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-game-backend/services/game-registry/internal/domain"
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

func (m *mockRepo) UpdateName(ctx context.Context, id, name string) error {
	return m.Called(ctx, id, name).Error(0)
}
func (m *mockRepo) UpsertMultiplayerProfile(ctx context.Context, gameID string, p domain.MultiplayerProfile) error {
	return m.Called(ctx, gameID, p).Error(0)
}

func TestRename_Valido_AtualizaNomeMantemChave(t *testing.T) {
	repo := new(mockRepo)
	repo.On("UpdateName", mock.Anything, "g1", "Novo Nome").Return(nil)
	repo.On("FindByID", mock.Anything, "g1").Return(&domain.Game{ID: "g1", Slug: "orb", Name: "Novo Nome"}, nil)

	g, err := New(repo).Rename(context.Background(), "g1", "  Novo Nome  ")
	require.NoError(t, err)
	assert.Equal(t, "Novo Nome", g.Name)
	assert.Equal(t, "orb", g.Slug, "slug estável")
	repo.AssertExpectations(t)
}

func TestRename_NomeInvalido_ErrInvalidName(t *testing.T) {
	_, err := New(new(mockRepo)).Rename(context.Background(), "g1", "   ")
	assert.ErrorIs(t, err, domain.ErrInvalidName)
}

func TestRename_Inexistente_ErrGameNotFound(t *testing.T) {
	repo := new(mockRepo)
	repo.On("UpdateName", mock.Anything, "nao", "X").Return(domain.ErrGameNotFound)
	_, err := New(repo).Rename(context.Background(), "nao", "X")
	assert.ErrorIs(t, err, domain.ErrGameNotFound)
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

func TestRegister_AssociaImagemGerada(t *testing.T) {
	repo := new(mockRepo)
	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Game")).Return(nil)

	g, err := New(repo).Register(context.Background(), "tetris", "Tetris")
	require.NoError(t, err)
	assert.Equal(t, "generated", g.ImageSource)
	assert.Equal(t, "tetris", g.GeneratedImageSeed, "seed default = slug")
	assert.Equal(t, "shapes", g.GeneratedImageStyle)
	assert.Empty(t, g.Image, "sem upload no MVP")
}

func TestRegister_SeedDeterministicaPorSlug(t *testing.T) {
	repo := new(mockRepo)
	repo.On("Create", mock.Anything, mock.Anything).Return(nil)
	svc := New(repo)

	a1, _ := svc.Register(context.Background(), "tetris", "Tetris")
	a2, _ := svc.Register(context.Background(), "tetris", "Tetris de novo")
	b, _ := svc.Register(context.Background(), "arkanoid", "Arkanoid")

	assert.Equal(t, a1.GeneratedImageSeed, a2.GeneratedImageSeed, "mesmo slug -> mesma seed")
	assert.NotEqual(t, a1.GeneratedImageSeed, b.GeneratedImageSeed, "slugs diferentes -> seeds diferentes")
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

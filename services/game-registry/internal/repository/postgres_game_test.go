//go:build integration

// Testes de integração com Postgres real (testcontainers). Rodar com:
//
//	go test -tags=integration ./services/game-registry/internal/repository/
//
// Requer Docker em execução.
package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/c130n/game-backend/services/game-registry/internal/domain"
)

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	ctr, err := tcpostgres.Run(ctx, "postgres:16",
		tcpostgres.WithDatabase("game_registry"),
		tcpostgres.WithUsername("dev"),
		tcpostgres.WithPassword("dev"),
		tcpostgres.WithInitScripts(
			"../../migrations/001_create_games.sql",
			"../../migrations/004_add_cover_metadata.sql",
		),
		tcpostgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func newGame(slug, name string) *domain.Game {
	return &domain.Game{ID: uuid.NewString(), Slug: slug, Name: name, CreatedAt: time.Now().UTC()}
}

func TestPostgresGameRepository_CreateEFindByID(t *testing.T) {
	repo := NewPostgresGameRepository(newTestPool(t))
	ctx := context.Background()

	g := newGame("tetris", "Tetris")
	require.NoError(t, repo.Create(ctx, g))

	got, err := repo.FindByID(ctx, g.ID)
	require.NoError(t, err)
	assert.Equal(t, g.ID, got.ID)
	assert.Equal(t, "tetris", got.Slug)
	assert.Equal(t, "Tetris", got.Name)
}

func TestPostgresGameRepository_SlugDuplicadoCaseInsensitive_ErrSlugTaken(t *testing.T) {
	repo := NewPostgresGameRepository(newTestPool(t))
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, newGame("tetris", "Tetris")))
	err := repo.Create(ctx, newGame("TETRIS", "Outro Tetris")) // mesmo slug, case diferente
	assert.ErrorIs(t, err, domain.ErrSlugTaken)
}

func TestPostgresGameRepository_List_OrdenadoPorNome(t *testing.T) {
	repo := NewPostgresGameRepository(newTestPool(t))
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newGame("zelda", "Zelda")))
	require.NoError(t, repo.Create(ctx, newGame("arkanoid", "Arkanoid")))
	require.NoError(t, repo.Create(ctx, newGame("mario", "Mario")))

	got, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, []string{"Arkanoid", "Mario", "Zelda"}, []string{got[0].Name, got[1].Name, got[2].Name})
}

func TestPostgresGameRepository_FindByID_Inexistente_ErrGameNotFound(t *testing.T) {
	repo := NewPostgresGameRepository(newTestPool(t))
	_, err := repo.FindByID(context.Background(), "nao-existe")
	assert.ErrorIs(t, err, domain.ErrGameNotFound)
}

func TestPostgresGameRepository_CapaRoundTrip(t *testing.T) {
	repo := NewPostgresGameRepository(newTestPool(t))
	ctx := context.Background()

	g := newGame("tetris", "Tetris")
	g.ImageSource = "generated"
	g.GeneratedImageSeed = "tetris"
	g.GeneratedImageStyle = "shapes"
	require.NoError(t, repo.Create(ctx, g))

	got, err := repo.FindByID(ctx, g.ID)
	require.NoError(t, err)
	assert.Equal(t, "generated", got.ImageSource)
	assert.Equal(t, "tetris", got.GeneratedImageSeed)
	assert.Equal(t, "shapes", got.GeneratedImageStyle)
	assert.Empty(t, got.Image, "sem upload -> image vazio (NULLIF gravou NULL, COALESCE leu '')")
}

func TestPostgresGameRepository_RegistroAntigoSemSeed_CaiNoSlug(t *testing.T) {
	pool := newTestPool(t)
	repo := NewPostgresGameRepository(pool)
	ctx := context.Background()

	// simula um registro "antigo": insere direto com seed/image NULL.
	_, err := pool.Exec(ctx,
		`INSERT INTO games (id, slug, name, created_at, image, generated_image_seed)
		 VALUES ($1, 'legado', 'Jogo Legado', now(), NULL, NULL)`, "old-1")
	require.NoError(t, err)

	got, err := repo.FindByID(ctx, "old-1")
	require.NoError(t, err)
	assert.Equal(t, "legado", got.GeneratedImageSeed, "seed NULL -> slug (COALESCE)")
	assert.Equal(t, "generated", got.ImageSource, "default da migration")
	assert.Equal(t, "shapes", got.GeneratedImageStyle, "default da migration")
	assert.Empty(t, got.Image)
}

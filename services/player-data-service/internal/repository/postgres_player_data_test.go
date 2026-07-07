//go:build integration

// Testes de integração com Postgres real (testcontainers). Rodar com:
//
//	go test -tags=integration ./services/player-data-service/internal/repository/
//
// Requer Docker em execução.
package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/player-data-service/internal/domain"
)

const gA = "game-a"

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	ctr, err := tcpostgres.Run(ctx, "postgres:16",
		tcpostgres.WithDatabase("player_data"),
		tcpostgres.WithUsername("dev"),
		tcpostgres.WithPassword("dev"),
		tcpostgres.WithInitScripts(
			"../../migrations/001_create_player_data.sql",
			"../../migrations/002_add_game_scope.sql",
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

func TestPutGet_RoundTrip(t *testing.T) {
	repo := NewPostgresPlayerDataRepository(newTestPool(t))
	ctx := context.Background()

	require.NoError(t, repo.Put(ctx, gA, "p1", "inventory", "gold", []byte(`{"amount":100}`)))

	v, err := repo.Get(ctx, gA, "p1", "inventory", "gold")
	require.NoError(t, err)
	assert.JSONEq(t, `{"amount":100}`, string(v))
}

func TestPut_Sobrescreve(t *testing.T) {
	repo := NewPostgresPlayerDataRepository(newTestPool(t))
	ctx := context.Background()

	require.NoError(t, repo.Put(ctx, gA, "p1", "stats", "kills", []byte(`1`)))
	require.NoError(t, repo.Put(ctx, gA, "p1", "stats", "kills", []byte(`42`)))

	v, err := repo.Get(ctx, gA, "p1", "stats", "kills")
	require.NoError(t, err)
	assert.JSONEq(t, `42`, string(v))
}

func TestGet_Inexistente_ErrNotFound(t *testing.T) {
	repo := NewPostgresPlayerDataRepository(newTestPool(t))
	_, err := repo.Get(context.Background(), gA, "p1", "inventory", "fantasma")
	assert.True(t, errors.Is(err, domain.ErrNotFound))
}

func TestGetNamespace_RetornaTodosOsPares(t *testing.T) {
	repo := NewPostgresPlayerDataRepository(newTestPool(t))
	ctx := context.Background()

	require.NoError(t, repo.Put(ctx, gA, "p1", "settings", "sound", []byte(`true`)))
	require.NoError(t, repo.Put(ctx, gA, "p1", "settings", "lang", []byte(`"pt"`)))
	require.NoError(t, repo.Put(ctx, gA, "p1", "other", "x", []byte(`1`))) // namespace diferente

	m, err := repo.GetNamespace(ctx, gA, "p1", "settings")
	require.NoError(t, err)
	require.Len(t, m, 2)
	assert.JSONEq(t, `true`, string(m["sound"]))
	assert.JSONEq(t, `"pt"`, string(m["lang"]))
}

func TestDelete_Idempotente(t *testing.T) {
	repo := NewPostgresPlayerDataRepository(newTestPool(t))
	ctx := context.Background()

	require.NoError(t, repo.Put(ctx, gA, "p1", "inventory", "gold", []byte(`1`)))
	require.NoError(t, repo.Delete(ctx, gA, "p1", "inventory", "gold"))
	_, err := repo.Get(ctx, gA, "p1", "inventory", "gold")
	assert.True(t, errors.Is(err, domain.ErrNotFound))

	// remover inexistente não erra
	require.NoError(t, repo.Delete(ctx, gA, "p1", "inventory", "gold"))
}

func TestData_IsoladoPorJogador(t *testing.T) {
	repo := NewPostgresPlayerDataRepository(newTestPool(t))
	ctx := context.Background()

	require.NoError(t, repo.Put(ctx, gA, "p1", "inventory", "gold", []byte(`100`)))

	_, err := repo.Get(ctx, gA, "p2", "inventory", "gold")
	assert.True(t, errors.Is(err, domain.ErrNotFound))
}

func TestData_IsoladoPorJogo(t *testing.T) {
	repo := NewPostgresPlayerDataRepository(newTestPool(t))
	ctx := context.Background()

	// mesmo jogador/namespace/key, jogos diferentes -> dados independentes.
	require.NoError(t, repo.Put(ctx, "game-a", "p1", "inventory", "gold", []byte(`100`)))
	require.NoError(t, repo.Put(ctx, "game-b", "p1", "inventory", "gold", []byte(`5`)))

	a, err := repo.Get(ctx, "game-a", "p1", "inventory", "gold")
	require.NoError(t, err)
	assert.JSONEq(t, `100`, string(a))

	b, err := repo.Get(ctx, "game-b", "p1", "inventory", "gold")
	require.NoError(t, err)
	assert.JSONEq(t, `5`, string(b))

	// dado do jogo A não aparece no jogo C.
	_, err = repo.Get(ctx, "game-c", "p1", "inventory", "gold")
	assert.True(t, errors.Is(err, domain.ErrNotFound))
}

func TestListForPlayer_SoDoJogo(t *testing.T) {
	repo := NewPostgresPlayerDataRepository(newTestPool(t))
	ctx := context.Background()

	require.NoError(t, repo.Put(ctx, "game-a", "p1", "inventory", "gold", []byte(`100`)))
	require.NoError(t, repo.Put(ctx, "game-a", "p1", "stats", "kills", []byte(`10`)))
	require.NoError(t, repo.Put(ctx, "game-a", "p2", "inventory", "gold", []byte(`1`)))  // outro jogador
	require.NoError(t, repo.Put(ctx, "game-b", "p1", "inventory", "gold", []byte(`999`))) // outro jogo

	entries, err := repo.ListForPlayer(ctx, "game-a", "p1")
	require.NoError(t, err)
	require.Len(t, entries, 2)
	// ORDER BY namespace, key -> inventory/gold antes de stats/kills.
	assert.Equal(t, "inventory", entries[0].Namespace)
	assert.Equal(t, "gold", entries[0].Key)
	assert.JSONEq(t, `100`, string(entries[0].Value))
	assert.Equal(t, "stats", entries[1].Namespace)
	assert.JSONEq(t, `10`, string(entries[1].Value))
}

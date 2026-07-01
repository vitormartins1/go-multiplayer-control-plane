//go:build integration

// Testes de integração com Postgres real (testcontainers). Rodar com:
//
//	go test -tags=integration ./services/game-registry/internal/repository/
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

	"github.com/vitormartins1/go-game-backend/services/game-registry/internal/domain"
)

// newPresencePool sobe um Postgres com as migrations 001 (games) + 003
// (game_players) — a presença tem FK para games.
func newPresencePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	ctr, err := tcpostgres.Run(ctx, "postgres:16",
		tcpostgres.WithDatabase("game_registry"),
		tcpostgres.WithUsername("dev"),
		tcpostgres.WithPassword("dev"),
		tcpostgres.WithInitScripts(
			"../../migrations/001_create_games.sql",
			"../../migrations/003_create_game_players.sql",
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

func TestPresence_UpsertIdempotente_FirstSeenEstavel(t *testing.T) {
	pool := newPresencePool(t)
	games := NewPostgresGameRepository(pool)
	pres := NewPostgresPresenceRepository(pool)
	ctx := context.Background()

	g := newGame("tetris", "Tetris")
	require.NoError(t, games.Create(ctx, g))

	require.NoError(t, pres.Upsert(ctx, g.ID, "p1", "Alice"))
	first, err := pres.ListByGame(ctx, g.ID)
	require.NoError(t, err)
	require.Len(t, first, 1)
	seen0 := first[0].FirstSeen

	// re-marcar mantém first_seen e atualiza o handle.
	require.NoError(t, pres.Upsert(ctx, g.ID, "p1", "Alice_Renamed"))
	again, err := pres.ListByGame(ctx, g.ID)
	require.NoError(t, err)
	require.Len(t, again, 1)
	assert.Equal(t, "Alice_Renamed", again[0].Handle)
	assert.True(t, again[0].FirstSeen.Equal(seen0), "first_seen deve permanecer estável no re-upsert")
}

func TestPresence_JogoInexistente_ErrGameNotFound(t *testing.T) {
	pres := NewPostgresPresenceRepository(newPresencePool(t))
	err := pres.Upsert(context.Background(), "00000000-0000-0000-0000-000000000000", "p1", "Alice")
	assert.True(t, errors.Is(err, domain.ErrGameNotFound))
}

func TestPresence_GamesByPlayer_SoOsJogosDoPlayer(t *testing.T) {
	pool := newPresencePool(t)
	games := NewPostgresGameRepository(pool)
	pres := NewPostgresPresenceRepository(pool)
	ctx := context.Background()

	ga := newGame("game-a", "Game A")
	gb := newGame("game-b", "Game B")
	gc := newGame("game-c", "Game C")
	require.NoError(t, games.Create(ctx, ga))
	require.NoError(t, games.Create(ctx, gb))
	require.NoError(t, games.Create(ctx, gc))

	// p1 está em A e B; p2 só em C.
	require.NoError(t, pres.Upsert(ctx, ga.ID, "p1", "Alice"))
	require.NoError(t, pres.Upsert(ctx, gb.ID, "p1", "Alice"))
	require.NoError(t, pres.Upsert(ctx, gc.ID, "p2", "Bob"))

	got, err := pres.GamesByPlayer(ctx, "p1")
	require.NoError(t, err)
	require.Len(t, got, 2) // só A e B, por nome
	assert.Equal(t, "Game A", got[0].Name)
	assert.Equal(t, "Game B", got[1].Name)

	none, err := pres.GamesByPlayer(ctx, "fantasma")
	require.NoError(t, err)
	assert.Empty(t, none, "player sem presença -> nenhum jogo")
}

func TestPresence_ListByGame_IsoladoEOrdenado(t *testing.T) {
	pool := newPresencePool(t)
	games := NewPostgresGameRepository(pool)
	pres := NewPostgresPresenceRepository(pool)
	ctx := context.Background()

	ga := newGame("game-a", "Game A")
	gb := newGame("game-b", "Game B")
	require.NoError(t, games.Create(ctx, ga))
	require.NoError(t, games.Create(ctx, gb))

	require.NoError(t, pres.Upsert(ctx, ga.ID, "p1", "Alice"))
	require.NoError(t, pres.Upsert(ctx, ga.ID, "p2", "Bob"))
	require.NoError(t, pres.Upsert(ctx, gb.ID, "p3", "Carol")) // outro jogo

	base, err := pres.ListByGame(ctx, ga.ID)
	require.NoError(t, err)
	require.Len(t, base, 2) // só os do game-a
	assert.Equal(t, "p1", base[0].PlayerID)
	assert.Equal(t, "p2", base[1].PlayerID) // por first_seen asc
}

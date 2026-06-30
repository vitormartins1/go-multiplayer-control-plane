//go:build integration

// Testes de integração com MinIO real (testcontainers). Rodar com:
//
//	go test -tags=integration ./services/cloud-save-service/internal/storage/
//
// Requer Docker em execução.
package storage

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcminio "github.com/testcontainers/testcontainers-go/modules/minio"

	"github.com/c130n/game-backend/services/cloud-save-service/internal/domain"
)

func newTestStore(t *testing.T) *S3Store {
	t.Helper()
	ctx := context.Background()
	ctr, err := tcminio.Run(ctx, "minio/minio:latest")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

	endpoint, err := ctr.ConnectionString(ctx)
	require.NoError(t, err)
	store, err := NewS3Store(ctx, "http://"+endpoint, ctr.Username, ctr.Password, "cloud-save")
	require.NoError(t, err)
	return store
}

const gA = "game-a"

func TestPutGet_RoundTrip(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	snap := domain.Snapshot{Blob: []byte("save-bytes-\x00\x01"), ContentType: "application/json"}

	require.NoError(t, store.Put(ctx, gA, "p1", snap))

	got, err := store.Get(ctx, gA, "p1")
	require.NoError(t, err)
	assert.Equal(t, snap.Blob, got.Blob)
	assert.Equal(t, "application/json", got.ContentType)
	assert.False(t, got.UpdatedAt.IsZero(), "UpdatedAt deve vir do LastModified do S3")
}

func TestPut_Sobrescreve(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Put(ctx, gA, "p1", domain.Snapshot{Blob: []byte("v1"), ContentType: "text/plain"}))
	require.NoError(t, store.Put(ctx, gA, "p1", domain.Snapshot{Blob: []byte("v2"), ContentType: "text/plain"}))

	got, err := store.Get(ctx, gA, "p1")
	require.NoError(t, err)
	assert.Equal(t, []byte("v2"), got.Blob)
}

func TestGet_Inexistente_ErrNotFound(t *testing.T) {
	store := newTestStore(t)
	_, err := store.Get(context.Background(), gA, "fantasma")
	assert.True(t, errors.Is(err, domain.ErrNotFound))
}

func TestData_IsoladoPorJogador(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Put(ctx, gA, "p1", domain.Snapshot{Blob: []byte("p1-save"), ContentType: "text/plain"}))

	_, err := store.Get(ctx, gA, "p2")
	assert.True(t, errors.Is(err, domain.ErrNotFound))
}

func TestData_IsoladoPorJogo(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// mesmo jogador, jogos diferentes -> saves independentes (prefixo da key).
	require.NoError(t, store.Put(ctx, "game-a", "p1", domain.Snapshot{Blob: []byte("a-save"), ContentType: "text/plain"}))
	require.NoError(t, store.Put(ctx, "game-b", "p1", domain.Snapshot{Blob: []byte("b-save"), ContentType: "text/plain"}))

	a, err := store.Get(ctx, "game-a", "p1")
	require.NoError(t, err)
	assert.Equal(t, []byte("a-save"), a.Blob)

	b, err := store.Get(ctx, "game-b", "p1")
	require.NoError(t, err)
	assert.Equal(t, []byte("b-save"), b.Blob)

	// save do jogo A não aparece no jogo C.
	_, err = store.Get(ctx, "game-c", "p1")
	assert.True(t, errors.Is(err, domain.ErrNotFound))
}

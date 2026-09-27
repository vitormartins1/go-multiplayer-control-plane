//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/achievements-service/internal/domain"
)

// spec 019 — fila do espelhamento na Steam.

func TestSteamSync_ClaimPegaSoPendenteElegivel(t *testing.T) {
	pool := newTestPool(t)
	ach := NewPostgresAchievementRepository(pool)
	sync := NewPostgresSteamSyncRepository(pool)
	ctx := context.Background()
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)

	require.NoError(t, ach.CreateAchievement(ctx, &domain.Achievement{GameID: "g1", ID: "boss", Name: "Boss", EventType: "boss_killed", Target: 1, SteamAPIName: "ACH_BOSS"}))
	require.NoError(t, ach.CreateAchievement(ctx, &domain.Achievement{GameID: "g1", ID: "local", Name: "Local", EventType: "boss_killed", Target: 1})) // sem nome Steam
	require.NoError(t, ach.CreateAchievement(ctx, &domain.Achievement{GameID: "g1", ID: "dois", Name: "Dois", EventType: "boss_killed", Target: 2, SteamAPIName: "ACH_DOIS"}))
	require.NoError(t, ach.CreateAchievement(ctx, &domain.Achievement{GameID: "g2", ID: "boss", Name: "Boss", EventType: "boss_killed", Target: 1, SteamAPIName: "ACH_BOSS"}))
	for _, id := range []string{"boss", "local", "dois"} {
		_, err := ach.RecordProgress(ctx, "g1", "p1", id, map[string]int{"boss": 1, "local": 1, "dois": 2}[id], now)
		require.NoError(t, err)
	}
	_, err := ach.RecordProgress(ctx, "g2", "p1", "boss", 1, now)
	require.NoError(t, err)

	items, err := sync.ClaimSteamPending(ctx, "g1", 10, now, now.Add(2*time.Minute))
	require.NoError(t, err)
	require.Len(t, items, 1, "só o desbloqueio com nome Steam do jogo g1 (dois ainda está 1/2)")
	assert.Equal(t, domain.SteamSyncItem{GameID: "g1", PlayerID: "p1", AchievementID: "boss", SteamAPIName: "ACH_BOSS"}, items[0])

	again, err := sync.ClaimSteamPending(ctx, "g1", 10, now, now.Add(2*time.Minute))
	require.NoError(t, err)
	assert.Empty(t, again, "alugado: não volta antes do lease vencer")

	later, err := sync.ClaimSteamPending(ctx, "g1", 10, now.Add(3*time.Minute), now.Add(5*time.Minute))
	require.NoError(t, err)
	assert.Len(t, later, 1, "lease vencido: volta para a fila (worker morreu no meio)")
}

func TestSteamSync_MarcaSyncedERetry(t *testing.T) {
	pool := newTestPool(t)
	ach := NewPostgresAchievementRepository(pool)
	sync := NewPostgresSteamSyncRepository(pool)
	ctx := context.Background()
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)

	require.NoError(t, ach.CreateAchievement(ctx, &domain.Achievement{GameID: "g1", ID: "boss", Name: "Boss", EventType: "boss_killed", Target: 1, SteamAPIName: "ACH_BOSS"}))
	for _, p := range []string{"p1", "p2"} {
		_, err := ach.RecordProgress(ctx, "g1", p, "boss", 1, now)
		require.NoError(t, err)
	}
	items, err := sync.ClaimSteamPending(ctx, "g1", 10, now, now)
	require.NoError(t, err)
	require.Len(t, items, 2)

	require.NoError(t, sync.MarkSteamDone(ctx, items[0], domain.SteamSyncSynced, "", now))
	require.NoError(t, sync.MarkSteamRetry(ctx, items[1], now.Add(time.Minute), "steam 503"))

	got, err := sync.ClaimSteamPending(ctx, "g1", 10, now.Add(30*time.Second), now.Add(30*time.Second))
	require.NoError(t, err)
	assert.Empty(t, got, "synced sai da fila; retry só depois do next_attempt")

	got, err = sync.ClaimSteamPending(ctx, "g1", 10, now.Add(2*time.Minute), now.Add(2*time.Minute))
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, items[1].PlayerID, got[0].PlayerID)
	assert.Equal(t, 1, got[0].Attempts, "a tentativa foi contada")

	var state string
	var syncedAt *time.Time
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT steam_sync_state, steam_synced_at FROM player_achievements WHERE game_id='g1' AND player_id=$1 AND achievement_id='boss'`,
		items[0].PlayerID).Scan(&state, &syncedAt))
	assert.Equal(t, "synced", state)
	require.NotNil(t, syncedAt)
}

// Preencher o nome Steam depois espelha quem já tinha desbloqueado (backfill).
func TestSteamSync_NomeSteamDepoisFazBackfill(t *testing.T) {
	pool := newTestPool(t)
	ach := NewPostgresAchievementRepository(pool)
	sync := NewPostgresSteamSyncRepository(pool)
	ctx := context.Background()
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)

	a := &domain.Achievement{GameID: "g1", ID: "boss", Name: "Boss", EventType: "boss_killed", Target: 1}
	require.NoError(t, ach.CreateAchievement(ctx, a))
	_, err := ach.RecordProgress(ctx, "g1", "p1", "boss", 1, now)
	require.NoError(t, err)
	got, err := sync.ClaimSteamPending(ctx, "g1", 10, now, now)
	require.NoError(t, err)
	require.Empty(t, got)

	a.SteamAPIName = "ACH_BOSS"
	require.NoError(t, ach.UpdateAchievement(ctx, a))
	got, err = sync.ClaimSteamPending(ctx, "g1", 10, now, now)
	require.NoError(t, err)
	assert.Len(t, got, 1)
}

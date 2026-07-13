//go:build integration

// Testes de integração com Postgres real (testcontainers). Rodar com:
//
//	go test -tags=integration ./services/session-service/internal/repository/
//
// Requer Docker em execução.
package repository

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/domain"
)

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	ctr, err := tcpostgres.Run(ctx, "postgres:16",
		tcpostgres.WithDatabase("session"),
		tcpostgres.WithUsername("dev"),
		tcpostgres.WithPassword("dev"),
		tcpostgres.WithInitScripts(
			"../../migrations/001_create_closed_sessions.sql",
			"../../migrations/002_create_closed_session_players.sql",
			"../../migrations/003_closed_session_result_and_outcome.sql",
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

func closedSession(id, gameID string, closedAt time.Time, participants ...domain.Participant) *domain.Session {
	openedAt := closedAt.Add(-10 * time.Minute)
	return &domain.Session{
		ID:           id,
		GameID:       gameID,
		ModeID:       "default",
		Origin:       domain.OriginHostWorld,
		HostPlayerID: "host-1",
		CloseReason:  domain.CloseHostLeft,
		Profile:      &domain.ProfileSnapshot{Archetype: domain.ArchetypeHostWorldDropIn, Hash: "h"},
		Participants: participants,
		OpenedAt:     openedAt,
		ClosedAt:     closedAt,
	}
}

func TestPostgresHistoryRepository_SaveClosed_PersisteSessaoEParticipantes(t *testing.T) {
	pool := newTestPool(t)
	repo := NewPostgresHistoryRepository(pool)
	ctx := context.Background()

	closedAt := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)
	sess := closedSession("s1", "game-1", closedAt,
		domain.Participant{PlayerID: "host-1", Role: domain.RoleHost, JoinedAt: closedAt.Add(-10 * time.Minute)},
		domain.Participant{PlayerID: "coop-1", Role: domain.RoleCoop, JoinedAt: closedAt.Add(-5 * time.Minute)},
	)
	require.NoError(t, repo.SaveClosed(ctx, sess))

	got, err := repo.ClosedByGame(ctx, "game-1", 10, time.Time{})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "s1", got[0].ID)
	assert.Equal(t, domain.CloseHostLeft, got[0].CloseReason)
	require.Len(t, got[0].Participants, 2)
}

func TestPostgresHistoryRepository_ClosedByGame_PaginaPorClosedAtDesc(t *testing.T) {
	pool := newTestPool(t)
	repo := NewPostgresHistoryRepository(pool)
	ctx := context.Background()

	base := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)
	for i, id := range []string{"s1", "s2", "s3"} {
		require.NoError(t, repo.SaveClosed(ctx, closedSession(id, "game-1", base.Add(time.Duration(i)*time.Minute))))
	}

	page1, err := repo.ClosedByGame(ctx, "game-1", 2, time.Time{})
	require.NoError(t, err)
	require.Len(t, page1, 2)
	assert.Equal(t, "s3", page1[0].ID, "mais recente primeiro")
	assert.Equal(t, "s2", page1[1].ID)

	page2, err := repo.ClosedByGame(ctx, "game-1", 2, page1[len(page1)-1].ClosedAt)
	require.NoError(t, err)
	require.Len(t, page2, 1)
	assert.Equal(t, "s1", page2[0].ID, "cursor exclui o que já veio na página 1")
}

func TestPostgresHistoryRepository_ClosedByGame_NaoVazaOutroJogo(t *testing.T) {
	pool := newTestPool(t)
	repo := NewPostgresHistoryRepository(pool)
	ctx := context.Background()

	now := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)
	require.NoError(t, repo.SaveClosed(ctx, closedSession("s1", "game-1", now)))
	require.NoError(t, repo.SaveClosed(ctx, closedSession("s2", "game-2", now)))

	got, err := repo.ClosedByGame(ctx, "game-1", 10, time.Time{})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "s1", got[0].ID)
}

func TestPostgresHistoryRepository_ClosedByPlayer_SoSessoesQueParticipou(t *testing.T) {
	pool := newTestPool(t)
	repo := NewPostgresHistoryRepository(pool)
	ctx := context.Background()

	now := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)
	require.NoError(t, repo.SaveClosed(ctx, closedSession("s1", "game-1", now,
		domain.Participant{PlayerID: "host-1", Role: domain.RoleHost, JoinedAt: now},
		domain.Participant{PlayerID: "coop-1", Role: domain.RoleCoop, JoinedAt: now},
	)))
	require.NoError(t, repo.SaveClosed(ctx, closedSession("s2", "game-1", now.Add(time.Minute),
		domain.Participant{PlayerID: "host-1", Role: domain.RoleHost, JoinedAt: now},
	)))

	got, err := repo.ClosedByPlayer(ctx, "game-1", "coop-1", 10, time.Time{})
	require.NoError(t, err)
	require.Len(t, got, 1, "coop-1 só participou de s1")
	assert.Equal(t, "s1", got[0].ID)
}

// TestPostgresHistoryRepository_SaveClosed_PersisteResultETeams (spec 027):
// os dois achados centrais da revisão — Result e TeamByPlayer nunca
// sobreviviam ao fechamento da sessão.
func TestPostgresHistoryRepository_SaveClosed_PersisteResultETeams(t *testing.T) {
	pool := newTestPool(t)
	repo := NewPostgresHistoryRepository(pool)
	ctx := context.Background()

	closedAt := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)
	sess := closedSession("s1", "game-1", closedAt,
		domain.Participant{PlayerID: "p1", Role: "team:blue", JoinedAt: closedAt.Add(-5 * time.Minute)},
	)
	sess.Result = &domain.MatchOutcome{
		Kind: "win_loss_draw",
		WinLossDraw: &domain.WinLossDrawResult{
			Players: []domain.PlayerOutcome{{PlayerID: "p1", TeamID: "blue", Outcome: "win", Score: 10}},
		},
	}
	sess.TeamByPlayer = map[string]string{"p1": "blue"}
	require.NoError(t, repo.SaveClosed(ctx, sess))

	got, err := repo.ClosedByGame(ctx, "game-1", 10, time.Time{})
	require.NoError(t, err)
	require.Len(t, got, 1)

	require.NotNil(t, got[0].Result)
	require.NotNil(t, got[0].Result.WinLossDraw)
	require.Len(t, got[0].Result.WinLossDraw.Players, 1)
	assert.Equal(t, "p1", got[0].Result.WinLossDraw.Players[0].PlayerID)
	assert.Equal(t, "win", got[0].Result.WinLossDraw.Players[0].Outcome)
	assert.Equal(t, int64(10), got[0].Result.WinLossDraw.Players[0].Score)

	assert.Equal(t, map[string]string{"p1": "blue"}, got[0].TeamByPlayer)
}

// TestPostgresHistoryRepository_SaveClosed_SemResultadoNemTimes_FicaNil cobre
// o caminho oposto: sessão fechada sem ReportResult (ttl/admin/abandoned/
// host_left/host_killed) e sem times (FFA/host_world) -> NULL, não erro.
func TestPostgresHistoryRepository_SaveClosed_SemResultadoNemTimes_FicaNil(t *testing.T) {
	pool := newTestPool(t)
	repo := NewPostgresHistoryRepository(pool)
	ctx := context.Background()

	closedAt := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)
	require.NoError(t, repo.SaveClosed(ctx, closedSession("s1", "game-1", closedAt)))

	got, err := repo.ClosedByGame(ctx, "game-1", 10, time.Time{})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Nil(t, got[0].Result)
	assert.Nil(t, got[0].TeamByPlayer)
}

// TestPostgresHistoryRepository_SaveClosed_PersisteLastEventLastOutcome
// (spec 027): outcome por participante do host_world_dropin sobrevive ao
// fechamento.
func TestPostgresHistoryRepository_SaveClosed_PersisteLastEventLastOutcome(t *testing.T) {
	pool := newTestPool(t)
	repo := NewPostgresHistoryRepository(pool)
	ctx := context.Background()

	closedAt := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)
	sess := closedSession("s1", "game-1", closedAt,
		domain.Participant{PlayerID: "coop-1", Role: domain.RoleCoop, JoinedAt: closedAt.Add(-5 * time.Minute),
			LastEvent: domain.EventDied, LastOutcome: domain.OutcomeFail},
		domain.Participant{PlayerID: "host-1", Role: domain.RoleHost, JoinedAt: closedAt.Add(-10 * time.Minute)},
	)
	require.NoError(t, repo.SaveClosed(ctx, sess))

	got, err := repo.ClosedByGame(ctx, "game-1", 10, time.Time{})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Len(t, got[0].Participants, 2)

	byID := map[string]domain.Participant{}
	for _, p := range got[0].Participants {
		byID[p.PlayerID] = p
	}
	assert.Equal(t, domain.EventDied, byID["coop-1"].LastEvent)
	assert.Equal(t, domain.OutcomeFail, byID["coop-1"].LastOutcome)
	assert.Empty(t, byID["host-1"].LastEvent, "host nunca reporta evento sobre si mesmo")
}

func TestPostgresHistoryRepository_ClosedByPlayer_SemHistorico_ListaVazia(t *testing.T) {
	pool := newTestPool(t)
	repo := NewPostgresHistoryRepository(pool)
	ctx := context.Background()

	got, err := repo.ClosedByPlayer(ctx, "game-1", "ninguem", 10, time.Time{})
	require.NoError(t, err)
	assert.Empty(t, got)
}

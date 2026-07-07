package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-game-backend/services/analytics-service/internal/domain"
	"github.com/vitormartins1/go-game-backend/shared/logger"
)

// fakeRepo conta as agregações de forma thread-safe (o pool chama de N goroutines).
type fakeRepo struct {
	applied      atomic.Int64
	participated atomic.Int64
	mu           sync.Mutex
	byGame       map[string]int64
	err          error
}

func newFakeRepo() *fakeRepo { return &fakeRepo{byGame: map[string]int64{}} }

func (f *fakeRepo) ApplyMatch(_ context.Context, m domain.MatchResult) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	f.applied.Add(1)
	f.mu.Lock()
	f.byGame[m.GameID]++
	f.mu.Unlock()
	return false, nil
}
func (f *fakeRepo) ApplyParticipation(_ context.Context, evt domain.ParticipationEvent) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	f.participated.Add(1)
	f.mu.Lock()
	f.byGame[evt.GameID]++
	f.mu.Unlock()
	return false, nil
}
func (f *fakeRepo) Overview(context.Context) (domain.Overview, error) { return domain.Overview{}, nil }
func (f *fakeRepo) GameAnalytics(context.Context, string, int) (domain.GameAnalytics, error) {
	return domain.GameAnalytics{}, nil
}

// SubmitParticipation roteia para ApplyParticipation (não para ApplyMatch).
func TestPool_SubmitParticipation_RoteiaParaApplyParticipation(t *testing.T) {
	repo := newFakeRepo()
	pool := NewPool(repo, 2, 4, logger.New("test"), nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool.Start(ctx)

	done := pool.SubmitParticipation(ctx, domain.ParticipationEvent{GameID: "g1", SessionID: "w1", PlayerID: "inv-1", Role: "invader", Event: "HOST_KILLED"})
	require.NoError(t, <-done)
	pool.Stop()

	assert.Equal(t, int64(1), repo.participated.Load())
	assert.Equal(t, int64(0), repo.applied.Load(), "não passou pelo caminho de match")
}

// N partidas submetidas concorrentemente são todas agregadas, sem corrida.
func TestPool_SubmitConcorrente_AgregaTodas(t *testing.T) {
	repo := newFakeRepo()
	pool := NewPool(repo, 4, 8, logger.New("test"), nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool.Start(ctx)

	const n = 200
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			done := pool.Submit(ctx, domain.MatchResult{MatchID: "m", GameID: "g1"})
			require.NoError(t, <-done)
		}(i)
	}
	wg.Wait()
	pool.Stop()

	assert.Equal(t, int64(n), repo.applied.Load())
	assert.Equal(t, int64(n), repo.byGame["g1"])
}

// Erro do repo propaga pelo channel done (o consumidor não deletaria a mensagem).
func TestPool_ErroDoRepo_PropagaNoDone(t *testing.T) {
	repo := &fakeRepo{err: errors.New("db fora")}
	pool := NewPool(repo, 2, 4, logger.New("test"), nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool.Start(ctx)

	err := <-pool.Submit(ctx, domain.MatchResult{MatchID: "m", GameID: "g1"})
	require.Error(t, err)
	pool.Stop()
}

// Stop drena os jobs já enfileirados antes de encerrar (nenhum se perde).
func TestPool_Stop_DrenaPendentes(t *testing.T) {
	repo := newFakeRepo()
	pool := NewPool(repo, 1, 64, logger.New("test"), nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool.Start(ctx)

	dones := make([]<-chan error, 0, 50)
	for i := 0; i < 50; i++ {
		dones = append(dones, pool.Submit(ctx, domain.MatchResult{MatchID: "m", GameID: "g1"}))
	}
	for _, d := range dones {
		require.NoError(t, <-d)
	}
	pool.Stop()
	assert.Equal(t, int64(50), repo.applied.Load())
}

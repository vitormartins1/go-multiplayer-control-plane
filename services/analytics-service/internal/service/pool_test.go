package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/analytics-service/internal/domain"
	"github.com/vitormartins1/go-multiplayer-control-plane/shared/logger"
)

// fakeRepo conta as agregações de forma thread-safe (o pool chama de N goroutines).
type fakeRepo struct {
	applied      atomic.Int64
	participated atomic.Int64
	placed       atomic.Int64
	collected    atomic.Int64
	closed       atomic.Int64
	mu           sync.Mutex
	byGame       map[string]int64
	err          error
	dup          bool // simula reentrega (idempotência do repo)
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
	return f.dup, nil
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
func (f *fakeRepo) ApplyPlacement(_ context.Context, pr domain.PlacementResult) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	f.placed.Add(1)
	f.mu.Lock()
	f.byGame[pr.GameID]++
	f.mu.Unlock()
	return false, nil
}
func (f *fakeRepo) ApplyCollective(_ context.Context, cr domain.CollectiveResult) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	f.collected.Add(1)
	f.mu.Lock()
	f.byGame[cr.GameID]++
	f.mu.Unlock()
	return false, nil
}
func (f *fakeRepo) ApplySessionClosed(_ context.Context, evt domain.SessionCloseEvent) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	f.closed.Add(1)
	f.mu.Lock()
	f.byGame[evt.GameID]++
	f.mu.Unlock()
	return false, nil
}
func (f *fakeRepo) Overview(context.Context) (domain.Overview, error) { return domain.Overview{}, nil }
func (f *fakeRepo) GameAnalytics(context.Context, string, int, string) (domain.GameAnalytics, error) {
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

// SubmitPlacement roteia para ApplyPlacement (não para match/participation).
func TestPool_SubmitPlacement_RoteiaParaApplyPlacement(t *testing.T) {
	repo := newFakeRepo()
	pool := NewPool(repo, 2, 4, logger.New("test"), nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool.Start(ctx)

	done := pool.SubmitPlacement(ctx, domain.PlacementResult{GameID: "g1", MatchID: "m1", Entries: []domain.PlacementEntry{{Placement: 1, SurvivedMs: 900000}}})
	require.NoError(t, <-done)
	pool.Stop()

	assert.Equal(t, int64(1), repo.placed.Load())
	assert.Equal(t, int64(0), repo.applied.Load(), "não passou pelo caminho de match")
	assert.Equal(t, int64(0), repo.participated.Load())
}

// SubmitCollective roteia para ApplyCollective (não para os demais caminhos).
func TestPool_SubmitCollective_RoteiaParaApplyCollective(t *testing.T) {
	repo := newFakeRepo()
	pool := NewPool(repo, 2, 4, logger.New("test"), nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool.Start(ctx)

	done := pool.SubmitCollective(ctx, domain.CollectiveResult{GameID: "g1", MatchID: "m1", Outcome: domain.MissionSuccess})
	require.NoError(t, <-done)
	pool.Stop()

	assert.Equal(t, int64(1), repo.collected.Load())
	assert.Equal(t, int64(0), repo.applied.Load(), "não passou pelo caminho de match")
	assert.Equal(t, int64(0), repo.placed.Load())
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

// SubmitSessionClosed roteia para ApplySessionClosed (spec 013).
func TestPool_SubmitSessionClosed_RoteiaParaApplySessionClosed(t *testing.T) {
	repo := newFakeRepo()
	pool := NewPool(repo, 2, 4, logger.New("test"), nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool.Start(ctx)

	done := pool.SubmitSessionClosed(ctx, domain.SessionCloseEvent{SessionID: "s1", GameID: "g1", CloseReason: "result"})
	require.NoError(t, <-done)
	pool.Stop()

	assert.Equal(t, int64(1), repo.closed.Load())
	assert.Equal(t, int64(0), repo.applied.Load(), "não passou pelo caminho de match")
	assert.Equal(t, int64(0), repo.collected.Load())
}

// Com o ctx já cancelado (shutdown), todo Submit* devolve ctx.Err() sem
// enfileirar — o pool nem precisa estar de pé.
func TestPool_SubmitComCtxCancelado_DevolveErroSemEnfileirar(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	submits := map[string]func(*Pool) <-chan error{
		"match":         func(p *Pool) <-chan error { return p.Submit(ctx, domain.MatchResult{}) },
		"participation": func(p *Pool) <-chan error { return p.SubmitParticipation(ctx, domain.ParticipationEvent{}) },
		"placement":     func(p *Pool) <-chan error { return p.SubmitPlacement(ctx, domain.PlacementResult{}) },
		"collective":    func(p *Pool) <-chan error { return p.SubmitCollective(ctx, domain.CollectiveResult{}) },
		"sessionClosed": func(p *Pool) <-chan error { return p.SubmitSessionClosed(ctx, domain.SessionCloseEvent{}) },
	}
	for name, submit := range submits {
		t.Run(name, func(t *testing.T) {
			pool := NewPool(newFakeRepo(), 1, 0, logger.New("test"), nil) // buffer 0 e sem workers: só o ctx libera
			assert.ErrorIs(t, <-submit(pool), context.Canceled)
		})
	}
}

// NewPool com workers < 1 sobe 1 worker (nunca um pool parado).
func TestNewPool_WorkersInvalido_UsaUm(t *testing.T) {
	pool := NewPool(newFakeRepo(), 0, 1, logger.New("test"), nil)
	assert.Equal(t, 1, pool.workers)
}

// As métricas RED do ingest contam recebidos e processados por resultado
// (ok/dup/error) — são o que alimenta o dashboard Analytics Worker Pool.
func TestPool_Metricas_ContamRecebidosEResultado(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewMetrics(reg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ok := newFakeRepo()
	pool := NewPool(ok, 1, 4, logger.New("test"), metrics)
	pool.Start(ctx)
	require.NoError(t, <-pool.Submit(ctx, domain.MatchResult{GameID: "g1"}))
	pool.Stop()

	dup := newFakeRepo()
	dup.dup = true
	pool = NewPool(dup, 1, 4, logger.New("test"), metrics)
	pool.Start(ctx)
	require.NoError(t, <-pool.Submit(ctx, domain.MatchResult{GameID: "g1"}))
	pool.Stop()

	failing := &fakeRepo{err: errors.New("db fora")}
	pool = NewPool(failing, 1, 4, logger.New("test"), metrics)
	pool.Start(ctx)
	require.Error(t, <-pool.Submit(ctx, domain.MatchResult{GameID: "g1"}))
	pool.Stop()

	assert.Equal(t, float64(3), testutil.ToFloat64(metrics.received))
	assert.Equal(t, float64(1), testutil.ToFloat64(metrics.processed.WithLabelValues("ok")))
	assert.Equal(t, float64(1), testutil.ToFloat64(metrics.processed.WithLabelValues("dup")))
	assert.Equal(t, float64(1), testutil.ToFloat64(metrics.processed.WithLabelValues("error")))
	assert.Equal(t, float64(0), testutil.ToFloat64(metrics.workersActive), "gauge volta a zero ao fim de cada job")
}

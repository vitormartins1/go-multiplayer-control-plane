// Package service é o core do analytics-service: um worker pool que agrega as
// partidas ingeridas do fan-out (ADR-0021). O consumidor SQS (driving adapter)
// faz Submit; N workers puxam do channel e agregam via o repositório. O
// resultado de cada partida volta por um channel próprio, para o consumidor
// deletar a mensagem só no sucesso (at-least-once → DLQ em falha).
package service

import (
	"context"
	"log/slog"
	"sync"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/analytics-service/internal/domain"
)

var _ domain.Ingestor = (*Pool)(nil)

// job é uma unidade a agregar + o channel de retorno do resultado. É polimórfica
// (specs 004/005/006): collective não-nil => desfecho de missão (coop_mission);
// senão placement não-nil => colocação (battle_royale); senão participation
// não-nil => participação (host_world_dropin); senão => o match (queue_arena).
type job struct {
	match         domain.MatchResult
	participation *domain.ParticipationEvent
	placement     *domain.PlacementResult
	collective    *domain.CollectiveResult
	sessionClosed *domain.SessionCloseEvent
	done          chan error
}

// Pool é o worker pool de agregação (o core de concorrência do serviço).
type Pool struct {
	repo    domain.MatchRepository
	jobs    chan job
	workers int
	wg      sync.WaitGroup
	log     *slog.Logger
	metrics *Metrics
}

// NewPool cria o pool. `workers` = goroutines; `buffer` = capacidade do channel
// (contrapressão: Submit bloqueia quando cheio).
func NewPool(repo domain.MatchRepository, workers, buffer int, log *slog.Logger, metrics *Metrics) *Pool {
	if workers < 1 {
		workers = 1
	}
	return &Pool{
		repo:    repo,
		jobs:    make(chan job, buffer),
		workers: workers,
		log:     log,
		metrics: metrics,
	}
}

// Start sobe os workers. Eles rodam até o ctx cancelar ou Stop fechar o channel.
func (p *Pool) Start(ctx context.Context) {
	p.log.Info("worker pool iniciando", slog.Int("workers", p.workers))
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(ctx)
	}
}

func (p *Pool) worker(ctx context.Context) {
	defer p.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case j, ok := <-p.jobs:
			if !ok {
				return // channel fechado (Stop): drena o que já estava na fila e sai
			}
			p.process(ctx, j)
		}
	}
}

func (p *Pool) process(ctx context.Context, j job) {
	if p.metrics != nil {
		p.metrics.workersActive.Inc()
		defer p.metrics.workersActive.Dec()
	}
	var (
		dup bool
		err error
	)
	switch {
	case j.collective != nil:
		dup, err = p.repo.ApplyCollective(ctx, *j.collective)
	case j.placement != nil:
		dup, err = p.repo.ApplyPlacement(ctx, *j.placement)
	case j.participation != nil:
		dup, err = p.repo.ApplyParticipation(ctx, *j.participation)
	case j.sessionClosed != nil:
		dup, err = p.repo.ApplySessionClosed(ctx, *j.sessionClosed)
	default:
		dup, err = p.repo.ApplyMatch(ctx, j.match)
	}
	p.observe(dup, err)
	j.done <- err
}

// Submit entrega uma partida ao pool e devolve o channel com o resultado. O
// consumidor espera nesse channel para deletar (ou não) a mensagem. Respeita o
// cancelamento do ctx (shutdown) enquanto tenta enfileirar.
func (p *Pool) Submit(ctx context.Context, m domain.MatchResult) <-chan error {
	done := make(chan error, 1)
	if p.metrics != nil {
		p.metrics.received.Inc()
	}
	select {
	case <-ctx.Done():
		done <- ctx.Err()
	case p.jobs <- job{match: m, done: done}:
	}
	return done
}

// SubmitParticipation entrega um evento de participação ao pool (spec 004), pelo
// mesmo mecanismo de fan-in (channel de retorno) do Submit.
func (p *Pool) SubmitParticipation(ctx context.Context, evt domain.ParticipationEvent) <-chan error {
	done := make(chan error, 1)
	if p.metrics != nil {
		p.metrics.received.Inc()
	}
	select {
	case <-ctx.Done():
		done <- ctx.Err()
	case p.jobs <- job{participation: &evt, done: done}:
	}
	return done
}

// SubmitPlacement entrega um resultado por colocação ao pool (spec 005), pelo
// mesmo mecanismo de fan-in (channel de retorno) do Submit.
func (p *Pool) SubmitPlacement(ctx context.Context, pr domain.PlacementResult) <-chan error {
	done := make(chan error, 1)
	if p.metrics != nil {
		p.metrics.received.Inc()
	}
	select {
	case <-ctx.Done():
		done <- ctx.Err()
	case p.jobs <- job{placement: &pr, done: done}:
	}
	return done
}

// SubmitCollective entrega um desfecho de missão ao pool (spec 006), pelo mesmo
// mecanismo de fan-in (channel de retorno) do Submit.
func (p *Pool) SubmitCollective(ctx context.Context, cr domain.CollectiveResult) <-chan error {
	done := make(chan error, 1)
	if p.metrics != nil {
		p.metrics.received.Inc()
	}
	select {
	case <-ctx.Done():
		done <- ctx.Err()
	case p.jobs <- job{collective: &cr, done: done}:
	}
	return done
}

// SubmitSessionClosed entrega um fechamento de sessão ao pool (spec 013),
// pelo mesmo mecanismo de fan-in (channel de retorno) do Submit.
func (p *Pool) SubmitSessionClosed(ctx context.Context, evt domain.SessionCloseEvent) <-chan error {
	done := make(chan error, 1)
	if p.metrics != nil {
		p.metrics.received.Inc()
	}
	select {
	case <-ctx.Done():
		done <- ctx.Err()
	case p.jobs <- job{sessionClosed: &evt, done: done}:
	}
	return done
}

// Stop fecha o channel de jobs e espera os workers drenarem e encerrarem. Deve
// ser chamado DEPOIS de parar o consumidor (senão haveria Submit em pool fechado).
func (p *Pool) Stop() {
	close(p.jobs)
	p.wg.Wait()
	p.log.Info("worker pool encerrado")
}

func (p *Pool) observe(dup bool, err error) {
	if p.metrics == nil {
		return
	}
	result := "ok"
	switch {
	case err != nil:
		result = "error"
	case dup:
		result = "dup"
	}
	p.metrics.processed.WithLabelValues(result).Inc()
}

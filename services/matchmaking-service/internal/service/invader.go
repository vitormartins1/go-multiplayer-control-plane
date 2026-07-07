package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/matchmaking-service/internal/domain"
)

var _ domain.Invader = (*Invader)(nil)

// invadeTimeout limita cada chamada ao session (list/inject) na orquestração da
// invasão. A invasão é stateless e síncrona — o cliente espera a resposta.
const invadeTimeout = 5 * time.Second

// roleInvader é o papel que o matchmaking ocupa ao injetar (vocabulário do
// host_world_dropin, session/011). O co-op (roleCoop) é H1b.
const roleInvader = "invader"

// Rótulos do contador invasions_total{result} (spec 006, observabilidade).
const (
	resultInjected     = "injected"
	resultNoWorld      = "no_world"
	resultNotSupported = "not_supported"
)

// Invader é o segundo modo do matchmaking (spec 006): descobre um mundo aberto
// elegível e injeta o invasor nele. Stateless — sem fila, sem match. Orquestra
// três Ports de saída: política (game-registry), diretório e injetor (session).
type Invader struct {
	policies  domain.InvasionPolicyProvider
	directory domain.WorldDirectory
	injector  domain.WorldInjector

	mu      sync.Mutex
	results map[string]int64 // result -> contador (injected/no_world/not_supported)
}

// NewInvader cria o serviço de invasão a partir das três Ports de saída.
func NewInvader(policies domain.InvasionPolicyProvider, directory domain.WorldDirectory, injector domain.WorldInjector) *Invader {
	return &Invader{
		policies:  policies,
		directory: directory,
		injector:  injector,
		results:   make(map[string]int64),
	}
}

// Invade lê a política do jogo, lista os mundos invadíveis, filtra por faixa
// (domain.Eligible) e injeta o invasor no primeiro elegível. A corrida
// listar→injetar (mundo encheu: ErrRoleFull) ou o mundo que fechou
// (ErrWorldNotFound) apenas descartam o candidato e seguem para o próximo.
func (in *Invader) Invade(ctx context.Context, gameID, modeID, playerID string, crit domain.InvaderCriteria) (domain.Invaded, error) {
	policy, err := in.policyOf(ctx, gameID, modeID)
	if err != nil {
		return domain.Invaded{}, err // ErrProfileUnavailable propaga
	}
	if !policy.SupportsInvasion() {
		in.count(resultNotSupported)
		return domain.Invaded{}, fmt.Errorf("invader: %w", domain.ErrInvasionNotSupported)
	}

	worlds, err := in.listWorlds(ctx, gameID)
	if err != nil {
		return domain.Invaded{}, fmt.Errorf("invader: list worlds: %w", err)
	}

	for _, w := range worlds {
		if !domain.Eligible(crit, w, policy) {
			continue
		}
		injected, err := in.inject(ctx, w.SessionID, playerID)
		switch {
		case err == nil:
			in.count(resultInjected)
			return domain.Invaded{SessionID: w.SessionID, ServerAddress: injected.ServerAddress, ConnectToken: injected.ConnectToken}, nil
		case errors.Is(err, domain.ErrRoleFull), errors.Is(err, domain.ErrWorldNotFound):
			continue // corrida: mundo encheu/fechou entre listar e injetar → tenta o próximo
		default:
			return domain.Invaded{}, fmt.Errorf("invader: inject: %w", err)
		}
	}

	in.count(resultNoWorld)
	return domain.Invaded{}, fmt.Errorf("invader: %w", domain.ErrNoEligibleWorld)
}

// policyOf lê a política com timeout próprio.
func (in *Invader) policyOf(ctx context.Context, gameID, modeID string) (domain.InvasionPolicy, error) {
	ctx, cancel := context.WithTimeout(ctx, invadeTimeout)
	defer cancel()
	return in.policies.InvasionPolicy(ctx, gameID, modeID)
}

// listWorlds lista os mundos invadíveis com timeout próprio.
func (in *Invader) listWorlds(ctx context.Context, gameID string) ([]domain.InvadableWorld, error) {
	ctx, cancel := context.WithTimeout(ctx, invadeTimeout)
	defer cancel()
	return in.directory.ListInvadableWorlds(ctx, gameID)
}

// inject injeta o invasor num mundo com timeout próprio.
func (in *Invader) inject(ctx context.Context, sessionID, playerID string) (domain.Injected, error) {
	ctx, cancel := context.WithTimeout(ctx, invadeTimeout)
	defer cancel()
	return in.injector.Inject(ctx, sessionID, playerID, roleInvader)
}

// count incrementa o contador do resultado (métrica invasions_total{result}).
func (in *Invader) count(result string) {
	in.mu.Lock()
	defer in.mu.Unlock()
	in.results[result]++
}

// InvasionsTotal devolve o total de invasões com o resultado (métrica
// invasions_total{result} — registrada como CounterFunc no main).
func (in *Invader) InvasionsTotal(result string) float64 {
	in.mu.Lock()
	defer in.mu.Unlock()
	return float64(in.results[result])
}

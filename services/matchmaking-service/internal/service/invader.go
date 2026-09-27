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

// invadeListAttempts limita quantas vezes Invade rebusca ListInvadableWorlds
// dentro de uma mesma chamada (achado no /code-review — "JoinWorld race"): se
// TODOS os candidatos da 1ª lista perderem a corrida listar→injetar (mundo
// encheu/fechou), um mundo elegível pode ter aberto nesse meio-tempo. Sem um
// refetch, isso é um falso-negativo: ErrNoEligibleWorld mesmo havendo mundo
// disponível. Bounded a 2 (não é retry indefinido) e só dispara quando a
// lista teve candidatos que perderam a corrida — lista vazia não é corrida,
// não vale re-tentar (evita dobrar a latência do caso comum "sem mundos").
const invadeListAttempts = 2

// Invade lê a política do jogo, lista os mundos invadíveis, filtra por faixa
// (domain.Eligible) e injeta o invasor no primeiro elegível. A corrida
// listar→injetar (mundo encheu: ErrRoleFull) ou o mundo que fechou
// (ErrWorldNotFound) apenas descartam o candidato e seguem para o próximo;
// ver invadeListAttempts para o que acontece quando a lista inteira perde.
func (in *Invader) Invade(ctx context.Context, gameID, modeID, playerID string, crit domain.InvaderCriteria) (domain.Invaded, error) {
	policy, err := in.policyOf(ctx, gameID, modeID)
	if err != nil {
		return domain.Invaded{}, err // ErrProfileUnavailable propaga
	}
	if !policy.SupportsInvasion() {
		in.count(resultNotSupported)
		return domain.Invaded{}, fmt.Errorf("invader: %w", domain.ErrInvasionNotSupported)
	}

	for attempt := 0; attempt < invadeListAttempts; attempt++ {
		worlds, err := in.listWorlds(ctx, gameID, modeID)
		if err != nil {
			return domain.Invaded{}, fmt.Errorf("invader: list worlds: %w", err)
		}
		invaded, injected, err := in.tryInject(ctx, worlds, playerID, crit, policy)
		if err != nil {
			return domain.Invaded{}, err
		}
		if injected {
			in.count(resultInjected)
			return invaded, nil
		}
		if len(worlds) == 0 {
			break // sem candidatos: não é corrida, não vale rebuscar
		}
	}

	in.count(resultNoWorld)
	return domain.Invaded{}, fmt.Errorf("invader: %w", domain.ErrNoEligibleWorld)
}

// tryInject percorre os mundos elegíveis (faixa ou bypass de senha) e tenta
// injetar no primeiro que aceitar. ErrRoleFull/ErrWorldNotFound descartam o
// candidato (corrida listar→injetar) e seguem para o próximo, sem propagar
// erro — só um erro inesperado (default) interrompe a busca.
func (in *Invader) tryInject(ctx context.Context, worlds []domain.InvadableWorld, playerID string, crit domain.InvaderCriteria, policy domain.InvasionPolicy) (domain.Invaded, bool, error) {
	for _, w := range worlds {
		// Bypass por senha (follow-up #1, matchmaking/011): senha bate → elegível
		// independente da faixa. Eligible continua só faixa, inalterada.
		matchesPassword := policy.PasswordEnabled && crit.Password != "" && w.PasswordHash != "" &&
			domain.HashWorldPassword(crit.Password) == w.PasswordHash
		if !matchesPassword && !domain.Eligible(crit, w, policy) {
			continue
		}
		injected, err := in.inject(ctx, w.SessionID, playerID)
		switch {
		case err == nil:
			return domain.Invaded{SessionID: w.SessionID, ServerAddress: injected.ServerAddress, HostConnection: injected.HostConnection, ConnectToken: injected.ConnectToken}, true, nil
		case errors.Is(err, domain.ErrRoleFull), errors.Is(err, domain.ErrWorldNotFound):
			continue // corrida: mundo encheu/fechou entre listar e injetar → tenta o próximo
		default:
			return domain.Invaded{}, false, fmt.Errorf("invader: inject: %w", err)
		}
	}
	return domain.Invaded{}, false, nil
}

// policyOf lê a política com timeout próprio.
func (in *Invader) policyOf(ctx context.Context, gameID, modeID string) (domain.InvasionPolicy, error) {
	ctx, cancel := context.WithTimeout(ctx, invadeTimeout)
	defer cancel()
	return in.policies.InvasionPolicy(ctx, gameID, modeID)
}

// listWorlds lista os mundos invadíveis DO MODO do invasor (spec 012) com
// timeout próprio — a política aplicada é a desse modo, então os mundos
// também precisam ser dele.
func (in *Invader) listWorlds(ctx context.Context, gameID, modeID string) ([]domain.InvadableWorld, error) {
	ctx, cancel := context.WithTimeout(ctx, invadeTimeout)
	defer cancel()
	return in.directory.ListInvadableWorlds(ctx, gameID, modeID)
}

// inject injeta o invasor num mundo com timeout próprio.
func (in *Invader) inject(ctx context.Context, sessionID, playerID string) (domain.Injected, error) {
	ctx, cancel := context.WithTimeout(ctx, invadeTimeout)
	defer cancel()
	return in.injector.Inject(ctx, sessionID, playerID, roleInvader, "")
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

package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/matchmaking-service/internal/domain"
)

var _ domain.Summoner = (*Summoner)(nil)

// summonTimeout limita cada chamada externa (policy/inject) na orquestração do
// co-op. roleCoop é o papel do aliado invocado (host_world_dropin, session/011).
const (
	summonTimeout = 5 * time.Second
	roleCoop      = "coop"
)

// Rótulos do contador summons_total{result} (spec 007, observabilidade).
const (
	resultSummoned  = "summoned"
	resultExpired   = "expired"
	resultCancelled = "cancelled"
)

// sign é o registro interno de um sinal no quadro: o dado puro (domain.Sign) +
// o canal de entrega ao aliado (buffer 1, como o ticket.ch do FindMatch) + o
// estado. claimed=true tira o sinal da List e o protege do sweeper/cancel: um
// Summon em andamento é o dono exclusivo dele.
type sign struct {
	domain.Sign
	ch      chan domain.Summoned
	claimed bool
}

// Summoner é o terceiro modo do matchmaking (spec 007): o quadro de sinais de
// co-op, em dois passos (anúncio → escolha do host → injeção). O store é estado
// interno em memória — a mesma natureza da fila do Matchmaker (D1). Orquestra a
// política (game-registry) e a injeção (session).
type Summoner struct {
	policies domain.CoopPolicyProvider
	injector domain.WorldInjector
	now      func() time.Time

	mu       sync.Mutex
	signs    map[string]*sign     // signID -> sinal ativo/claimed
	consumed map[string]time.Time // signID -> quando foi consumido (tombstone p/ idempotência)
	results  map[string]int64     // result -> contador (summoned/expired/cancelled)
}

// NewSummoner cria o serviço a partir da política de co-op e do injetor de mundo.
func NewSummoner(policies domain.CoopPolicyProvider, injector domain.WorldInjector) *Summoner {
	return &Summoner{
		policies: policies,
		injector: injector,
		now:      time.Now,
		signs:    make(map[string]*sign),
		consumed: make(map[string]time.Time),
		results:  make(map[string]int64),
	}
}

// PostSign publica um sinal do aliado no quadro e devolve o signID e o canal por
// onde o Summoned chega na invocação (o handler bloqueia nele). ErrCoopNotSupported
// se o jogo não aceita co-op.
func (s *Summoner) PostSign(ctx context.Context, gameID, modeID, playerID string, crit domain.CoopCriteria) (string, <-chan domain.Summoned, error) {
	policy, err := s.coopPolicy(ctx, gameID, modeID)
	if err != nil {
		return "", nil, err // ErrProfileUnavailable propaga
	}
	if !policy.SupportsCoop() {
		return "", nil, fmt.Errorf("summoner: %w", domain.ErrCoopNotSupported)
	}

	signID := uuid.NewString()
	sg := &sign{
		Sign: domain.Sign{
			SignID: signID, GameID: gameID, ModeID: modeID, PlayerID: playerID,
			SoulLevel: crit.SoulLevel, WeaponTier: crit.WeaponTier, PostedAt: s.now(),
			Password: crit.Password,
		},
		ch: make(chan domain.Summoned, 1), // buffer 1: a entrega nunca bloqueia o Summon
	}

	s.mu.Lock()
	s.signs[signID] = sg
	s.mu.Unlock()
	return signID, sg.ch, nil
}

// ListSigns devolve os sinais ativos do jogo compatíveis com o mundo do host
// (faixa de co-op). Lista vazia se nada casa; ErrCoopNotSupported se o jogo não
// aceita co-op.
func (s *Summoner) ListSigns(ctx context.Context, gameID, modeID string, host domain.CoopCriteria) ([]domain.Sign, error) {
	policy, err := s.coopPolicy(ctx, gameID, modeID)
	if err != nil {
		return nil, err
	}
	if !policy.SupportsCoop() {
		return nil, fmt.Errorf("summoner: %w", domain.ErrCoopNotSupported)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Sign
	for _, sg := range s.signs {
		// filtro por (jogo, modo): sinal de outro modo não aparece (ADR-0024).
		if sg.GameID != gameID || sg.ModeID != modeID || sg.claimed {
			continue
		}
		ally := domain.CoopCriteria{SoulLevel: sg.SoulLevel, WeaponTier: sg.WeaponTier}
		// Bypass por senha (follow-up #1, matchmaking/011): a mesma senha que o
		// host definiu no OpenWorld do seu mundo, comparada plaintext-contra-
		// plaintext com a do sinal (nunca atravessa o session-service).
		matchesPassword := policy.PasswordEnabled && host.Password != "" && sg.Password != "" && host.Password == sg.Password
		if matchesPassword || domain.CoopEligible(ally, host.SoulLevel, host.WeaponTier, policy) {
			out = append(out, sg.Sign)
		}
	}
	return out, nil
}

// Summon injeta o aliado do sinal no mundo como coop, consome o sinal (no máximo
// uma vez) e entrega o Summoned pelo canal. A transição active→claimed é atômica
// sob o lock (um sinal, um host); a injeção roda fora do lock (rede) e, se o mundo
// estiver cheio, o sinal volta a ficar disponível (não consome).
func (s *Summoner) Summon(ctx context.Context, sessionID, signID, summonerID string) (string, error) {
	s.mu.Lock()
	sg, ok := s.signs[signID]
	if !ok {
		// sinal fora do quadro: consumido antes (tombstone) → ErrSignConsumed;
		// senão nunca existiu/expirou → ErrSignNotFound.
		_, wasConsumed := s.consumed[signID]
		s.mu.Unlock()
		if wasConsumed {
			return "", fmt.Errorf("summoner: %w", domain.ErrSignConsumed)
		}
		return "", fmt.Errorf("summoner: %w", domain.ErrSignNotFound)
	}
	if sg.claimed {
		s.mu.Unlock()
		return "", fmt.Errorf("summoner: %w", domain.ErrSignConsumed) // invocação em andamento
	}
	sg.claimed = true // reserva exclusiva: só este host prossegue
	allyID := sg.PlayerID
	s.mu.Unlock()

	injected, err := s.inject(ctx, sessionID, allyID, summonerID)
	if err != nil {
		s.mu.Lock()
		sg.claimed = false // release: mundo cheio/fechado → sinal segue disponível
		s.mu.Unlock()
		switch {
		case errors.Is(err, domain.ErrRoleFull):
			return "", fmt.Errorf("summoner: %w", domain.ErrRoleFull)
		case errors.Is(err, domain.ErrWorldNotFound):
			return "", fmt.Errorf("summoner: %w", domain.ErrWorldNotFound)
		case errors.Is(err, domain.ErrNotWorldHost):
			return "", fmt.Errorf("summoner: %w", domain.ErrNotWorldHost)
		default:
			return "", fmt.Errorf("summoner: inject: %w", err)
		}
	}

	s.mu.Lock()
	delete(s.signs, signID)
	s.consumed[signID] = s.now() // tombstone: reinvocação vira ErrSignConsumed
	s.mu.Unlock()
	sg.ch <- domain.Summoned{SessionID: sessionID, ServerAddress: injected.ServerAddress, HostConnection: injected.HostConnection, ConnectToken: injected.ConnectToken}
	s.count(resultSummoned)
	return allyID, nil
}

// CancelSign remove o sinal do quadro (o aliado fechou o stream). Idempotente e
// no-op se o sinal já foi consumido ou está claimed (invocação em andamento
// tem prioridade). Fecha o canal — o handler que ainda escutasse veria EXPIRED,
// mas o cancelamento parte do próprio aliado, que já saiu.
func (s *Summoner) CancelSign(signID string) {
	s.mu.Lock()
	sg, ok := s.signs[signID]
	removed := ok && !sg.claimed
	if removed {
		delete(s.signs, signID)
		close(sg.ch)
	}
	s.mu.Unlock()
	if removed {
		s.count(resultCancelled)
	}
}

// SweepExpired fecha (EXPIRED) os sinais ativos publicados há mais que ttl e
// devolve quantos fechou. Sinais claimed (invocação em andamento) ficam de fora.
func (s *Summoner) SweepExpired(ttl time.Duration) int {
	cutoff := s.now().Add(-ttl)
	s.mu.Lock()
	n := 0
	for id, sg := range s.signs {
		if !sg.claimed && sg.PostedAt.Before(cutoff) {
			delete(s.signs, id)
			close(sg.ch) // handler distingue EXPIRED pelo canal fechado (ok=false)
			n++
		}
	}
	// tombstones de consumidos também vencem (o canal já foi entregue, não fecha).
	for id, at := range s.consumed {
		if at.Before(cutoff) {
			delete(s.consumed, id)
		}
	}
	s.mu.Unlock()
	for i := 0; i < n; i++ {
		s.count(resultExpired)
	}
	return n
}

// RunSweeper roda SweepExpired a cada interval até o ctx encerrar (goroutine no
// main — mesmo padrão do RunSweeper do session).
func (s *Summoner) RunSweeper(ctx context.Context, ttl, interval time.Duration, log *slog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n := s.SweepExpired(ttl); n > 0 {
				log.Info("summon sweeper: sinais expirados", "count", n, "ttl", ttl)
			}
		}
	}
}

// coopPolicy lê a política de co-op com timeout próprio.
func (s *Summoner) coopPolicy(ctx context.Context, gameID, modeID string) (domain.CoopPolicy, error) {
	ctx, cancel := context.WithTimeout(ctx, summonTimeout)
	defer cancel()
	return s.policies.CoopPolicy(ctx, gameID, modeID)
}

// inject injeta o aliado no mundo como coop, com timeout próprio. summonerID
// é quem invocou (o session exige que seja o host do mundo — session/030).
func (s *Summoner) inject(ctx context.Context, sessionID, allyID, summonerID string) (domain.Injected, error) {
	ctx, cancel := context.WithTimeout(ctx, summonTimeout)
	defer cancel()
	return s.injector.Inject(ctx, sessionID, allyID, roleCoop, summonerID)
}

// count incrementa o contador do resultado (métrica summons_total{result}).
func (s *Summoner) count(result string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.results[result]++
}

// SummonsTotal devolve o total de sinais com o desfecho (métrica summons_total
// {result} — CounterFunc no main).
func (s *Summoner) SummonsTotal(result string) float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return float64(s.results[result])
}

// SignsActive devolve quantos sinais estão no quadro (gauge summon_signs_active).
func (s *Summoner) SignsActive() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return float64(len(s.signs))
}

package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/achievements-service/internal/domain"
)

// Parâmetros do espelhamento na Steam (spec 019).
const (
	// lote × pior caso por item (identity 5 s + Steam 10 s) = 300 s < lease:
	// o lote termina antes de outra réplica poder pegar o mesmo item. Mesmo
	// assim as marcações conferem o lease (ErrSteamLeaseLost).
	steamBatch        = 20
	steamLease        = 6 * time.Minute // tempo que um item fica "alugado" por este worker
	steamMaxAttempts  = 10
	steamBaseBackoff  = time.Minute
	steamMaxBackoff   = time.Hour
	SteamSyncInterval = 15 * time.Second
)

// Resultados da métrica achievements_steam_sync_total{result}.
const (
	SteamResultSynced  = "synced"
	SteamResultSkipped = "skipped"
	SteamResultRetry   = "retry"
	SteamResultFailed  = "failed"
)

// SteamMirror espelha na Steam os desbloqueios das conquistas com
// steamApiName do jogo configurado (spec 019). Roda em segundo plano no
// consumer; o desbloqueio aqui nunca depende dele.
type SteamMirror struct {
	repo   domain.SteamSyncRepository
	ids    domain.SteamIDResolver
	steam  domain.SteamAchievementSetter
	gameID string
	now    func() time.Time
	log    *slog.Logger

	mu     sync.Mutex
	counts map[string]float64
}

// NewSteamMirror cria o espelhamento para o jogo gameID (STEAM_GAME_ID).
func NewSteamMirror(repo domain.SteamSyncRepository, ids domain.SteamIDResolver, steam domain.SteamAchievementSetter, gameID string, log *slog.Logger) *SteamMirror {
	if log == nil {
		log = slog.Default()
	}
	return &SteamMirror{repo: repo, ids: ids, steam: steam, gameID: gameID, now: time.Now, log: log, counts: map[string]float64{}}
}

// Run chama SyncOnce a cada interval até o ctx encerrar (goroutine do
// consumer, lifecycle por context + WaitGroup no main).
func (m *SteamMirror) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if _, err := m.SyncOnce(ctx); err != nil && ctx.Err() == nil {
			m.log.Warn("steam mirror: rodada falhou", slog.String("err", err.Error()))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// SyncOnce processa um lote de pendentes e devolve quantos tratou.
func (m *SteamMirror) SyncOnce(ctx context.Context) (int, error) {
	now := m.now()
	items, err := m.repo.ClaimSteamPending(ctx, m.gameID, steamBatch, now, now.Add(steamLease))
	if err != nil {
		return 0, fmt.Errorf("steam mirror: claim: %w", err)
	}
	for _, it := range items {
		if ctx.Err() != nil {
			break // o lease devolve o resto à fila
		}
		m.process(ctx, it)
	}
	return len(items), nil
}

func (m *SteamMirror) process(ctx context.Context, it domain.SteamSyncItem) {
	steamID, err := m.ids.SteamID(ctx, it.PlayerID)
	if errors.Is(err, domain.ErrNoSteamAccount) {
		m.done(ctx, it, domain.SteamSyncSkipped, "sem conta Steam vinculada", SteamResultSkipped)
		return
	}
	if err != nil {
		m.retryOrFail(ctx, it, fmt.Errorf("identity: %w", err))
		return
	}
	if err := m.steam.SetAchievement(ctx, steamID, it.SteamAPIName); err != nil {
		m.retryOrFail(ctx, it, err)
		return
	}
	m.done(ctx, it, domain.SteamSyncSynced, "", SteamResultSynced)
}

// retryOrFail reagenda com espera crescente; na última tentativa desiste.
func (m *SteamMirror) retryOrFail(ctx context.Context, it domain.SteamSyncItem, cause error) {
	attempt := it.Attempts + 1
	if attempt >= steamMaxAttempts {
		m.done(ctx, it, domain.SteamSyncFailed, cause.Error(), SteamResultFailed)
		return
	}
	if err := m.repo.MarkSteamRetry(ctx, it, m.now().Add(steamBackoff(attempt)), cause.Error()); err != nil {
		m.markFailed(it, err)
		return
	}
	m.count(SteamResultRetry)
}

func (m *SteamMirror) done(ctx context.Context, it domain.SteamSyncItem, state, reason, result string) {
	if err := m.repo.MarkSteamDone(ctx, it, state, reason, m.now()); err != nil {
		// o lease vence e o item volta: no pior caso a Steam recebe o mesmo
		// desbloqueio de novo, o que é idempotente do lado dela.
		m.markFailed(it, err)
		return
	}
	m.count(result)
}

// markFailed registra a falha ao gravar o estado. Lease perdido é esperado
// (outra réplica pegou o item depois do aluguel vencer) e não é erro.
func (m *SteamMirror) markFailed(it domain.SteamSyncItem, err error) {
	if errors.Is(err, domain.ErrSteamLeaseLost) {
		m.log.Debug("steam mirror: lease perdido, outra réplica cuida do item", slog.String("player_id", it.PlayerID))
		return
	}
	m.log.Warn("steam mirror: falha ao gravar estado", slog.String("player_id", it.PlayerID), slog.String("err", err.Error()))
}

func (m *SteamMirror) count(result string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counts[result]++
}

// SyncTotal alimenta a métrica achievements_steam_sync_total{result}.
func (m *SteamMirror) SyncTotal(result string) float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.counts[result]
}

// steamBackoff é 1 min × 2^(tentativa-1), com teto de 1 h.
func steamBackoff(attempt int) time.Duration {
	if attempt > 7 { // 2^6 min já passa do teto; evita estourar o shift
		return steamMaxBackoff
	}
	return min(steamBaseBackoff<<(attempt-1), steamMaxBackoff)
}

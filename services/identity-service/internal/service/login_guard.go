package service

import (
	"strings"
	"sync"
	"time"
)

// Limites do login por senha (spec 007): 5 falhas por handle em 15 min
// bloqueiam o handle até a janela vencer.
const (
	defaultLoginMaxFailures = 5
	defaultLoginWindow      = 15 * time.Minute
	// acima disto o mapa é varrido para descartar janelas vencidas
	loginGuardPruneAt = 10_000
)

// loginGuard conta falhas de login por handle, em memória (spec 007).
type loginGuard struct {
	max    int
	window time.Duration
	now    func() time.Time

	mu       sync.Mutex
	failures map[string]*loginFailures
}

type loginFailures struct {
	count int
	first time.Time // início da janela
}

func newLoginGuard(max int, window time.Duration, now func() time.Time) *loginGuard {
	return &loginGuard{max: max, window: window, now: now, failures: map[string]*loginFailures{}}
}

func guardKey(handle string) string { return strings.ToLower(strings.TrimSpace(handle)) }

// blocked diz se o handle esgotou as tentativas da janela atual.
func (g *loginGuard) blocked(handle string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	f, ok := g.failures[guardKey(handle)]
	if !ok {
		return false
	}
	if g.now().Sub(f.first) >= g.window {
		delete(g.failures, guardKey(handle))
		return false
	}
	return f.count >= g.max
}

// fail conta uma falha; a primeira falha abre a janela.
func (g *loginGuard) fail(handle string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	k := guardKey(handle)
	f, ok := g.failures[k]
	if !ok || now.Sub(f.first) >= g.window {
		if len(g.failures) >= loginGuardPruneAt {
			g.pruneLocked(now)
		}
		g.failures[k] = &loginFailures{count: 1, first: now}
		return
	}
	f.count++
}

// reset zera o handle depois de um login bem-sucedido.
func (g *loginGuard) reset(handle string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.failures, guardKey(handle))
}

func (g *loginGuard) pruneLocked(now time.Time) {
	for k, f := range g.failures {
		if now.Sub(f.first) >= g.window {
			delete(g.failures, k)
		}
	}
}

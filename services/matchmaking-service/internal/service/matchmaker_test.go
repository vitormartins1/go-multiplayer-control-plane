package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c130n/game-backend/services/matchmaking-service/internal/domain"
)

// fakeProvisioner permite trocar o comportamento por teste (sucesso/falha).
type fakeProvisioner struct {
	mu       sync.Mutex
	fn       func(matchID string, players []string) (domain.Provisioned, error)
	gotGames []string // game_ids recebidos (para o teste de isolamento)
}

func (f *fakeProvisioner) set(fn func(matchID string, players []string) (domain.Provisioned, error)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fn = fn
}

func (f *fakeProvisioner) Provision(_ context.Context, matchID string, players []string, gameID string) (domain.Provisioned, error) {
	f.mu.Lock()
	fn := f.fn
	f.gotGames = append(f.gotGames, gameID)
	f.mu.Unlock()
	return fn(matchID, players)
}

// successProvisioner devolve um token determinístico por jogador (tok-<player>).
func successProvisioner() *fakeProvisioner {
	f := &fakeProvisioner{}
	f.set(func(_ string, players []string) (domain.Provisioned, error) {
		tokens := make(map[string]string, len(players))
		for _, p := range players {
			tokens[p] = "tok-" + p
		}
		return domain.Provisioned{ServerAddress: "10.0.0.1:7777", Tokens: tokens}, nil
	})
	return f
}

func TestProvision_MatchSizeJogadores_TokenPorJogador(t *testing.T) {
	mm := New(2, time.Hour, successProvisioner())
	ctx := context.Background()

	ch1, _ := mm.Join(ctx, "game-1", "p1")
	ch2, _ := mm.Join(ctx, "game-1", "p2")
	mm.tryForm()

	m1 := <-ch1
	m2 := <-ch2
	assert.Equal(t, m1.MatchID, m2.MatchID, "mesmo matchId")
	assert.False(t, m1.Failed)
	assert.Equal(t, "10.0.0.1:7777", m1.ServerAddress)
	assert.Equal(t, "tok-p1", m1.ConnectToken, "p1 recebe o token do próprio jogador")
	assert.Equal(t, "tok-p2", m2.ConnectToken, "p2 recebe o token do próprio jogador")
	assert.ElementsMatch(t, []string{"p1", "p2"}, m1.Players)
	require.Eventually(t, func() bool { return mm.MatchesTotal() == 1 }, time.Second, 10*time.Millisecond)
}

func TestProvision_SemServer_EntregaFailed(t *testing.T) {
	prov := &fakeProvisioner{}
	prov.set(func(string, []string) (domain.Provisioned, error) { return domain.Provisioned{}, domain.ErrNoServer })
	mm := New(2, time.Hour, prov)

	ch1, _ := mm.Join(context.Background(), "game-1", "p1")
	ch2, _ := mm.Join(context.Background(), "game-1", "p2")
	mm.tryForm()

	m1 := <-ch1
	<-ch2
	assert.True(t, m1.Failed)
	assert.Equal(t, "no game server available", m1.Reason)
	assert.Empty(t, m1.ConnectToken)
	require.Eventually(t, func() bool { return mm.MatchesFailed() == 1 }, time.Second, 10*time.Millisecond)
}

func TestProvision_ErroGenerico_EntregaFailed(t *testing.T) {
	prov := &fakeProvisioner{}
	prov.set(func(string, []string) (domain.Provisioned, error) {
		return domain.Provisioned{}, errors.New("session indisponível")
	})
	mm := New(2, time.Hour, prov)

	ch1, _ := mm.Join(context.Background(), "game-1", "p1")
	ch2, _ := mm.Join(context.Background(), "game-1", "p2")
	mm.tryForm()

	m1 := <-ch1
	<-ch2
	assert.True(t, m1.Failed)
	assert.Equal(t, "session provisioning failed", m1.Reason)
}

func TestProvision_ReJoinAposFalha_ConsegueMatch(t *testing.T) {
	prov := &fakeProvisioner{}
	prov.set(func(string, []string) (domain.Provisioned, error) { return domain.Provisioned{}, domain.ErrNoServer })
	mm := New(2, time.Hour, prov)

	ch1, _ := mm.Join(context.Background(), "game-1", "p1")
	ch2, _ := mm.Join(context.Background(), "game-1", "p2")
	mm.tryForm()
	<-ch1
	<-ch2 // ambos falharam, saíram da fila

	// session volta: jogadores re-ingressam e agora são matched.
	prov.set(func(_ string, players []string) (domain.Provisioned, error) {
		tokens := map[string]string{}
		for _, p := range players {
			tokens[p] = "tok-" + p
		}
		return domain.Provisioned{ServerAddress: "10.0.0.2:7777", Tokens: tokens}, nil
	})
	ch1b, _ := mm.Join(context.Background(), "game-1", "p1")
	ch2b, _ := mm.Join(context.Background(), "game-1", "p2")
	mm.tryForm()

	m := <-ch1b
	<-ch2b
	assert.False(t, m.Failed)
	assert.Equal(t, "10.0.0.2:7777", m.ServerAddress)
}

func TestTryForm_MenosQueMatchSize_NaoForma(t *testing.T) {
	mm := New(2, time.Hour, successProvisioner())
	ch1, _ := mm.Join(context.Background(), "game-1", "p1")
	mm.tryForm()

	select {
	case <-ch1:
		t.Fatal("não deveria formar match com 1 jogador")
	default:
	}
	assert.Equal(t, 1, mm.QueueSize())
}

func TestJoin_Cancelado_SaiDaFila(t *testing.T) {
	mm := New(2, time.Hour, successProvisioner())
	ctx1, cancel1 := context.WithCancel(context.Background())
	mm.Join(ctx1, "game-1", "p1")
	ch2, _ := mm.Join(context.Background(), "game-1", "p2")

	cancel1() // p1 sai
	require.Eventually(t, func() bool { return mm.QueueSize() == 1 }, time.Second, 10*time.Millisecond)

	mm.tryForm() // só p2 na fila -> não forma
	select {
	case <-ch2:
		t.Fatal("p2 não deveria ser matched (p1 cancelou)")
	default:
	}
}

func TestJoin_MesmoJogador_UmTicket(t *testing.T) {
	mm := New(2, time.Hour, successProvisioner())
	ctx := context.Background()
	mm.Join(ctx, "game-1", "p1")
	mm.Join(ctx, "game-1", "p1") // segundo Join do mesmo jogador

	assert.Equal(t, 1, mm.QueueSize(), "um ticket por jogador")
}

func TestRun_FormaPeloTicker(t *testing.T) {
	mm := New(2, 10*time.Millisecond, successProvisioner())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mm.Run(ctx)

	ch1, _ := mm.Join(ctx, "game-1", "p1")
	ch2, _ := mm.Join(ctx, "game-1", "p2")

	select {
	case m := <-ch1:
		assert.Len(t, m.Players, 2)
		assert.Equal(t, "tok-p1", m.ConnectToken)
	case <-time.After(time.Second):
		t.Fatal("timeout esperando o match pelo ticker")
	}
	<-ch2
}

func TestTryForm_JogosDiferentes_NaoCasam(t *testing.T) {
	prov := successProvisioner()
	mm := New(2, time.Hour, prov)
	ctx := context.Background()

	chA, _ := mm.Join(ctx, "game-a", "p1")
	chB, _ := mm.Join(ctx, "game-b", "p2") // jogo diferente
	mm.tryForm()

	// nenhum forma: só 1 jogador por jogo.
	select {
	case <-chA:
		t.Fatal("game-a não deveria formar com 1 jogador")
	case <-chB:
		t.Fatal("p1(game-a) e p2(game-b) não podem casar")
	default:
	}
	assert.Equal(t, 2, mm.QueueSize())

	// chega outro jogador no game-a -> forma só no game-a.
	chA2, _ := mm.Join(ctx, "game-a", "p3")
	mm.tryForm()
	m := <-chA
	<-chA2
	assert.False(t, m.Failed)
	assert.ElementsMatch(t, []string{"p1", "p3"}, m.Players)

	// o provisioner recebeu o game-a (e nunca misturou).
	prov.mu.Lock()
	defer prov.mu.Unlock()
	assert.Contains(t, prov.gotGames, "game-a")
	assert.NotContains(t, prov.gotGames, "game-b")
}

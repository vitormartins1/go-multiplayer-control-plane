package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTryForm_MatchSizeJogadores_MesmoMatch(t *testing.T) {
	mm := New(2, time.Hour) // intervalo alto: formamos manualmente via tryForm
	ctx := context.Background()

	ch1, _ := mm.Join(ctx, "p1")
	ch2, _ := mm.Join(ctx, "p2")
	mm.tryForm()

	m1 := <-ch1
	m2 := <-ch2
	assert.Equal(t, m1.ID, m2.ID, "mesmo matchId")
	assert.Len(t, m1.Players, 2)
	assert.ElementsMatch(t, []string{"p1", "p2"}, m1.Players)
	assert.NotEmpty(t, m1.ConnectInfo)
	assert.Equal(t, float64(1), mm.MatchesTotal())
}

func TestTryForm_MenosQueMatchSize_NaoForma(t *testing.T) {
	mm := New(2, time.Hour)
	ch1, _ := mm.Join(context.Background(), "p1")
	mm.tryForm()

	select {
	case <-ch1:
		t.Fatal("não deveria formar match com 1 jogador")
	default:
	}
	assert.Equal(t, 1, mm.QueueSize())
}

func TestJoin_Cancelado_SaiDaFila(t *testing.T) {
	mm := New(2, time.Hour)
	ctx1, cancel1 := context.WithCancel(context.Background())
	mm.Join(ctx1, "p1")
	ch2, _ := mm.Join(context.Background(), "p2")

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
	mm := New(2, time.Hour)
	ctx := context.Background()
	mm.Join(ctx, "p1")
	mm.Join(ctx, "p1") // segundo Join do mesmo jogador

	assert.Equal(t, 1, mm.QueueSize(), "um ticket por jogador")
}

func TestRun_FormaPeloTicker(t *testing.T) {
	mm := New(2, 10*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mm.Run(ctx)

	ch1, _ := mm.Join(ctx, "p1")
	ch2, _ := mm.Join(ctx, "p2")

	select {
	case m := <-ch1:
		assert.Len(t, m.Players, 2)
	case <-time.After(time.Second):
		t.Fatal("timeout esperando o match pelo ticker")
	}
	<-ch2
}

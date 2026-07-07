package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/matchmaking-service/internal/domain"
)

// Testes da fila por (jogo, modo) — ADR-0024, spec 010. Fakes em matchmaker_test.go.

// Modos diferentes do MESMO jogo nunca se misturam: 1 ticket em cada modo não
// forma; o segundo ticket do mesmo modo forma o par daquele modo.
func TestTryForm_ModosDiferentes_NaoMisturam(t *testing.T) {
	prov := successProvisioner()
	mm := New(time.Hour, newFakeProvider(2), prov)
	ctx := context.Background()

	chTDM1, _ := mm.Join(ctx, "game-1", "tdm", "p1")
	chRanked, _ := mm.Join(ctx, "game-1", "ranked", "p2")
	mm.tryForm()
	select {
	case <-chTDM1:
		t.Fatal("não deveria formar: modos diferentes do mesmo jogo")
	case <-chRanked:
		t.Fatal("não deveria formar: modos diferentes do mesmo jogo")
	default: // ninguém formou — correto
	}

	chTDM2, _ := mm.Join(ctx, "game-1", "tdm", "p3")
	mm.tryForm()
	r1 := <-chTDM1
	r3 := <-chTDM2
	assert.ElementsMatch(t, []string{"p1", "p3"}, r1.Players, "só o modo tdm formou")
	assert.Equal(t, r1.MatchID, r3.MatchID)
	select {
	case <-chRanked:
		t.Fatal("ranked segue aguardando o segundo ticket")
	default:
	}
}

// mode_id normalizado na borda: quem entra como "default" agrupa com quem
// entrou explícito (a normalização "" -> default é do transport; aqui os dois
// tickets já chegam como "default").
func TestTryForm_MesmoModo_FormaEPropagaModeIDAoProvision(t *testing.T) {
	prov := successProvisioner()
	mm := New(time.Hour, newFakeProvider(2), prov)
	ctx := context.Background()

	ch1, _ := mm.Join(ctx, "game-1", "warzone", "p1")
	ch2, _ := mm.Join(ctx, "game-1", "warzone", "p2")
	mm.tryForm()
	<-ch1
	<-ch2

	prov.mu.Lock()
	defer prov.mu.Unlock()
	require.Len(t, prov.gotModes, 1)
	assert.Equal(t, "warzone", prov.gotModes[0], "modeID segue para o session (snapshot)")
}

// Perfil indisponível é por (jogo, modo): o modo quebrado rejeita o ticket sem
// afetar o modo saudável do mesmo jogo.
func TestJoin_ModoSemPerfil_ErrProfileUnavailable(t *testing.T) {
	provider := newFakeProvider(2)
	provider.err = domain.ErrProfileUnavailable
	mm := New(time.Hour, provider, successProvisioner())

	_, err := mm.Join(context.Background(), "game-1", "quebrado", "p1")
	assert.ErrorIs(t, err, domain.ErrProfileUnavailable)
}

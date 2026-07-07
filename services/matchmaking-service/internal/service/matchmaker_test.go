package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/matchmaking-service/internal/domain"
)

// fakeProvider simula o domain.ProfileProvider: devolve uma formação por gameID
// (ou o default FFA), ou um erro. Conta as chamadas.
type fakeProvider struct {
	mu      sync.Mutex
	def     domain.MatchFormat
	formats map[string]domain.MatchFormat
	err     error
	calls   int
}

func newFakeProvider(defSize int) *fakeProvider {
	return &fakeProvider{
		def:     domain.MatchFormat{Size: defSize, Structure: domain.StructureFFA},
		formats: map[string]domain.MatchFormat{},
	}
}

func (f *fakeProvider) Format(_ context.Context, gameID, _ string) (domain.MatchFormat, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return domain.MatchFormat{}, f.err
	}
	if fm, ok := f.formats[gameID]; ok {
		return fm, nil
	}
	return f.def, nil
}

// fakeProvisioner permite trocar o comportamento por teste (sucesso/falha).
type fakeProvisioner struct {
	mu       sync.Mutex
	fn       func(matchID string, teams []domain.TeamAssignment) (domain.Provisioned, error)
	gotGames []string                  // game_ids recebidos (para o teste de isolamento)
	gotModes []string                  // mode_ids recebidos (ADR-0024)
	gotTeams [][]domain.TeamAssignment // composições recebidas (para o teste de times)
}

func (f *fakeProvisioner) set(fn func(matchID string, teams []domain.TeamAssignment) (domain.Provisioned, error)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fn = fn
}

func (f *fakeProvisioner) Provision(_ context.Context, matchID string, teams []domain.TeamAssignment, gameID, modeID string) (domain.Provisioned, error) {
	f.mu.Lock()
	fn := f.fn
	f.gotGames = append(f.gotGames, gameID)
	f.gotModes = append(f.gotModes, modeID)
	f.gotTeams = append(f.gotTeams, teams)
	f.mu.Unlock()
	return fn(matchID, teams)
}

// tokensFor monta um token determinístico por jogador de todos os times.
func tokensFor(teams []domain.TeamAssignment) map[string]string {
	tokens := map[string]string{}
	for _, tm := range teams {
		for _, p := range tm.Players {
			tokens[p] = "tok-" + p
		}
	}
	return tokens
}

// successProvisioner devolve um token determinístico por jogador (tok-<player>).
func successProvisioner() *fakeProvisioner {
	f := &fakeProvisioner{}
	f.set(func(_ string, teams []domain.TeamAssignment) (domain.Provisioned, error) {
		return domain.Provisioned{ServerAddress: "10.0.0.1:7777", Tokens: tokensFor(teams)}, nil
	})
	return f
}

func TestProvision_MatchSizeJogadores_TokenPorJogador(t *testing.T) {
	mm := New(time.Hour, newFakeProvider(2), successProvisioner())
	ctx := context.Background()

	ch1, _ := mm.Join(ctx, "game-1", "default", "p1")
	ch2, _ := mm.Join(ctx, "game-1", "default", "p2")
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
	prov.set(func(string, []domain.TeamAssignment) (domain.Provisioned, error) {
		return domain.Provisioned{}, domain.ErrNoServer
	})
	mm := New(time.Hour, newFakeProvider(2), prov)

	ch1, _ := mm.Join(context.Background(), "game-1", "default", "p1")
	ch2, _ := mm.Join(context.Background(), "game-1", "default", "p2")
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
	prov.set(func(string, []domain.TeamAssignment) (domain.Provisioned, error) {
		return domain.Provisioned{}, errors.New("session indisponível")
	})
	mm := New(time.Hour, newFakeProvider(2), prov)

	ch1, _ := mm.Join(context.Background(), "game-1", "default", "p1")
	ch2, _ := mm.Join(context.Background(), "game-1", "default", "p2")
	mm.tryForm()

	m1 := <-ch1
	<-ch2
	assert.True(t, m1.Failed)
	assert.Equal(t, "session provisioning failed", m1.Reason)
}

func TestProvision_ReJoinAposFalha_ConsegueMatch(t *testing.T) {
	prov := &fakeProvisioner{}
	prov.set(func(string, []domain.TeamAssignment) (domain.Provisioned, error) {
		return domain.Provisioned{}, domain.ErrNoServer
	})
	mm := New(time.Hour, newFakeProvider(2), prov)

	ch1, _ := mm.Join(context.Background(), "game-1", "default", "p1")
	ch2, _ := mm.Join(context.Background(), "game-1", "default", "p2")
	mm.tryForm()
	<-ch1
	<-ch2 // ambos falharam, saíram da fila

	// session volta: jogadores re-ingressam e agora são matched.
	prov.set(func(_ string, teams []domain.TeamAssignment) (domain.Provisioned, error) {
		return domain.Provisioned{ServerAddress: "10.0.0.2:7777", Tokens: tokensFor(teams)}, nil
	})
	ch1b, _ := mm.Join(context.Background(), "game-1", "default", "p1")
	ch2b, _ := mm.Join(context.Background(), "game-1", "default", "p2")
	mm.tryForm()

	m := <-ch1b
	<-ch2b
	assert.False(t, m.Failed)
	assert.Equal(t, "10.0.0.2:7777", m.ServerAddress)
}

func TestTryForm_MenosQueMatchSize_NaoForma(t *testing.T) {
	mm := New(time.Hour, newFakeProvider(2), successProvisioner())
	ch1, _ := mm.Join(context.Background(), "game-1", "default", "p1")
	mm.tryForm()

	select {
	case <-ch1:
		t.Fatal("não deveria formar match com 1 jogador")
	default:
	}
	assert.Equal(t, 1, mm.QueueSize())
}

func TestJoin_Cancelado_SaiDaFila(t *testing.T) {
	mm := New(time.Hour, newFakeProvider(2), successProvisioner())
	ctx1, cancel1 := context.WithCancel(context.Background())
	mm.Join(ctx1, "game-1", "default", "p1")
	ch2, _ := mm.Join(context.Background(), "game-1", "default", "p2")

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
	mm := New(time.Hour, newFakeProvider(2), successProvisioner())
	ctx := context.Background()
	mm.Join(ctx, "game-1", "default", "p1")
	mm.Join(ctx, "game-1", "default", "p1") // segundo Join do mesmo jogador

	assert.Equal(t, 1, mm.QueueSize(), "um ticket por jogador")
}

func TestRun_FormaPeloTicker(t *testing.T) {
	mm := New(10*time.Millisecond, newFakeProvider(2), successProvisioner())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mm.Run(ctx)

	ch1, _ := mm.Join(ctx, "game-1", "default", "p1")
	ch2, _ := mm.Join(ctx, "game-1", "default", "p2")

	select {
	case m := <-ch1:
		assert.Len(t, m.Players, 2)
		assert.Equal(t, "tok-p1", m.ConnectToken)
	case <-time.After(time.Second):
		t.Fatal("timeout esperando o match pelo ticker")
	}
	<-ch2
}

func TestJoin_TamanhoDoPerfil_FormaComQuatro(t *testing.T) {
	mm := New(time.Hour, newFakeProvider(4), successProvisioner())
	ctx := context.Background()

	var chs []<-chan domain.MatchResult
	for _, p := range []string{"p1", "p2", "p3", "p4"} {
		ch, err := mm.Join(ctx, "game-1", "default", p)
		require.NoError(t, err)
		chs = append(chs, ch)
	}
	mm.tryForm()

	m := <-chs[0]
	assert.False(t, m.Failed)
	assert.Len(t, m.Players, 4, "match forma no tamanho do perfil (4)")
	for _, ch := range chs[1:] {
		<-ch
	}
}

func TestJoin_JogosComTamanhosDiferentes_FormamIndependente(t *testing.T) {
	prov := newFakeProvider(0)
	prov.formats = map[string]domain.MatchFormat{
		"game-a": {Size: 2, Structure: domain.StructureFFA},
		"game-b": {Size: 3, Structure: domain.StructureFFA},
	}
	mm := New(time.Hour, prov, successProvisioner())
	ctx := context.Background()

	// game-a: 2 jogadores (forma); game-b: 3 jogadores (forma).
	a1, _ := mm.Join(ctx, "game-a", "default", "a1")
	a2, _ := mm.Join(ctx, "game-a", "default", "a2")
	b1, _ := mm.Join(ctx, "game-b", "default", "b1")
	b2, _ := mm.Join(ctx, "game-b", "default", "b2")
	b3, _ := mm.Join(ctx, "game-b", "default", "b3")
	mm.tryForm()

	ma := <-a1
	<-a2
	mb := <-b1
	<-b2
	<-b3
	assert.Len(t, ma.Players, 2, "game-a forma com 2")
	assert.Len(t, mb.Players, 3, "game-b forma com 3")
	assert.Equal(t, 0, mm.QueueSize(), "fila esvazia")
}

func TestJoin_PerfilIndisponivel_NaoEntraNaFila(t *testing.T) {
	prov := newFakeProvider(2)
	prov.err = domain.ErrProfileUnavailable
	mm := New(time.Hour, prov, successProvisioner())

	ch, err := mm.Join(context.Background(), "sem-perfil", "default", "p1")
	assert.ErrorIs(t, err, domain.ErrProfileUnavailable)
	assert.Nil(t, ch)
	assert.Equal(t, 0, mm.QueueSize(), "ticket não entra na fila sem perfil")
}

func TestTryForm_JogosDiferentes_NaoCasam(t *testing.T) {
	prov := successProvisioner()
	mm := New(time.Hour, newFakeProvider(2), prov)
	ctx := context.Background()

	chA, _ := mm.Join(ctx, "game-a", "default", "p1")
	chB, _ := mm.Join(ctx, "game-b", "default", "p2") // jogo diferente
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
	chA2, _ := mm.Join(ctx, "game-a", "default", "p3")
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

// --- composição de times (ADR-0023, spec 005) ---

// teams: 4 jogadores viram 2 times de 2, por blocos contíguos na ordem FIFO,
// rótulos team-1/team-2. O provisioner recebe a composição.
func TestTryForm_Teams_ParticionaEmLados(t *testing.T) {
	prov := successProvisioner()
	provider := newFakeProvider(0)
	provider.formats["game-1"] = domain.MatchFormat{Size: 4, Structure: domain.StructureTeams, TeamCount: 2, TeamSize: 2}
	mm := New(time.Hour, provider, prov)
	ctx := context.Background()

	var chs []<-chan domain.MatchResult
	for _, p := range []string{"p1", "p2", "p3", "p4"} {
		ch, _ := mm.Join(ctx, "game-1", "default", p)
		chs = append(chs, ch)
	}
	mm.tryForm()
	for _, ch := range chs {
		<-ch
	}

	prov.mu.Lock()
	defer prov.mu.Unlock()
	require.Len(t, prov.gotTeams, 1)
	teams := prov.gotTeams[0]
	require.Len(t, teams, 2)
	assert.Equal(t, "team-1", teams[0].TeamID)
	assert.Equal(t, []string{"p1", "p2"}, teams[0].Players)
	assert.Equal(t, "team-2", teams[1].TeamID)
	assert.Equal(t, []string{"p3", "p4"}, teams[1].Players)
}

// teams 3x3: valida um tamanho de time diferente.
func TestTryForm_Teams3x3_SeisJogadoresDoisTimesDeTres(t *testing.T) {
	prov := successProvisioner()
	provider := newFakeProvider(0)
	provider.formats["g"] = domain.MatchFormat{Size: 6, Structure: domain.StructureTeams, TeamCount: 2, TeamSize: 3}
	mm := New(time.Hour, provider, prov)
	ctx := context.Background()

	var chs []<-chan domain.MatchResult
	for _, p := range []string{"a", "b", "c", "d", "e", "f"} {
		ch, _ := mm.Join(ctx, "g", "default", p)
		chs = append(chs, ch)
	}
	mm.tryForm()
	for _, ch := range chs {
		<-ch
	}

	prov.mu.Lock()
	defer prov.mu.Unlock()
	teams := prov.gotTeams[0]
	require.Len(t, teams, 2)
	assert.Equal(t, []string{"a", "b", "c"}, teams[0].Players)
	assert.Equal(t, []string{"d", "e", "f"}, teams[1].Players)
}

// coop_mission: um único time (TeamCount=1) de teamSize contra o ambiente. O
// provisioner recebe 1 assignment "team-1" com todos os jogadores (C1).
func TestTryForm_CoopMission_TimeUnico(t *testing.T) {
	prov := successProvisioner()
	provider := newFakeProvider(0)
	provider.formats["coop"] = domain.MatchFormat{Size: 4, Structure: domain.StructureTeams, TeamCount: 1, TeamSize: 4}
	mm := New(time.Hour, provider, prov)
	ctx := context.Background()

	var chs []<-chan domain.MatchResult
	for _, p := range []string{"p1", "p2", "p3", "p4"} {
		ch, _ := mm.Join(ctx, "coop", "default", p)
		chs = append(chs, ch)
	}
	mm.tryForm()
	for _, ch := range chs {
		<-ch
	}

	prov.mu.Lock()
	defer prov.mu.Unlock()
	require.Len(t, prov.gotTeams, 1)
	teams := prov.gotTeams[0]
	require.Len(t, teams, 1, "co-op = um único time")
	assert.Equal(t, "team-1", teams[0].TeamID)
	assert.Equal(t, []string{"p1", "p2", "p3", "p4"}, teams[0].Players)
}

// squads (battle_royale): particiona o lobby em blocos de teamSize, rótulos
// "squad-K". lobbySize múltiplo de teamSize → todos os squads cheios.
func TestTryForm_Squads_ParticionaEmSquadsCheios(t *testing.T) {
	prov := successProvisioner()
	provider := newFakeProvider(0)
	provider.formats["br"] = domain.MatchFormat{Size: 8, Structure: domain.StructureSquads, TeamSize: 4}
	mm := New(time.Hour, provider, prov)
	ctx := context.Background()

	var chs []<-chan domain.MatchResult
	for _, p := range []string{"p1", "p2", "p3", "p4", "p5", "p6", "p7", "p8"} {
		ch, _ := mm.Join(ctx, "br", "default", p)
		chs = append(chs, ch)
	}
	mm.tryForm()
	for _, ch := range chs {
		<-ch
	}

	prov.mu.Lock()
	defer prov.mu.Unlock()
	require.Len(t, prov.gotTeams, 1)
	teams := prov.gotTeams[0]
	require.Len(t, teams, 2)
	assert.Equal(t, "squad-1", teams[0].TeamID)
	assert.Equal(t, []string{"p1", "p2", "p3", "p4"}, teams[0].Players)
	assert.Equal(t, "squad-2", teams[1].TeamID)
	assert.Equal(t, []string{"p5", "p6", "p7", "p8"}, teams[1].Players)
}

// squads ragged: lobbySize NÃO múltiplo de teamSize → o último squad é menor.
// Nenhum jogador fica de fora (BR0 permite lobby não-múltiplo).
func TestTryForm_Squads_UltimoSquadRagged(t *testing.T) {
	prov := successProvisioner()
	provider := newFakeProvider(0)
	provider.formats["br"] = domain.MatchFormat{Size: 10, Structure: domain.StructureSquads, TeamSize: 4}
	mm := New(time.Hour, provider, prov)
	ctx := context.Background()

	var chs []<-chan domain.MatchResult
	players := []string{"p1", "p2", "p3", "p4", "p5", "p6", "p7", "p8", "p9", "p10"}
	for _, p := range players {
		ch, _ := mm.Join(ctx, "br", "default", p)
		chs = append(chs, ch)
	}
	mm.tryForm()
	for _, ch := range chs {
		<-ch
	}

	prov.mu.Lock()
	defer prov.mu.Unlock()
	teams := prov.gotTeams[0]
	require.Len(t, teams, 3, "⌈10/4⌉ = 3 squads")
	assert.Equal(t, []string{"p1", "p2", "p3", "p4"}, teams[0].Players)
	assert.Equal(t, []string{"p5", "p6", "p7", "p8"}, teams[1].Players)
	assert.Equal(t, "squad-3", teams[2].TeamID)
	assert.Equal(t, []string{"p9", "p10"}, teams[2].Players, "último squad ragged (2 jogadores)")

	var total int
	for _, tm := range teams {
		total += len(tm.Players)
	}
	assert.Equal(t, len(players), total, "nenhum jogador perdido")
}

// battle_royale solo: teamSize 1 vira ffa no Format → um assignment sem TeamID.
func TestTryForm_BattleRoyaleSolo_FFA(t *testing.T) {
	prov := successProvisioner()
	provider := newFakeProvider(0)
	provider.formats["br"] = domain.MatchFormat{Size: 3, Structure: domain.StructureFFA}
	mm := New(time.Hour, provider, prov)
	ctx := context.Background()

	var chs []<-chan domain.MatchResult
	for _, p := range []string{"p1", "p2", "p3"} {
		ch, _ := mm.Join(ctx, "br", "default", p)
		chs = append(chs, ch)
	}
	mm.tryForm()
	for _, ch := range chs {
		<-ch
	}

	prov.mu.Lock()
	defer prov.mu.Unlock()
	teams := prov.gotTeams[0]
	require.Len(t, teams, 1)
	assert.Empty(t, teams[0].TeamID, "solo BR = FFA, sem team_id")
	assert.Equal(t, []string{"p1", "p2", "p3"}, teams[0].Players)
}

// ffa: o provisioner recebe um único assignment sem TeamID (sem times).
func TestTryForm_FFA_UmAssignmentSemTime(t *testing.T) {
	prov := successProvisioner()
	mm := New(time.Hour, newFakeProvider(2), prov)
	ctx := context.Background()

	ch1, _ := mm.Join(ctx, "game-1", "default", "p1")
	ch2, _ := mm.Join(ctx, "game-1", "default", "p2")
	mm.tryForm()
	<-ch1
	<-ch2

	prov.mu.Lock()
	defer prov.mu.Unlock()
	require.Len(t, prov.gotTeams, 1)
	teams := prov.gotTeams[0]
	require.Len(t, teams, 1)
	assert.Empty(t, teams[0].TeamID, "FFA: assignment sem TeamID")
	assert.ElementsMatch(t, []string{"p1", "p2"}, teams[0].Players)
}

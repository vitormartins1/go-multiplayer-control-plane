package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/matchmaking-service/internal/domain"
)

// Fakes das três Ports de saída da invasão (mesmo estilo dos fakes do matchmaker).

type fakePolicies struct {
	policy domain.InvasionPolicy
	err    error
}

func (f *fakePolicies) InvasionPolicy(context.Context, string, string) (domain.InvasionPolicy, error) {
	return f.policy, f.err
}

// fakeDirectory devolve worlds em toda chamada (compat com os testes de
// mundo único); quando responses está setado, devolve uma lista diferente
// por chamada (índice além do fim repete a última) — usado pra provar o
// refetch de Invade (achado no /code-review — "JoinWorld race").
type fakeDirectory struct {
	worlds    []domain.InvadableWorld
	responses [][]domain.InvadableWorld
	err       error
	calls     int
	gotMode   string
}

func (f *fakeDirectory) ListInvadableWorlds(_ context.Context, _, modeID string) ([]domain.InvadableWorld, error) {
	f.calls++
	f.gotMode = modeID
	if f.err != nil {
		return nil, f.err
	}
	if f.responses == nil {
		return f.worlds, nil
	}
	idx := f.calls - 1
	if idx >= len(f.responses) {
		idx = len(f.responses) - 1
	}
	return f.responses[idx], nil
}

// fakeInjector devolve, por sessionID, o erro configurado (nil = sucesso) e
// registra a ordem das chamadas — para provar o retry no próximo mundo.
type fakeInjector struct {
	byWorld map[string]error
	calls   []string
}

func (f *fakeInjector) Inject(_ context.Context, sessionID, playerID, role, _ string) (domain.Injected, error) {
	f.calls = append(f.calls, sessionID)
	if err := f.byWorld[sessionID]; err != nil {
		return domain.Injected{}, err
	}
	return domain.Injected{ServerAddress: "addr-" + sessionID, ConnectToken: "tok-" + playerID}, nil
}

// invasionOn é uma política host_world_dropin com invasão ligada e sem filtro de
// faixa (todo mundo aberto é elegível) — simplifica os ramos de orquestração.
func invasionOn() domain.InvasionPolicy {
	return domain.InvasionPolicy{Archetype: "host_world_dropin", InvasionEnabled: true, LevelRangeEnabled: false}
}

func TestInvade_MundoElegivel_InjetaEDevolveServerEToken(t *testing.T) {
	pol := &fakePolicies{policy: invasionOn()}
	dir := &fakeDirectory{worlds: []domain.InvadableWorld{{SessionID: "w1", FreeInvaderSlots: 1}}}
	inj := &fakeInjector{byWorld: map[string]error{}}
	in := NewInvader(pol, dir, inj)

	got, err := in.Invade(context.Background(), "game-1", "default", "invader-1", domain.InvaderCriteria{SoulLevel: 100})
	require.NoError(t, err)
	assert.Equal(t, "w1", got.SessionID)
	assert.Equal(t, "addr-w1", got.ServerAddress)
	assert.Equal(t, "tok-invader-1", got.ConnectToken)
	assert.Equal(t, float64(1), in.InvasionsTotal(resultInjected))
}

func TestInvade_ArchetypeErrado_ErrInvasionNotSupported(t *testing.T) {
	pol := &fakePolicies{policy: domain.InvasionPolicy{Archetype: "queue_arena", InvasionEnabled: true}}
	in := NewInvader(pol, &fakeDirectory{}, &fakeInjector{})

	_, err := in.Invade(context.Background(), "game-1", "default", "invader-1", domain.InvaderCriteria{})
	assert.ErrorIs(t, err, domain.ErrInvasionNotSupported)
	assert.Equal(t, float64(1), in.InvasionsTotal(resultNotSupported))
}

func TestInvade_InvasaoDesligada_ErrInvasionNotSupported(t *testing.T) {
	pol := &fakePolicies{policy: domain.InvasionPolicy{Archetype: "host_world_dropin", InvasionEnabled: false}}
	in := NewInvader(pol, &fakeDirectory{}, &fakeInjector{})

	_, err := in.Invade(context.Background(), "game-1", "default", "invader-1", domain.InvaderCriteria{})
	assert.ErrorIs(t, err, domain.ErrInvasionNotSupported)
}

func TestInvade_ListaVazia_ErrNoEligibleWorld(t *testing.T) {
	dir := &fakeDirectory{worlds: nil}
	in := NewInvader(&fakePolicies{policy: invasionOn()}, dir, &fakeInjector{})

	_, err := in.Invade(context.Background(), "game-1", "default", "invader-1", domain.InvaderCriteria{SoulLevel: 100})
	assert.ErrorIs(t, err, domain.ErrNoEligibleWorld)
	assert.Equal(t, float64(1), in.InvasionsTotal(resultNoWorld))
	assert.Equal(t, 1, dir.calls, "lista vazia não é corrida: sem refetch, evita dobrar a latência do caso comum")
}

func TestInvade_NenhumNaFaixa_ErrNoEligibleWorld(t *testing.T) {
	// faixa ligada, invasor L=100 (banda [90,130]); host 500 fora → nada elegível.
	pol := &fakePolicies{policy: domain.InvasionPolicy{Archetype: "host_world_dropin", InvasionEnabled: true, LevelRangeEnabled: true}}
	dir := &fakeDirectory{worlds: []domain.InvadableWorld{{SessionID: "w1", HostSoulLevel: 500, FreeInvaderSlots: 1}}}
	inj := &fakeInjector{byWorld: map[string]error{}}
	in := NewInvader(pol, dir, inj)

	_, err := in.Invade(context.Background(), "game-1", "default", "invader-1", domain.InvaderCriteria{SoulLevel: 100})
	assert.ErrorIs(t, err, domain.ErrNoEligibleWorld)
	assert.Empty(t, inj.calls, "mundo fora da faixa nem é tentado")
}

func TestInvade_SenhaBateForaDaFaixa_InjetaMesmoAssim(t *testing.T) {
	// faixa ligada, invasor L=100 fora da banda do host L=500 — só a senha salva.
	pol := &fakePolicies{policy: domain.InvasionPolicy{Archetype: "host_world_dropin", InvasionEnabled: true, LevelRangeEnabled: true, PasswordEnabled: true}}
	dir := &fakeDirectory{worlds: []domain.InvadableWorld{
		{SessionID: "w1", HostSoulLevel: 500, FreeInvaderSlots: 1, PasswordHash: domain.HashWorldPassword("abrepalavra")},
	}}
	inj := &fakeInjector{byWorld: map[string]error{}}
	in := NewInvader(pol, dir, inj)

	got, err := in.Invade(context.Background(), "game-1", "default", "invader-1",
		domain.InvaderCriteria{SoulLevel: 100, Password: "abrepalavra"})
	require.NoError(t, err)
	assert.Equal(t, "w1", got.SessionID)
}

func TestInvade_SenhaErrada_CaiNoCaminhoNormalDeFaixa(t *testing.T) {
	pol := &fakePolicies{policy: domain.InvasionPolicy{Archetype: "host_world_dropin", InvasionEnabled: true, LevelRangeEnabled: true, PasswordEnabled: true}}
	dir := &fakeDirectory{worlds: []domain.InvadableWorld{
		{SessionID: "w1", HostSoulLevel: 500, FreeInvaderSlots: 1, PasswordHash: domain.HashWorldPassword("abrepalavra")},
	}}
	inj := &fakeInjector{byWorld: map[string]error{}}
	in := NewInvader(pol, dir, inj)

	_, err := in.Invade(context.Background(), "game-1", "default", "invader-1",
		domain.InvaderCriteria{SoulLevel: 100, Password: "senhaerrada"})
	assert.ErrorIs(t, err, domain.ErrNoEligibleWorld)
	assert.Empty(t, inj.calls)
}

func TestInvade_SenhaCertaMasPasswordEnabledFalso_NaoBypassa(t *testing.T) {
	// modo não declarou PasswordEnabled — a senha certa não deve isentar a faixa.
	pol := &fakePolicies{policy: domain.InvasionPolicy{Archetype: "host_world_dropin", InvasionEnabled: true, LevelRangeEnabled: true, PasswordEnabled: false}}
	dir := &fakeDirectory{worlds: []domain.InvadableWorld{
		{SessionID: "w1", HostSoulLevel: 500, FreeInvaderSlots: 1, PasswordHash: domain.HashWorldPassword("abrepalavra")},
	}}
	inj := &fakeInjector{byWorld: map[string]error{}}
	in := NewInvader(pol, dir, inj)

	_, err := in.Invade(context.Background(), "game-1", "default", "invader-1",
		domain.InvaderCriteria{SoulLevel: 100, Password: "abrepalavra"})
	assert.ErrorIs(t, err, domain.ErrNoEligibleWorld)
	assert.Empty(t, inj.calls)
}

func TestInvade_InvasaoDesligada_SenhaCertaNaoIsentaOPortao(t *testing.T) {
	pol := &fakePolicies{policy: domain.InvasionPolicy{Archetype: "host_world_dropin", InvasionEnabled: false}}
	dir := &fakeDirectory{worlds: []domain.InvadableWorld{
		{SessionID: "w1", FreeInvaderSlots: 1, PasswordHash: domain.HashWorldPassword("abrepalavra")},
	}}
	in := NewInvader(pol, dir, &fakeInjector{})

	_, err := in.Invade(context.Background(), "game-1", "default", "invader-1",
		domain.InvaderCriteria{Password: "abrepalavra"})
	assert.ErrorIs(t, err, domain.ErrInvasionNotSupported)
}

func TestInvade_PrimeiroEncheNaCorrida_TentaOProximo(t *testing.T) {
	pol := &fakePolicies{policy: invasionOn()}
	dir := &fakeDirectory{worlds: []domain.InvadableWorld{
		{SessionID: "w1", FreeInvaderSlots: 1},
		{SessionID: "w2", FreeInvaderSlots: 1},
	}}
	inj := &fakeInjector{byWorld: map[string]error{"w1": domain.ErrRoleFull}} // w1 encheu; w2 ok
	in := NewInvader(pol, dir, inj)

	got, err := in.Invade(context.Background(), "game-1", "default", "invader-1", domain.InvaderCriteria{SoulLevel: 100})
	require.NoError(t, err)
	assert.Equal(t, "w2", got.SessionID)
	assert.Equal(t, []string{"w1", "w2"}, inj.calls, "tentou w1 (cheio) e depois w2")
}

func TestInvade_MundoFechadoNaCorrida_TentaOProximo(t *testing.T) {
	pol := &fakePolicies{policy: invasionOn()}
	dir := &fakeDirectory{worlds: []domain.InvadableWorld{
		{SessionID: "w1", FreeInvaderSlots: 1},
		{SessionID: "w2", FreeInvaderSlots: 1},
	}}
	inj := &fakeInjector{byWorld: map[string]error{"w1": domain.ErrWorldNotFound}}
	in := NewInvader(pol, dir, inj)

	got, err := in.Invade(context.Background(), "game-1", "default", "invader-1", domain.InvaderCriteria{SoulLevel: 100})
	require.NoError(t, err)
	assert.Equal(t, "w2", got.SessionID)
}

func TestInvade_TodosCheios_ErrNoEligibleWorld(t *testing.T) {
	pol := &fakePolicies{policy: invasionOn()}
	dir := &fakeDirectory{worlds: []domain.InvadableWorld{
		{SessionID: "w1", FreeInvaderSlots: 1},
		{SessionID: "w2", FreeInvaderSlots: 1},
	}}
	inj := &fakeInjector{byWorld: map[string]error{"w1": domain.ErrRoleFull, "w2": domain.ErrRoleFull}}
	in := NewInvader(pol, dir, inj)

	_, err := in.Invade(context.Background(), "game-1", "default", "invader-1", domain.InvaderCriteria{SoulLevel: 100})
	assert.ErrorIs(t, err, domain.ErrNoEligibleWorld)
	assert.Equal(t, []string{"w1", "w2", "w1", "w2"}, inj.calls,
		"tentou os dois, rebuscou a lista (bounded, achado no /code-review) e tentou os dois de novo antes de desistir")
	assert.Equal(t, 2, dir.calls, "refetch é bounded a 2 chamadas de lista, não retry indefinido")
}

// Achado no /code-review ("JoinWorld race"): se TODOS os candidatos da 1ª
// lista perderem a corrida listar→injetar, um mundo elegível pode ter aberto
// nesse meio-tempo — sem o refetch isso seria um falso-negativo
// (ErrNoEligibleWorld mesmo havendo mundo disponível).
func TestInvade_TodosPerdemNaPrimeiraLista_RebuscaEAchaNaSegunda(t *testing.T) {
	pol := &fakePolicies{policy: invasionOn()}
	dir := &fakeDirectory{responses: [][]domain.InvadableWorld{
		{{SessionID: "w1", FreeInvaderSlots: 1}}, // 1ª lista: só w1, que perde a corrida
		{{SessionID: "w2", FreeInvaderSlots: 1}}, // 2ª lista (refetch): w2 abriu nesse meio-tempo
	}}
	inj := &fakeInjector{byWorld: map[string]error{"w1": domain.ErrRoleFull}}
	in := NewInvader(pol, dir, inj)

	got, err := in.Invade(context.Background(), "game-1", "default", "invader-1", domain.InvaderCriteria{SoulLevel: 100})
	require.NoError(t, err)
	assert.Equal(t, "w2", got.SessionID, "achou o mundo que abriu depois do 1º snapshot")
	assert.Equal(t, 2, dir.calls, "precisou rebuscar a lista pra achar w2")
}

func TestInvade_PerfilIndisponivel_Propaga(t *testing.T) {
	pol := &fakePolicies{err: domain.ErrProfileUnavailable}
	in := NewInvader(pol, &fakeDirectory{}, &fakeInjector{})

	_, err := in.Invade(context.Background(), "game-1", "default", "invader-1", domain.InvaderCriteria{})
	assert.ErrorIs(t, err, domain.ErrProfileUnavailable)
}

// Spec 012: o diretório é consultado no modo do invasor — a política aplicada
// é a desse modo, então os mundos candidatos também precisam ser dele.
func TestInvade_ListaMundosDoModoDoInvasor(t *testing.T) {
	pol := &fakePolicies{policy: invasionOn()}
	dir := &fakeDirectory{worlds: []domain.InvadableWorld{{SessionID: "w1", FreeInvaderSlots: 1}}}
	in := NewInvader(pol, dir, &fakeInjector{byWorld: map[string]error{}})

	_, err := in.Invade(context.Background(), "game-1", "batizado", "invader-1", domain.InvaderCriteria{})
	require.NoError(t, err)
	assert.Equal(t, "batizado", dir.gotMode)
}

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

type fakeDirectory struct {
	worlds []domain.InvadableWorld
	err    error
}

func (f *fakeDirectory) ListInvadableWorlds(context.Context, string) ([]domain.InvadableWorld, error) {
	return f.worlds, f.err
}

// fakeInjector devolve, por sessionID, o erro configurado (nil = sucesso) e
// registra a ordem das chamadas — para provar o retry no próximo mundo.
type fakeInjector struct {
	byWorld map[string]error
	calls   []string
}

func (f *fakeInjector) Inject(_ context.Context, sessionID, playerID, role string) (domain.Injected, error) {
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
	in := NewInvader(&fakePolicies{policy: invasionOn()}, &fakeDirectory{worlds: nil}, &fakeInjector{})

	_, err := in.Invade(context.Background(), "game-1", "default", "invader-1", domain.InvaderCriteria{SoulLevel: 100})
	assert.ErrorIs(t, err, domain.ErrNoEligibleWorld)
	assert.Equal(t, float64(1), in.InvasionsTotal(resultNoWorld))
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
	assert.Equal(t, []string{"w1", "w2"}, inj.calls, "tentou os dois antes de desistir")
}

func TestInvade_PerfilIndisponivel_Propaga(t *testing.T) {
	pol := &fakePolicies{err: domain.ErrProfileUnavailable}
	in := NewInvader(pol, &fakeDirectory{}, &fakeInjector{})

	_, err := in.Invade(context.Background(), "game-1", "default", "invader-1", domain.InvaderCriteria{})
	assert.ErrorIs(t, err, domain.ErrProfileUnavailable)
}

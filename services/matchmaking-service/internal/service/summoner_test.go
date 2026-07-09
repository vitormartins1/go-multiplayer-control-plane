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

// fakeCoopPolicies adapta uma política de co-op fixa à Port CoopPolicyProvider.
type fakeCoopPolicies struct {
	policy domain.CoopPolicy
	err    error
}

func (f *fakeCoopPolicies) CoopPolicy(context.Context, string, string) (domain.CoopPolicy, error) {
	return f.policy, f.err
}

// coopOn: co-op habilitado, sem filtro de faixa (todo sinal casa) — simplifica a
// orquestração. coopRange liga a faixa para os testes de compatibilidade.
func coopOn() domain.CoopPolicy {
	return domain.CoopPolicy{Archetype: "host_world_dropin", CoopEnabled: true, LevelRangeEnabled: false}
}

func coopRange() domain.CoopPolicy {
	return domain.CoopPolicy{Archetype: "host_world_dropin", CoopEnabled: true, LevelRangeEnabled: true}
}

// recvSummoned lê o canal do aliado com timeout; ok=false = canal fechado (EXPIRED).
func recvSummoned(t *testing.T, ch <-chan domain.Summoned) (domain.Summoned, bool) {
	t.Helper()
	select {
	case s, ok := <-ch:
		return s, ok
	case <-time.After(time.Second):
		t.Fatal("timeout esperando o canal do sinal")
		return domain.Summoned{}, false
	}
}

func TestPostSign_CoopOk_PublicaEDevolveCanal(t *testing.T) {
	s := NewSummoner(&fakeCoopPolicies{policy: coopOn()}, &fakeInjector{byWorld: map[string]error{}})

	signID, ch, err := s.PostSign(context.Background(), "game-1", "default", "ally-1", domain.CoopCriteria{SoulLevel: 100})
	require.NoError(t, err)
	assert.NotEmpty(t, signID)
	assert.NotNil(t, ch)
	assert.Equal(t, float64(1), s.SignsActive())
}

func TestPostSign_CoopNaoSuportado_ErrCoopNotSupported(t *testing.T) {
	s := NewSummoner(&fakeCoopPolicies{policy: domain.CoopPolicy{Archetype: "queue_arena"}}, &fakeInjector{})

	_, _, err := s.PostSign(context.Background(), "game-1", "default", "ally-1", domain.CoopCriteria{})
	assert.ErrorIs(t, err, domain.ErrCoopNotSupported)
	assert.Equal(t, float64(0), s.SignsActive())
}

func TestListSigns_FiltraPorFaixa(t *testing.T) {
	s := NewSummoner(&fakeCoopPolicies{policy: coopRange()}, &fakeInjector{byWorld: map[string]error{}})
	ctx := context.Background()
	// aliado A=100 → banda [80,120]; aliado B=300 fora do host 100.
	_, _, err := s.PostSign(ctx, "game-1", "default", "ally-a", domain.CoopCriteria{SoulLevel: 100})
	require.NoError(t, err)
	_, _, err = s.PostSign(ctx, "game-1", "default", "ally-b", domain.CoopCriteria{SoulLevel: 300})
	require.NoError(t, err)

	signs, err := s.ListSigns(ctx, "game-1", "default", domain.CoopCriteria{SoulLevel: 100})
	require.NoError(t, err)
	require.Len(t, signs, 1, "só o aliado na faixa do host")
	assert.Equal(t, "ally-a", signs[0].PlayerID)
}

func TestListSigns_SenhaBateForaDaFaixa_Aparece(t *testing.T) {
	s := NewSummoner(&fakeCoopPolicies{policy: coopRange()}, &fakeInjector{byWorld: map[string]error{}})
	ctx := context.Background()
	// aliado B=300 fora da banda do host 100, mas com a senha certa.
	_, _, err := s.PostSign(ctx, "game-1", "default", "ally-b", domain.CoopCriteria{SoulLevel: 300, Password: "abrepalavra"})
	require.NoError(t, err)

	signs, err := s.ListSigns(ctx, "game-1", "default", domain.CoopCriteria{SoulLevel: 100, Password: "abrepalavra"})
	require.NoError(t, err)
	require.Len(t, signs, 1)
	assert.Equal(t, "ally-b", signs[0].PlayerID)
}

func TestListSigns_SenhaErrada_CaiNoCaminhoNormalDeFaixa(t *testing.T) {
	s := NewSummoner(&fakeCoopPolicies{policy: coopRange()}, &fakeInjector{byWorld: map[string]error{}})
	ctx := context.Background()
	_, _, err := s.PostSign(ctx, "game-1", "default", "ally-b", domain.CoopCriteria{SoulLevel: 300, Password: "abrepalavra"})
	require.NoError(t, err)

	signs, err := s.ListSigns(ctx, "game-1", "default", domain.CoopCriteria{SoulLevel: 100, Password: "senhaerrada"})
	require.NoError(t, err)
	assert.Empty(t, signs, "senha errada não isenta a faixa")
}

func TestListSigns_OutroJogo_Exclui(t *testing.T) {
	s := NewSummoner(&fakeCoopPolicies{policy: coopOn()}, &fakeInjector{byWorld: map[string]error{}})
	ctx := context.Background()
	_, _, err := s.PostSign(ctx, "game-2", "default", "ally-x", domain.CoopCriteria{SoulLevel: 100})
	require.NoError(t, err)

	signs, err := s.ListSigns(ctx, "game-1", "default", domain.CoopCriteria{SoulLevel: 100})
	require.NoError(t, err)
	assert.Empty(t, signs)
}

func TestSummon_Sucesso_InjetaEntregaEConsome(t *testing.T) {
	s := NewSummoner(&fakeCoopPolicies{policy: coopOn()}, &fakeInjector{byWorld: map[string]error{}})
	ctx := context.Background()
	signID, ch, err := s.PostSign(ctx, "game-1", "default", "ally-1", domain.CoopCriteria{SoulLevel: 100})
	require.NoError(t, err)

	allyID, err := s.Summon(ctx, "world-7", signID)
	require.NoError(t, err)
	assert.Equal(t, "ally-1", allyID)

	got, ok := recvSummoned(t, ch)
	require.True(t, ok, "aliado recebe SUMMONED")
	assert.Equal(t, "world-7", got.SessionID)
	assert.Equal(t, "addr-world-7", got.ServerAddress)
	assert.Equal(t, "tok-ally-1", got.ConnectToken)

	assert.Equal(t, float64(0), s.SignsActive(), "sinal consumido")
	assert.Equal(t, float64(1), s.SummonsTotal(resultSummoned))
}

func TestSummon_SinalInexistente_ErrSignNotFound(t *testing.T) {
	s := NewSummoner(&fakeCoopPolicies{policy: coopOn()}, &fakeInjector{byWorld: map[string]error{}})
	_, err := s.Summon(context.Background(), "world-1", "nope")
	assert.ErrorIs(t, err, domain.ErrSignNotFound)
}

func TestSummon_JaConsumido_ErrSignConsumed(t *testing.T) {
	s := NewSummoner(&fakeCoopPolicies{policy: coopOn()}, &fakeInjector{byWorld: map[string]error{}})
	ctx := context.Background()
	signID, _, err := s.PostSign(ctx, "game-1", "default", "ally-1", domain.CoopCriteria{SoulLevel: 100})
	require.NoError(t, err)
	_, err = s.Summon(ctx, "world-1", signID)
	require.NoError(t, err)

	_, err = s.Summon(ctx, "world-2", signID)
	assert.ErrorIs(t, err, domain.ErrSignConsumed)
}

func TestSummon_MundoCheio_NaoConsome(t *testing.T) {
	s := NewSummoner(&fakeCoopPolicies{policy: coopOn()}, &fakeInjector{byWorld: map[string]error{"world-1": domain.ErrRoleFull}})
	ctx := context.Background()
	signID, _, err := s.PostSign(ctx, "game-1", "default", "ally-1", domain.CoopCriteria{SoulLevel: 100})
	require.NoError(t, err)

	_, err = s.Summon(ctx, "world-1", signID)
	assert.ErrorIs(t, err, domain.ErrRoleFull)
	assert.Equal(t, float64(1), s.SignsActive(), "sinal segue disponível")

	// e ainda pode ser invocado por outro mundo (com vaga).
	s.injector = &fakeInjector{byWorld: map[string]error{}}
	allyID, err := s.Summon(ctx, "world-2", signID)
	require.NoError(t, err)
	assert.Equal(t, "ally-1", allyID)
}

func TestSummon_Corrida_UmSoConsome(t *testing.T) {
	s := NewSummoner(&fakeCoopPolicies{policy: coopOn()}, &fakeInjector{byWorld: map[string]error{}})
	ctx := context.Background()
	signID, _, err := s.PostSign(ctx, "game-1", "default", "ally-1", domain.CoopCriteria{SoulLevel: 100})
	require.NoError(t, err)

	var wg sync.WaitGroup
	var mu sync.Mutex
	var ok, consumed int
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.Summon(ctx, "world-1", signID)
			mu.Lock()
			switch {
			case err == nil:
				ok++
			case errors.Is(err, domain.ErrSignConsumed):
				consumed++
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
	assert.Equal(t, 1, ok, "exatamente um host injeta")
	assert.Equal(t, 7, consumed, "os outros veem ErrSignConsumed")
	assert.Equal(t, float64(0), s.SignsActive(), "sinal consumido uma vez")
}

func TestCancelSign_RemoveDoQuadro(t *testing.T) {
	s := NewSummoner(&fakeCoopPolicies{policy: coopOn()}, &fakeInjector{byWorld: map[string]error{}})
	ctx := context.Background()
	signID, ch, err := s.PostSign(ctx, "game-1", "default", "ally-1", domain.CoopCriteria{SoulLevel: 100})
	require.NoError(t, err)

	s.CancelSign(signID)
	assert.Equal(t, float64(0), s.SignsActive())
	_, ok := recvSummoned(t, ch)
	assert.False(t, ok, "canal fechado no cancelamento")
	assert.Equal(t, float64(1), s.SummonsTotal(resultCancelled))
}

func TestSweepExpired_FechaCanalEConta(t *testing.T) {
	s := NewSummoner(&fakeCoopPolicies{policy: coopOn()}, &fakeInjector{byWorld: map[string]error{}})
	t0 := time.Now()
	s.now = func() time.Time { return t0 }
	signID, ch, err := s.PostSign(context.Background(), "game-1", "default", "ally-1", domain.CoopCriteria{SoulLevel: 100})
	require.NoError(t, err)
	require.NotEmpty(t, signID)

	s.now = func() time.Time { return t0.Add(10 * time.Second) } // avança o relógio
	n := s.SweepExpired(5 * time.Second)
	assert.Equal(t, 1, n)
	assert.Equal(t, float64(0), s.SignsActive())
	_, ok := recvSummoned(t, ch)
	assert.False(t, ok, "canal fechado na expiração (EXPIRED)")
	assert.Equal(t, float64(1), s.SummonsTotal(resultExpired))
}

// Sinal anunciado num modo não aparece para host de outro modo (ADR-0024).
func TestListSigns_FiltraPorModo(t *testing.T) {
	s := NewSummoner(&fakeCoopPolicies{policy: coopOn()}, &fakeInjector{byWorld: map[string]error{}})
	_, _, err := s.PostSign(context.Background(), "game-1", "tdm", "ally-1", domain.CoopCriteria{SoulLevel: 50})
	require.NoError(t, err)

	outros, err := s.ListSigns(context.Background(), "game-1", "ranked", domain.CoopCriteria{SoulLevel: 50})
	require.NoError(t, err)
	assert.Empty(t, outros, "modo diferente não vê o sinal")

	mesmos, err := s.ListSigns(context.Background(), "game-1", "tdm", domain.CoopCriteria{SoulLevel: 50})
	require.NoError(t, err)
	require.Len(t, mesmos, 1)
	assert.Equal(t, "tdm", mesmos[0].ModeID)
}

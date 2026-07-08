package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/leaderboard-service/internal/domain"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/leaderboard-service/internal/service"
	"github.com/vitormartins1/go-multiplayer-control-plane/shared/logger"
)

// submitCall registra um highscore aplicado (board/player/score).
type submitCall struct {
	board    string
	playerID string
	score    int64
}

// fakeStore captura os Submit (highscore) e os Increment (cumulative) para provar
// o mapeamento entrada→boards do handler, sem Redis. Rank devolve zero (o handler
// não usa o rank).
type fakeStore struct {
	submits    []submitCall
	incrs      []domain.Increment
	cumMatchID string
	dup        bool
}

func (f *fakeStore) Submit(_ context.Context, _, board, playerID string, s int64) error {
	f.submits = append(f.submits, submitCall{board: board, playerID: playerID, score: s})
	return nil
}
func (f *fakeStore) ForceScore(context.Context, string, string, string, int64) error { return nil }
func (f *fakeStore) Rank(context.Context, string, string, string, string) (int64, int64, error) {
	return 0, 0, nil
}
func (f *fakeStore) Top(context.Context, string, string, string, int) ([]domain.Entry, error) {
	return nil, nil
}
func (f *fakeStore) Range(context.Context, string, string, string, int64, int64) ([]domain.Entry, error) {
	return nil, nil
}
func (f *fakeStore) ApplyCumulative(_ context.Context, _, matchID string, incrs []domain.Increment) (bool, error) {
	f.cumMatchID = matchID
	f.incrs = append(f.incrs, incrs...)
	return f.dup, nil
}

func newHandler(t *testing.T) (*fakeStore, func(body []byte) (bool, error)) {
	t.Helper()
	store := &fakeStore{}
	h := makeHandler(service.New(store), logger.New("test"))
	return store, func(body []byte) (bool, error) { return h(context.Background(), body) }
}

// hasIncr diz se há uma parcela cumulativa (board, player, delta) entre as capturadas.
func hasIncr(incrs []domain.Increment, board, player string, delta int64) bool {
	for _, i := range incrs {
		if i.Board == board && i.PlayerID == player && i.Delta == delta {
			return true
		}
	}
	return false
}

func TestMakeHandler_Placement_1oLugar_WinsETopNESurvival(t *testing.T) {
	store, handle := newHandler(t)
	_, err := handle([]byte(`{"kind":"placement","matchId":"m1","gameId":"g","entries":[{"playerId":"p1","placement":1,"survivedMs":900000,"stats":{"kills":4}}]}`))
	require.NoError(t, err)

	assert.Equal(t, "m1", store.cumMatchID)
	assert.True(t, hasIncr(store.incrs, domain.BoardWins, "p1", 1), "1º lugar conta vitória")
	assert.True(t, hasIncr(store.incrs, domain.BoardTopNFinishes, "p1", 1), "1º lugar também é top N")
	assert.True(t, hasIncr(store.incrs, domain.TotalBoard("kills"), "p1", 4), "kills_total cumulativo")
	// highscore: survived_ms e kills submetidos.
	assert.Contains(t, store.submits, submitCall{board: domain.BoardSurvivedMs, playerID: "p1", score: 900000})
	assert.Contains(t, store.submits, submitCall{board: "kills", playerID: "p1", score: 4})
}

func TestMakeHandler_Placement_TopN_SemWin(t *testing.T) {
	store, handle := newHandler(t)
	_, err := handle([]byte(`{"kind":"placement","matchId":"m1","gameId":"g","entries":[{"playerId":"p2","placement":7,"survivedMs":500000},{"playerId":"p3","placement":50,"survivedMs":100000}]}`))
	require.NoError(t, err)

	assert.True(t, hasIncr(store.incrs, domain.BoardTopNFinishes, "p2", 1), "colocação 7 é top N")
	assert.False(t, hasIncr(store.incrs, domain.BoardWins, "p2", 1), "colocação 7 não é vitória")
	assert.False(t, hasIncr(store.incrs, domain.BoardTopNFinishes, "p3", 1), "colocação 50 fora do top N")
	assert.False(t, hasIncr(store.incrs, domain.BoardWins, "p3", 1))
}

func TestMakeHandler_Placement_Duplicado_PropagaDup(t *testing.T) {
	store, handle := newHandler(t)
	store.dup = true
	dup, err := handle([]byte(`{"kind":"placement","matchId":"m1","gameId":"g","entries":[{"playerId":"p1","placement":1}]}`))
	require.NoError(t, err)
	assert.True(t, dup, "reprocessar a mesma partida é no-op (dup do ApplyCumulative)")
}

// coop_mission (C3): missão cumprida → todos ganham missions_completed +1; os
// stats por jogador viram highscore + <stat>_total.
func TestMakeHandler_Collective_Sucesso_MissionsCompletedParaTodos(t *testing.T) {
	store, handle := newHandler(t)
	_, err := handle([]byte(`{"kind":"collective","matchId":"m1","gameId":"g","outcome":"MISSION_SUCCESS","players":[{"playerId":"p1","stats":{"kills":5}},{"playerId":"p2","stats":{"revives":2}}]}`))
	require.NoError(t, err)

	assert.Equal(t, "m1", store.cumMatchID)
	assert.True(t, hasIncr(store.incrs, domain.BoardMissionsCompleted, "p1", 1), "missão cumprida conta para todo o time")
	assert.True(t, hasIncr(store.incrs, domain.BoardMissionsCompleted, "p2", 1))
	assert.True(t, hasIncr(store.incrs, domain.TotalBoard("kills"), "p1", 5), "kills_total cumulativo")
	assert.True(t, hasIncr(store.incrs, domain.TotalBoard("revives"), "p2", 2))
	assert.Contains(t, store.submits, submitCall{board: "kills", playerID: "p1", score: 5})
}

// Falha da missão: os stats ainda contam, mas missions_completed não incrementa.
func TestMakeHandler_Collective_Falha_SemMissionsCompleted(t *testing.T) {
	store, handle := newHandler(t)
	_, err := handle([]byte(`{"kind":"collective","matchId":"m1","gameId":"g","outcome":"MISSION_FAILURE","players":[{"playerId":"p1","stats":{"kills":3}}]}`))
	require.NoError(t, err)

	assert.False(t, hasIncr(store.incrs, domain.BoardMissionsCompleted, "p1", 1), "falha não conta missão cumprida")
	assert.True(t, hasIncr(store.incrs, domain.TotalBoard("kills"), "p1", 3), "performance vale mesmo na falha")
}

func TestMakeHandler_Collective_Duplicado_PropagaDup(t *testing.T) {
	store, handle := newHandler(t)
	store.dup = true
	dup, err := handle([]byte(`{"kind":"collective","matchId":"m1","gameId":"g","outcome":"MISSION_SUCCESS","players":[{"playerId":"p1"}]}`))
	require.NoError(t, err)
	assert.True(t, dup, "reprocessar a mesma missão é no-op")
}

func TestMakeHandler_WinLossDraw_SemRegressao(t *testing.T) {
	store, handle := newHandler(t)
	_, err := handle([]byte(`{"kind":"win_loss_draw","matchId":"m1","gameId":"g","results":[{"playerId":"p1","outcome":"win","score":10,"stats":{"kills":2}}]}`))
	require.NoError(t, err)

	assert.Contains(t, store.submits, submitCall{board: domain.BoardScore, playerID: "p1", score: 10})
	assert.True(t, hasIncr(store.incrs, domain.BoardWins, "p1", 1))
	assert.True(t, hasIncr(store.incrs, domain.TotalBoard("kills"), "p1", 2))
}

func TestMakeHandler_V1LegadoSemKind_SemRegressao(t *testing.T) {
	store, handle := newHandler(t)
	_, err := handle([]byte(`{"matchId":"m1","gameId":"g","results":[{"playerId":"p1","score":5}]}`))
	require.NoError(t, err)
	assert.Contains(t, store.submits, submitCall{board: domain.BoardScore, playerID: "p1", score: 5})
}

func TestMakeHandler_KindSemRanking_Descarta(t *testing.T) {
	store, handle := newHandler(t)
	// participation_event não tem matchId nem results — não é rankeável.
	dup, err := handle([]byte(`{"kind":"participation_event","gameId":"g","sessionId":"w1","playerId":"p1","event":"HOST_KILLED"}`))
	require.NoError(t, err, "kind sem ranking não é erro (não vai à DLQ)")
	assert.False(t, dup)
	assert.Empty(t, store.submits, "nada é rankeado")
	assert.Empty(t, store.incrs)
}

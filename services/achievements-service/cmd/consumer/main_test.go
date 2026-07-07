package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/achievements-service/internal/domain"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/achievements-service/internal/service"
	"github.com/vitormartins1/go-multiplayer-control-plane/shared/logger"
)

// dispatchRepo registra qual caminho de aplicação foi chamado — só o suficiente
// para provar o dispatch por kind do makeHandler. Os demais métodos da Port são
// stubs (não exercidos aqui).
type dispatchRepo struct {
	matchCalled         bool
	participationCalled bool
	gotEvent            string
	gotPlayers          []domain.PlayerMatchInput
}

func (r *dispatchRepo) ApplyMatchResult(_ context.Context, _, _ string, players []domain.PlayerMatchInput, _ time.Time) (bool, error) {
	r.matchCalled = true
	r.gotPlayers = players
	return false, nil
}

func (r *dispatchRepo) ApplyParticipationEvent(_ context.Context, _, _, _, event string, _ []string, _ time.Time) (bool, error) {
	r.participationCalled = true
	r.gotEvent = event
	return false, nil
}

func (r *dispatchRepo) ListCatalog(context.Context, string) ([]domain.Achievement, error) {
	return nil, nil
}
func (r *dispatchRepo) FindByEventType(context.Context, string, string) ([]domain.Achievement, error) {
	return nil, nil
}
func (r *dispatchRepo) FindByStatKey(context.Context, string, string) ([]domain.Achievement, error) {
	return nil, nil
}
func (r *dispatchRepo) ListPlayerProgress(context.Context, string, string) ([]domain.PlayerProgress, error) {
	return nil, nil
}
func (r *dispatchRepo) ProgressForAchievement(context.Context, string, string) ([]domain.AchievementProgressEntry, error) {
	return nil, nil
}
func (r *dispatchRepo) RecordProgress(context.Context, string, string, string, int, time.Time) (bool, error) {
	return false, nil
}
func (r *dispatchRepo) CreateAchievement(context.Context, *domain.Achievement) error { return nil }
func (r *dispatchRepo) UpdateAchievement(context.Context, *domain.Achievement) error { return nil }
func (r *dispatchRepo) DeleteAchievement(context.Context, string, string) error       { return nil }

func newHandler(t *testing.T) (*dispatchRepo, func(body []byte) (bool, error)) {
	t.Helper()
	repo := &dispatchRepo{}
	h := makeHandler(service.New(repo), logger.New("test"))
	return repo, func(body []byte) (bool, error) { return h(context.Background(), body) }
}

func TestMakeHandler_ParticipationEvent_RoteiaParaParticipation(t *testing.T) {
	repo, handle := newHandler(t)
	_, err := handle([]byte(`{"kind":"participation_event","gameId":"g","sessionId":"w1","playerId":"inv-1","role":"invader","event":"HOST_KILLED","outcome":"SUCCESS"}`))
	require.NoError(t, err)
	assert.True(t, repo.participationCalled)
	assert.False(t, repo.matchCalled)
	assert.Equal(t, "HOST_KILLED", repo.gotEvent)
}

func TestMakeHandler_WinLossDraw_RoteiaParaMatchResult(t *testing.T) {
	repo, handle := newHandler(t)
	_, err := handle([]byte(`{"kind":"win_loss_draw","matchId":"m1","gameId":"g","results":[{"playerId":"p1","outcome":"win"}]}`))
	require.NoError(t, err)
	assert.True(t, repo.matchCalled)
	assert.False(t, repo.participationCalled)
}

func TestMakeHandler_V1SemKindComMatchId_RoteiaParaMatchResult(t *testing.T) {
	repo, handle := newHandler(t)
	_, err := handle([]byte(`{"matchId":"m1","gameId":"g","results":[{"playerId":"p1","won":true}]}`))
	require.NoError(t, err)
	assert.True(t, repo.matchCalled, "envelope v1 legado ainda vai para o match result")
}

func TestMakeHandler_Placement_RoteiaParaPlacement(t *testing.T) {
	repo, handle := newHandler(t)
	_, err := handle([]byte(`{"kind":"placement","matchId":"m1","gameId":"g","entries":[{"playerId":"p1","placement":1,"survivedMs":900000,"stats":{"kills":4}},{"playerId":"p2","placement":42,"survivedMs":100000}]}`))
	require.NoError(t, err)
	assert.True(t, repo.matchCalled, "placement usa o caminho ApplyMatchResult (idempotente por matchId)")
	assert.False(t, repo.participationCalled)
	require.Len(t, repo.gotPlayers, 2)
	assert.Equal(t, []string{"battle_royale_played", "battle_royale_win", "battle_royale_top_n"}, repo.gotPlayers[0].EventTypes)
	assert.Equal(t, int64(900000), repo.gotPlayers[0].Stats["survived_ms"], "survivedMs exposto como stat")
	assert.Equal(t, []string{"battle_royale_played"}, repo.gotPlayers[1].EventTypes, "fora do top N só played")
}

func TestMakeHandler_KindDesconhecido_Descarta(t *testing.T) {
	repo, handle := newHandler(t)
	dup, err := handle([]byte(`{"kind":"summon_posted","gameId":"g"}`))
	require.NoError(t, err, "kind desconhecido não é erro (não vai à DLQ)")
	assert.False(t, dup)
	assert.False(t, repo.matchCalled)
	assert.False(t, repo.participationCalled)
}

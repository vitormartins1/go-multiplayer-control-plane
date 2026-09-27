package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/achievements-service/internal/domain"
)

// spec 019 — espelhamento na Steam.

type mockSteamRepo struct{ mock.Mock }

func (m *mockSteamRepo) ClaimSteamPending(ctx context.Context, gameID string, limit int, now, leaseUntil time.Time) ([]domain.SteamSyncItem, error) {
	args := m.Called(ctx, gameID, limit, now, leaseUntil)
	items, _ := args.Get(0).([]domain.SteamSyncItem)
	return items, args.Error(1)
}

func (m *mockSteamRepo) MarkSteamDone(ctx context.Context, it domain.SteamSyncItem, state, reason string, at time.Time) error {
	return m.Called(ctx, it, state, reason, at).Error(0)
}

func (m *mockSteamRepo) MarkSteamRetry(ctx context.Context, it domain.SteamSyncItem, nextAt time.Time, reason string) error {
	return m.Called(ctx, it, nextAt, reason).Error(0)
}

type mockSteamIDs struct{ mock.Mock }

func (m *mockSteamIDs) SteamID(ctx context.Context, playerID string) (string, error) {
	args := m.Called(ctx, playerID)
	return args.String(0), args.Error(1)
}

type mockSteamSetter struct{ mock.Mock }

func (m *mockSteamSetter) SetAchievement(ctx context.Context, steamID, apiName string) error {
	return m.Called(ctx, steamID, apiName).Error(0)
}

var steamNow = time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)

func newTestMirror(item domain.SteamSyncItem) (*SteamMirror, *mockSteamRepo, *mockSteamIDs, *mockSteamSetter) {
	repo, ids, setter := &mockSteamRepo{}, &mockSteamIDs{}, &mockSteamSetter{}
	repo.On("ClaimSteamPending", mock.Anything, "folclore", steamBatch, steamNow, steamNow.Add(steamLease)).
		Return([]domain.SteamSyncItem{item}, nil)
	m := NewSteamMirror(repo, ids, setter, "folclore", nil)
	m.now = func() time.Time { return steamNow }
	return m, repo, ids, setter
}

func item(attempts int) domain.SteamSyncItem {
	return domain.SteamSyncItem{GameID: "folclore", PlayerID: "p1", AchievementID: "boss", SteamAPIName: "ACH_BOSS", Attempts: attempts}
}

func TestSteamMirror_Sucesso_MarcaSynced(t *testing.T) {
	m, repo, ids, setter := newTestMirror(item(0))
	ids.On("SteamID", mock.Anything, "p1").Return("7656", nil)
	setter.On("SetAchievement", mock.Anything, "7656", "ACH_BOSS").Return(nil)
	repo.On("MarkSteamDone", mock.Anything, item(0), domain.SteamSyncSynced, "", steamNow).Return(nil)

	n, err := m.SyncOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	repo.AssertExpectations(t)
	assert.Equal(t, float64(1), m.SyncTotal(SteamResultSynced))
}

func TestSteamMirror_SemContaSteam_Pula(t *testing.T) {
	m, repo, ids, setter := newTestMirror(item(0))
	ids.On("SteamID", mock.Anything, "p1").Return("", domain.ErrNoSteamAccount)
	repo.On("MarkSteamDone", mock.Anything, item(0), domain.SteamSyncSkipped, mock.Anything, steamNow).Return(nil)

	_, err := m.SyncOnce(context.Background())
	require.NoError(t, err)
	setter.AssertNotCalled(t, "SetAchievement", mock.Anything, mock.Anything, mock.Anything)
	assert.Equal(t, float64(1), m.SyncTotal(SteamResultSkipped))
}

func TestSteamMirror_SteamFalha_ReagendaComBackoff(t *testing.T) {
	m, repo, ids, setter := newTestMirror(item(2)) // 3ª tentativa
	ids.On("SteamID", mock.Anything, "p1").Return("7656", nil)
	setter.On("SetAchievement", mock.Anything, "7656", "ACH_BOSS").Return(errors.New("steam: http 503"))
	repo.On("MarkSteamRetry", mock.Anything, item(2), steamNow.Add(4*time.Minute), "steam: http 503").Return(nil)

	_, err := m.SyncOnce(context.Background())
	require.NoError(t, err)
	repo.AssertExpectations(t)
	assert.Equal(t, float64(1), m.SyncTotal(SteamResultRetry))
}

func TestSteamMirror_IdentityFora_Reagenda(t *testing.T) {
	m, repo, ids, _ := newTestMirror(item(0))
	ids.On("SteamID", mock.Anything, "p1").Return("", errors.New("http 500"))
	repo.On("MarkSteamRetry", mock.Anything, item(0), steamNow.Add(time.Minute), mock.Anything).Return(nil)

	_, err := m.SyncOnce(context.Background())
	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestSteamMirror_UltimaTentativa_Desiste(t *testing.T) {
	m, repo, ids, setter := newTestMirror(item(steamMaxAttempts - 1))
	ids.On("SteamID", mock.Anything, "p1").Return("7656", nil)
	setter.On("SetAchievement", mock.Anything, "7656", "ACH_BOSS").Return(errors.New("steam: result 8"))
	repo.On("MarkSteamDone", mock.Anything, item(steamMaxAttempts-1), domain.SteamSyncFailed, "steam: result 8", steamNow).Return(nil)

	_, err := m.SyncOnce(context.Background())
	require.NoError(t, err)
	repo.AssertExpectations(t)
	assert.Equal(t, float64(1), m.SyncTotal(SteamResultFailed))
}

func TestSteamMirror_ClaimFalha_DevolveErro(t *testing.T) {
	repo := &mockSteamRepo{}
	repo.On("ClaimSteamPending", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("db fora"))
	_, err := NewSteamMirror(repo, &mockSteamIDs{}, &mockSteamSetter{}, "g", nil).SyncOnce(context.Background())
	assert.Error(t, err)
}

// Falha ao gravar o estado não derruba a rodada nem conta como sucesso.
func TestSteamMirror_FalhaAoMarcar_NaoConta(t *testing.T) {
	m, repo, ids, setter := newTestMirror(item(0))
	ids.On("SteamID", mock.Anything, "p1").Return("7656", nil)
	setter.On("SetAchievement", mock.Anything, "7656", "ACH_BOSS").Return(nil)
	repo.On("MarkSteamDone", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("db"))

	_, err := m.SyncOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, float64(0), m.SyncTotal(SteamResultSynced))
}

func TestSteamBackoff(t *testing.T) {
	assert.Equal(t, time.Minute, steamBackoff(1))
	assert.Equal(t, 2*time.Minute, steamBackoff(2))
	assert.Equal(t, 32*time.Minute, steamBackoff(6))
	assert.Equal(t, time.Hour, steamBackoff(7))
	assert.Equal(t, time.Hour, steamBackoff(20))
	assert.Equal(t, time.Hour, steamBackoff(100), "sem estouro do shift")
}

// Revisão: lease perdido (outra réplica pegou o item) não é contado nem é erro.
func TestSteamMirror_LeasePerdido_NaoConta(t *testing.T) {
	m, repo, ids, setter := newTestMirror(item(0))
	ids.On("SteamID", mock.Anything, "p1").Return("7656", nil)
	setter.On("SetAchievement", mock.Anything, "7656", "ACH_BOSS").Return(nil)
	repo.On("MarkSteamDone", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domain.ErrSteamLeaseLost)

	_, err := m.SyncOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, float64(0), m.SyncTotal(SteamResultSynced))
}

func TestSteamMirror_Run_ParaComContexto(t *testing.T) {
	repo := &mockSteamRepo{}
	repo.On("ClaimSteamPending", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	m := NewSteamMirror(repo, &mockSteamIDs{}, &mockSteamSetter{}, "g", nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { m.Run(ctx, time.Millisecond); close(done) }()
	time.Sleep(10 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run não parou com o contexto cancelado")
	}
}

// spec 019: formato do API Name do Steamworks no catálogo.
func TestCreateAchievement_SteamAPIName(t *testing.T) {
	ok := domain.Achievement{ID: "boss", Name: "Boss", EventType: "boss_killed", SteamAPIName: " ACH_BOSS_1 "}
	require.NoError(t, validateCriterion(&ok))
	assert.Equal(t, "ACH_BOSS_1", ok.SteamAPIName, "espaços em volta saem")

	bad := domain.Achievement{ID: "boss", Name: "Boss", EventType: "boss_killed", SteamAPIName: "ACH BOSS!"}
	assert.ErrorIs(t, validateCriterion(&bad), domain.ErrInvalidAchievement)
}

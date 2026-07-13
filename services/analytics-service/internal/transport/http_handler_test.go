package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/analytics-service/internal/domain"
)

type fakeReader struct {
	ov      domain.Overview
	ga      domain.GameAnalytics
	gotGame string
	gotDays int
}

func (f *fakeReader) Overview(context.Context) (domain.Overview, error) { return f.ov, nil }
func (f *fakeReader) GameAnalytics(_ context.Context, gameID string, days int) (domain.GameAnalytics, error) {
	f.gotGame, f.gotDays = gameID, days
	return f.ga, nil
}

func newMux(r Reader) *http.ServeMux {
	mux := http.NewServeMux()
	NewAdminHandler(r).Register(mux)
	return mux
}

func TestOverview_200_SerializaTotaisEJogos(t *testing.T) {
	r := &fakeReader{ov: domain.Overview{
		Totals: domain.Totals{Matches: 3, UniquePlayers: 2, Games: 1},
		Games:  []domain.GameStat{{GameID: "g1", Matches: 3, UniquePlayers: 2, Wins: 1, AvgScore: 4.5}},
	}}
	rec := httptest.NewRecorder()
	newMux(r).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/analytics/overview", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Totals struct {
			Matches int64 `json:"matches"`
			Games   int64 `json:"games"`
		} `json:"totals"`
		Games []struct {
			GameID   string  `json:"gameId"`
			AvgScore float64 `json:"avgScore"`
		} `json:"games"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, int64(3), body.Totals.Matches)
	require.Len(t, body.Games, 1)
	assert.Equal(t, "g1", body.Games[0].GameID)
	assert.Equal(t, 4.5, body.Games[0].AvgScore)
}

func TestGameAnalytics_200_RepassaDaysEDaily(t *testing.T) {
	r := &fakeReader{ga: domain.GameAnalytics{
		GameStat:           domain.GameStat{GameID: "g1", Matches: 2},
		Daily:              []domain.DailyPoint{{Day: "2026-07-04", Matches: 2}},
		RoleParticipations: []domain.ParticipationStat{{Role: "invader", Event: "HOST_KILLED", Outcome: "SUCCESS", Count: 3}},
	}}
	rec := httptest.NewRecorder()
	newMux(r).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/games/g1/analytics?days=7", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "g1", r.gotGame)
	assert.Equal(t, 7, r.gotDays)
	var body struct {
		Matches int64 `json:"matches"`
		Daily   []struct {
			Day     string `json:"day"`
			Matches int64  `json:"matches"`
		} `json:"daily"`
		RoleParticipations []struct {
			Role    string `json:"role"`
			Event   string `json:"event"`
			Outcome string `json:"outcome"`
			Count   int64  `json:"count"`
		} `json:"roleParticipations"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, int64(2), body.Matches)
	require.Len(t, body.Daily, 1)
	assert.Equal(t, "2026-07-04", body.Daily[0].Day)
	require.Len(t, body.RoleParticipations, 1)
	assert.Equal(t, "invader", body.RoleParticipations[0].Role)
	assert.Equal(t, "SUCCESS", body.RoleParticipations[0].Outcome, "outcome não pode se perder no DTO (achado 2026-07-10)")
	assert.Equal(t, int64(3), body.RoleParticipations[0].Count)
}

func TestGameAnalytics_QueueArena_RoleParticipationsVazio(t *testing.T) {
	r := &fakeReader{ga: domain.GameAnalytics{GameStat: domain.GameStat{GameID: "g1", Matches: 5}}}
	rec := httptest.NewRecorder()
	newMux(r).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/games/g1/analytics", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"roleParticipations":[]`, "campo aditivo, vazio no queue_arena")
	assert.Contains(t, rec.Body.String(), `"placementDaily":[]`, "campo aditivo, vazio fora do BR")
	assert.Contains(t, rec.Body.String(), `"missionDaily":[]`, "campo aditivo, vazio fora do coop")
	assert.Contains(t, rec.Body.String(), `"participationDaily":[]`, "campo aditivo, vazio fora do HWD")
}

// séries diárias novas (spec 010): expostas no DTO, uma por arquétipo.
func TestGameAnalytics_200_ExpoeSeriesDiariasNovas(t *testing.T) {
	r := &fakeReader{ga: domain.GameAnalytics{
		GameStat:           domain.GameStat{GameID: "g1"},
		PlacementDaily:     []domain.DailyPoint{{Day: "2026-07-13", Matches: 3}},
		MissionDaily:       []domain.DailyPoint{{Day: "2026-07-13", Matches: 1}},
		ParticipationDaily: []domain.DailyPoint{{Day: "2026-07-13", Matches: 42}},
	}}
	rec := httptest.NewRecorder()
	newMux(r).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/games/g1/analytics", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		PlacementDaily     []struct{ Matches int64 } `json:"placementDaily"`
		MissionDaily       []struct{ Matches int64 } `json:"missionDaily"`
		ParticipationDaily []struct{ Matches int64 } `json:"participationDaily"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.PlacementDaily, 1)
	assert.Equal(t, int64(3), body.PlacementDaily[0].Matches)
	require.Len(t, body.MissionDaily, 1)
	assert.Equal(t, int64(1), body.MissionDaily[0].Matches)
	require.Len(t, body.ParticipationDaily, 1)
	assert.Equal(t, int64(42), body.ParticipationDaily[0].Matches)
}

// modeId (ADR-0024, spec 009): o DTO expõe o modo de cada linha de participação,
// permitindo o admin-ui filtrar/agrupar por modo nomeado do host_world_dropin.
func TestGameAnalytics_200_ExpoeModeID(t *testing.T) {
	r := &fakeReader{ga: domain.GameAnalytics{
		GameStat: domain.GameStat{GameID: "g1", Matches: 2},
		RoleParticipations: []domain.ParticipationStat{
			{ModeID: "batizado", Role: "invader", Event: "HOST_KILLED", Outcome: "SUCCESS", Count: 3},
			{ModeID: "", Role: "invader", Event: "HOST_KILLED", Outcome: "SUCCESS", Count: 1},
		},
	}}
	rec := httptest.NewRecorder()
	newMux(r).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/games/g1/analytics", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		RoleParticipations []struct {
			ModeID string `json:"modeId"`
			Role   string `json:"role"`
			Count  int64  `json:"count"`
		} `json:"roleParticipations"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.RoleParticipations, 2)
	assert.Equal(t, "batizado", body.RoleParticipations[0].ModeID)
	assert.Equal(t, "", body.RoleParticipations[1].ModeID, "sem modo (dado anterior à spec 009) cai no bucket vazio")
}

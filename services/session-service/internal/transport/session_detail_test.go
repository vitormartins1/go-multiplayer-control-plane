package transport

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/domain"
)

// Spec 018: o DTO admin carrega snapshot do modo, formação e resultado.
func TestAdminSessions_DetalheRico_SnapshotFormacaoResultado(t *testing.T) {
	opened := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	dir := &fakeDirectory{
		closed: []domain.Session{{
			ID: "s1", MatchID: "m1", GameID: "g1", ModeID: "teams-2v2",
			Players: []string{"p1", "p2", "p3", "p4"}, State: domain.SessionClosed,
			OpenedAt: opened, ClosedAt: opened.Add(time.Minute), CloseReason: domain.CloseResult,
			Profile: &domain.ProfileSnapshot{
				ModeID: "teams-2v2", Archetype: domain.ArchetypeQueueArena, Topology: "dedicated",
				ParamsJSON: []byte(`{"structure":"teams","teamCount":2,"teamSize":2,"scoreTarget":5}`),
				Hash:       "abc123",
			},
			TeamByPlayer: map[string]string{"p1": "team-1", "p2": "team-1", "p3": "team-2", "p4": "team-2"},
			Result: &domain.MatchOutcome{
				Kind: domain.OutcomeWinLossDraw,
				WinLossDraw: &domain.WinLossDrawResult{Players: []domain.PlayerOutcome{
					{PlayerID: "p1", TeamID: "team-1", Outcome: domain.ResultWin, Score: 4, Stats: map[string]int64{"orbs": 4}},
					{PlayerID: "p3", TeamID: "team-2", Outcome: domain.ResultLoss, Score: 1},
				}},
			},
		}},
	}

	rec, body := doGet(t, newAdminMux(dir), "/admin/games/g1/sessions")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, body.Closed, 1)
	s := body.Closed[0]

	assert.Equal(t, domain.ArchetypeQueueArena, s.Archetype)
	assert.Equal(t, "abc123", s.ProfileHash)
	var params map[string]any
	require.NoError(t, json.Unmarshal(s.ModeParams, &params))
	assert.Equal(t, float64(5), params["scoreTarget"])

	// formação agrupada e ordenada por teamId, players ordenados
	require.Len(t, s.Teams, 2)
	assert.Equal(t, teamDTO{TeamID: "team-1", Players: []string{"p1", "p2"}}, s.Teams[0])
	assert.Equal(t, teamDTO{TeamID: "team-2", Players: []string{"p3", "p4"}}, s.Teams[1])

	require.NotNil(t, s.Result)
	assert.Equal(t, domain.OutcomeWinLossDraw, s.Result.Kind)
	require.Len(t, s.Result.WinLossDraw, 2)
	assert.Equal(t, "win", s.Result.WinLossDraw[0].Outcome)
	assert.Equal(t, int64(4), s.Result.WinLossDraw[0].Stats["orbs"])
	assert.Empty(t, s.Result.Placement)
	assert.Nil(t, s.Result.Collective)
}

// Sessão sem snapshot/formação/resultado (legada, FFA, ttl): campos omitidos —
// o JSON antigo continua válido para consumidores existentes.
func TestAdminSessions_SemDetalhe_CamposOmitidos(t *testing.T) {
	dir := &fakeDirectory{
		closed: []domain.Session{{
			ID: "s0", MatchID: "m0", GameID: "g1", Players: []string{"p1"},
			State: domain.SessionClosed, OpenedAt: time.Now(), ClosedAt: time.Now(),
			CloseReason: domain.CloseTTL,
		}},
	}

	rec, _ := doGet(t, newAdminMux(dir), "/admin/games/g1/sessions")
	require.Equal(t, http.StatusOK, rec.Code)
	raw := rec.Body.String()
	assert.NotContains(t, raw, `"result"`)
	assert.NotContains(t, raw, `"teams"`)
	assert.NotContains(t, raw, `"modeParams"`)
	assert.NotContains(t, raw, `"archetype"`)
}

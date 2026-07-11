package consumer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/analytics-service/internal/domain"
)

// Envelope v2 (ADR-0023): o parse lê o campo outcome por jogador (win/loss/draw).
func TestParse_EnvelopeV2_LeOutcome(t *testing.T) {
	body := []byte(`{
		"schemaVersion": 2,
		"matchId": "m1",
		"gameId": "g1",
		"kind": "win_loss_draw",
		"results": [
			{"playerId": "p1", "outcome": "win",  "won": true,  "score": 10},
			{"playerId": "p2", "outcome": "draw", "won": false, "score": 5},
			{"playerId": "p3", "outcome": "loss", "won": false, "score": 2}
		]
	}`)

	m, err := parse(body)
	require.NoError(t, err)
	assert.Equal(t, "m1", m.MatchID)
	require.Len(t, m.Results, 3)
	assert.Equal(t, domain.OutcomeWin, m.Results[0].Outcome)
	assert.Equal(t, domain.OutcomeDraw, m.Results[1].Outcome, "empate preservado (não vira loss)")
	assert.Equal(t, domain.OutcomeLoss, m.Results[2].Outcome)
}

func TestPeekKind_Dispatch(t *testing.T) {
	cases := map[string]struct {
		body      string
		wantKind  string
		wantMatch string
	}{
		"participation":      {`{"kind":"participation_event","sessionId":"w1"}`, "participation_event", ""},
		"placement":          {`{"kind":"placement","matchId":"m1"}`, "placement", "m1"},
		"win_loss_draw":      {`{"kind":"win_loss_draw","matchId":"m1"}`, "win_loss_draw", "m1"},
		"v1 legado sem kind": {`{"matchId":"m1","results":[]}`, "", "m1"},
		"desconhecido":       {`{"kind":"summon_posted"}`, "summon_posted", ""},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			kind, matchID := peekKind([]byte(c.body))
			assert.Equal(t, c.wantKind, kind)
			assert.Equal(t, c.wantMatch, matchID)
		})
	}
}

func TestParseParticipation_LeCampos(t *testing.T) {
	body := []byte(`{"kind":"participation_event","gameId":"g1","sessionId":"w1","playerId":"inv-1","role":"invader","event":"HOST_KILLED","outcome":"SUCCESS"}`)
	evt, err := parseParticipation(body)
	require.NoError(t, err)
	assert.Equal(t, domain.ParticipationEvent{
		GameID: "g1", SessionID: "w1", PlayerID: "inv-1", Role: "invader", Event: "HOST_KILLED", Outcome: "SUCCESS",
	}, evt)
}

// modeId (ADR-0024, spec 009): o envelope já publica o campo — sem ele no
// JSON (envelope antigo), ModeID cai em "" (bucket "sem modo").
func TestParseParticipation_LeModeID(t *testing.T) {
	body := []byte(`{"kind":"participation_event","gameId":"g1","modeId":"batizado","sessionId":"w1","playerId":"inv-1","role":"invader","event":"HOST_KILLED","outcome":"SUCCESS"}`)
	evt, err := parseParticipation(body)
	require.NoError(t, err)
	assert.Equal(t, "batizado", evt.ModeID)
}

// O envelope placement (battle_royale) é lido em colocação + sobrevivência.
func TestParsePlacement_LeEntradas(t *testing.T) {
	body := []byte(`{"kind":"placement","matchId":"m1","gameId":"g1","entries":[
		{"playerId":"p1","teamId":"squad-1","placement":1,"survivedMs":900000,"stats":{"kills":4}},
		{"playerId":"p2","placement":42,"survivedMs":100000}
	]}`)
	pr, err := parsePlacement(body)
	require.NoError(t, err)
	assert.Equal(t, "m1", pr.MatchID)
	assert.Equal(t, "g1", pr.GameID)
	require.Len(t, pr.Entries, 2)
	assert.Equal(t, domain.PlacementEntry{Placement: 1, SurvivedMs: 900000}, pr.Entries[0])
	assert.Equal(t, domain.PlacementEntry{Placement: 42, SurvivedMs: 100000}, pr.Entries[1])
}

// O envelope collective (coop_mission) é lido em matchId/gameId/outcome (o
// analytics ignora players/stats — o desfecho é da sessão).
func TestParseCollective_LeDesfecho(t *testing.T) {
	body := []byte(`{"kind":"collective","matchId":"m1","gameId":"g1","outcome":"MISSION_SUCCESS","players":[{"playerId":"p1","stats":{"kills":9}}]}`)
	cr, err := parseCollective(body)
	require.NoError(t, err)
	assert.Equal(t, domain.CollectiveResult{MatchID: "m1", GameID: "g1", Outcome: "MISSION_SUCCESS"}, cr)
}

// Envelope v1 legado (sem outcome): deriva de won (win/loss), sem empate.
func TestParse_EnvelopeV1_FallbackPorWon(t *testing.T) {
	body := []byte(`{
		"matchId": "m1",
		"gameId": "g1",
		"results": [
			{"playerId": "p1", "won": true,  "score": 7},
			{"playerId": "p2", "won": false, "score": 1}
		]
	}`)

	m, err := parse(body)
	require.NoError(t, err)
	require.Len(t, m.Results, 2)
	assert.Equal(t, domain.OutcomeWin, m.Results[0].Outcome)
	assert.Equal(t, domain.OutcomeLoss, m.Results[1].Outcome)
}

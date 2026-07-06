package consumer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-game-backend/services/analytics-service/internal/domain"
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

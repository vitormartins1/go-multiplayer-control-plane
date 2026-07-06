package publisher

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-game-backend/services/session-service/internal/domain"
	"github.com/vitormartins1/go-game-backend/shared/logger"
)

type fakeSNS struct {
	in  *sns.PublishInput
	err error
}

func (f *fakeSNS) Publish(_ context.Context, in *sns.PublishInput, _ ...func(*sns.Options)) (*sns.PublishOutput, error) {
	f.in = in
	if f.err != nil {
		return nil, f.err
	}
	return &sns.PublishOutput{}, nil
}

func wld(players ...domain.PlayerOutcome) domain.MatchOutcome {
	return domain.MatchOutcome{Kind: domain.OutcomeWinLossDraw, WinLossDraw: &domain.WinLossDrawResult{Players: players}}
}

func TestSNSPublish_EnvelopeV2_RetrocompatEComOutcome(t *testing.T) {
	fake := &fakeSNS{}
	p := &SNS{client: fake, topicARN: "arn:aws:sns:local:match-results", log: logger.New("test")}

	outcome := wld(
		domain.PlayerOutcome{PlayerID: "p1", TeamID: "A", Outcome: domain.ResultWin, Score: 5, Stats: map[string]int64{"kills": 7}},
		domain.PlayerOutcome{PlayerID: "p2", TeamID: "B", Outcome: domain.ResultDraw, Score: 3},
	)
	require.NoError(t, p.Publish(context.Background(), "game-1", "match-1", outcome))

	require.NotNil(t, fake.in)
	assert.Equal(t, "arn:aws:sns:local:match-results", *fake.in.TopicArn)

	var got envelope
	require.NoError(t, json.Unmarshal([]byte(*fake.in.Message), &got))
	assert.Equal(t, 2, got.SchemaVersion)
	assert.Equal(t, envelopeVersion, got.SchemaVersion)
	assert.Equal(t, domain.OutcomeWinLossDraw, got.Kind)
	assert.Equal(t, "match-1", got.MatchID)
	assert.Equal(t, "game-1", got.GameID)
	require.Len(t, got.Results, 2)
	// campos novos + retrocompat (won derivado do outcome; score mantido).
	assert.Equal(t, playerResult{
		PlayerID: "p1", Won: true, Score: 5,
		Outcome: "win", TeamID: "A", Stats: map[string]int64{"kills": 7},
	}, got.Results[0])
	assert.False(t, got.Results[1].Won, "draw -> won=false (compat)")
	assert.Equal(t, "draw", got.Results[1].Outcome)
}

func TestSNSPublish_ErroDoSNS_Propaga(t *testing.T) {
	fake := &fakeSNS{err: errors.New("topic indisponível")}
	p := &SNS{client: fake, topicARN: "arn", log: logger.New("test")}

	err := p.Publish(context.Background(), "game-1", "match-1",
		wld(domain.PlayerOutcome{PlayerID: "p1", Outcome: domain.ResultWin}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sns publish")
}

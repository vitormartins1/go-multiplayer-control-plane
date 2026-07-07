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

func TestSNSPublishParticipation_EnvelopeDiscriminadoPorKind(t *testing.T) {
	fake := &fakeSNS{}
	p := &SNS{client: fake, topicARN: "arn:aws:sns:local:match-results", log: logger.New("test")}

	evt := domain.ParticipationEvent{PlayerID: "inv-1", Role: domain.RoleInvader, Event: domain.EventHostKilled, Outcome: domain.OutcomeSuccess}
	require.NoError(t, p.PublishParticipation(context.Background(), "game-1", "world-7", evt))

	require.NotNil(t, fake.in)
	assert.Equal(t, "arn:aws:sns:local:match-results", *fake.in.TopicArn, "mesmo tópico do win_loss_draw")

	var got participationEnvelope
	require.NoError(t, json.Unmarshal([]byte(*fake.in.Message), &got))
	assert.Equal(t, envelopeVersion, got.SchemaVersion)
	assert.Equal(t, "participation_event", got.Kind)
	assert.Equal(t, "game-1", got.GameID)
	assert.Equal(t, "world-7", got.SessionID)
	assert.Equal(t, "inv-1", got.PlayerID)
	assert.Equal(t, domain.RoleInvader, got.Role)
	assert.Equal(t, domain.EventHostKilled, got.Event)
	assert.Equal(t, domain.OutcomeSuccess, got.Outcome)

	// não carrega os campos do win_loss_draw.
	var raw map[string]any
	require.NoError(t, json.Unmarshal([]byte(*fake.in.Message), &raw))
	assert.NotContains(t, raw, "results")
	assert.NotContains(t, raw, "matchId")
}

func TestSNSPublishParticipation_ErroDoSNS_Propaga(t *testing.T) {
	fake := &fakeSNS{err: errors.New("topic indisponível")}
	p := &SNS{client: fake, topicARN: "arn", log: logger.New("test")}

	err := p.PublishParticipation(context.Background(), "game-1", "world-1",
		domain.ParticipationEvent{PlayerID: "p1", Role: domain.RoleCoop, Event: domain.EventLeft, Outcome: domain.OutcomeNeutral})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sns publish participation")
}

func TestSNSPublish_ErroDoSNS_Propaga(t *testing.T) {
	fake := &fakeSNS{err: errors.New("topic indisponível")}
	p := &SNS{client: fake, topicARN: "arn", log: logger.New("test")}

	err := p.Publish(context.Background(), "game-1", "match-1",
		wld(domain.PlayerOutcome{PlayerID: "p1", Outcome: domain.ResultWin}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sns publish")
}

// placementOutcome monta um MatchOutcome placement para os testes do publisher.
func placementOutcome(entries ...domain.PlacementEntry) domain.MatchOutcome {
	return domain.MatchOutcome{Kind: domain.OutcomePlacement, Placement: &domain.PlacementResult{Entries: entries}}
}

// BR2: o envelope placement é discriminado por kind e carrega entries[] (sem os
// campos results/won do win_loss_draw).
func TestSNSPublish_Placement_EnvelopeDiscriminadoPorKind(t *testing.T) {
	fake := &fakeSNS{}
	p := &SNS{client: fake, topicARN: "arn:aws:sns:local:match-results", log: logger.New("test")}

	outcome := placementOutcome(
		domain.PlacementEntry{PlayerID: "p1", TeamID: "squad-1", Placement: 1, SurvivedMs: 900000, Stats: map[string]int64{"kills": 4}},
		domain.PlacementEntry{PlayerID: "p3", Placement: 2, SurvivedMs: 500000},
	)
	require.NoError(t, p.Publish(context.Background(), "game-1", "match-1", outcome))

	require.NotNil(t, fake.in)
	assert.Equal(t, "arn:aws:sns:local:match-results", *fake.in.TopicArn, "mesmo tópico do win_loss_draw")

	var got placementEnvelope
	require.NoError(t, json.Unmarshal([]byte(*fake.in.Message), &got))
	assert.Equal(t, envelopeVersion, got.SchemaVersion)
	assert.Equal(t, kindPlacement, got.Kind)
	assert.Equal(t, "match-1", got.MatchID)
	assert.Equal(t, "game-1", got.GameID)
	require.Len(t, got.Entries, 2)
	assert.Equal(t, placementEntryDTO{
		PlayerID: "p1", TeamID: "squad-1", Placement: 1, SurvivedMs: 900000, Stats: map[string]int64{"kills": 4},
	}, got.Entries[0])
	assert.Equal(t, 2, got.Entries[1].Placement)
	assert.Empty(t, got.Entries[1].TeamID, "solo/FFA: teamId omitido")

	// não carrega os campos do win_loss_draw.
	var raw map[string]any
	require.NoError(t, json.Unmarshal([]byte(*fake.in.Message), &raw))
	assert.NotContains(t, raw, "results")
	assert.Contains(t, raw, "entries")
}

// win_loss_draw segue intacto após o dispatch por kind (sem regressão).
func TestSNSPublish_WinLossDraw_IntactoAposDispatch(t *testing.T) {
	fake := &fakeSNS{}
	p := &SNS{client: fake, topicARN: "arn", log: logger.New("test")}

	require.NoError(t, p.Publish(context.Background(), "game-1", "match-1",
		wld(domain.PlayerOutcome{PlayerID: "p1", Outcome: domain.ResultWin, Score: 5})))

	var got envelope
	require.NoError(t, json.Unmarshal([]byte(*fake.in.Message), &got))
	assert.Equal(t, domain.OutcomeWinLossDraw, got.Kind)
	require.Len(t, got.Results, 1)
	assert.True(t, got.Results[0].Won)
}

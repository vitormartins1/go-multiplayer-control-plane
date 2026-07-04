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

func TestSNSPublish_SerializaEnvelopeComMatchIDeResultados(t *testing.T) {
	fake := &fakeSNS{}
	p := &SNS{client: fake, topicARN: "arn:aws:sns:local:match-results", log: logger.New("test")}

	results := []domain.PlayerResult{
		{PlayerID: "p1", Score: 5, Won: true},
		{PlayerID: "p2", Score: 3, Won: false},
	}
	require.NoError(t, p.Publish(context.Background(), "game-1", "match-1", results))

	require.NotNil(t, fake.in)
	assert.Equal(t, "arn:aws:sns:local:match-results", *fake.in.TopicArn)

	var got envelope
	require.NoError(t, json.Unmarshal([]byte(*fake.in.Message), &got))
	assert.Equal(t, envelopeVersion, got.SchemaVersion)
	assert.Equal(t, "match-1", got.MatchID)
	assert.Equal(t, "game-1", got.GameID)
	require.Len(t, got.Results, 2)
	assert.Equal(t, playerResult{PlayerID: "p1", Won: true, Score: 5}, got.Results[0])
}

func TestSNSPublish_ErroDoSNS_Propaga(t *testing.T) {
	fake := &fakeSNS{err: errors.New("topic indisponível")}
	p := &SNS{client: fake, topicARN: "arn", log: logger.New("test")}

	err := p.Publish(context.Background(), "game-1", "match-1", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sns publish")
}

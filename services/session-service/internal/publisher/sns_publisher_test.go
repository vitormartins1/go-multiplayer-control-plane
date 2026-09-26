package publisher

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/domain"
	"github.com/vitormartins1/go-multiplayer-control-plane/shared/logger"
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
	require.NoError(t, p.Publish(context.Background(), "game-1", "match-1", "default", outcome))

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
	require.NoError(t, p.PublishParticipation(context.Background(), "game-1", "world-7", "default", evt))

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

func TestSNSPublishParticipation_EventIDPassaParaOEnvelope(t *testing.T) {
	fake := &fakeSNS{}
	p := &SNS{client: fake, topicARN: "arn:aws:sns:local:match-results", log: logger.New("test")}

	evt := domain.ParticipationEvent{
		PlayerID: "coop-1", Role: domain.RoleCoop, Event: domain.EventBossDefeated, Outcome: domain.OutcomeSuccess,
		EventID: "11111111-1111-1111-1111-111111111111",
	}
	require.NoError(t, p.PublishParticipation(context.Background(), "game-1", "world-7", "default", evt))

	var got participationEnvelope
	require.NoError(t, json.Unmarshal([]byte(*fake.in.Message), &got))
	assert.Equal(t, "11111111-1111-1111-1111-111111111111", got.EventID)
}

func TestSNSPublishParticipation_EventIDVazio_OmiteDoEnvelope(t *testing.T) {
	fake := &fakeSNS{}
	p := &SNS{client: fake, topicARN: "arn:aws:sns:local:match-results", log: logger.New("test")}

	evt := domain.ParticipationEvent{PlayerID: "coop-1", Role: domain.RoleCoop, Event: domain.EventBossDefeated, Outcome: domain.OutcomeSuccess}
	require.NoError(t, p.PublishParticipation(context.Background(), "game-1", "world-7", "default", evt))

	var raw map[string]any
	require.NoError(t, json.Unmarshal([]byte(*fake.in.Message), &raw))
	assert.NotContains(t, raw, "eventId", "omitempty: data plane antigo não manda o campo")
}

// TestSNSPublishParticipation_StatsPassamParaOEnvelope (spec 026): mesmo
// padrão dos outros 3 kinds — stats repassados sem validação de conteúdo.
func TestSNSPublishParticipation_StatsPassamParaOEnvelope(t *testing.T) {
	fake := &fakeSNS{}
	p := &SNS{client: fake, topicARN: "arn:aws:sns:local:match-results", log: logger.New("test")}

	evt := domain.ParticipationEvent{
		PlayerID: "coop-1", Role: domain.RoleCoop, Event: domain.EventBossDefeated, Outcome: domain.OutcomeSuccess,
		Stats: map[string]int64{"kills": 2, "revives": 1},
	}
	require.NoError(t, p.PublishParticipation(context.Background(), "game-1", "world-7", "default", evt))

	var got participationEnvelope
	require.NoError(t, json.Unmarshal([]byte(*fake.in.Message), &got))
	assert.Equal(t, map[string]int64{"kills": 2, "revives": 1}, got.Stats)
}

func TestSNSPublishParticipation_StatsVazio_OmiteDoEnvelope(t *testing.T) {
	fake := &fakeSNS{}
	p := &SNS{client: fake, topicARN: "arn:aws:sns:local:match-results", log: logger.New("test")}

	evt := domain.ParticipationEvent{PlayerID: "coop-1", Role: domain.RoleCoop, Event: domain.EventBossDefeated, Outcome: domain.OutcomeSuccess}
	require.NoError(t, p.PublishParticipation(context.Background(), "game-1", "world-7", "default", evt))

	var raw map[string]any
	require.NoError(t, json.Unmarshal([]byte(*fake.in.Message), &raw))
	assert.NotContains(t, raw, "stats", "omitempty: data plane antigo não manda o campo")
}

func TestSNSPublishParticipation_ErroDoSNS_Propaga(t *testing.T) {
	fake := &fakeSNS{err: errors.New("topic indisponível")}
	p := &SNS{client: fake, topicARN: "arn", log: logger.New("test")}

	err := p.PublishParticipation(context.Background(), "game-1", "world-1", "default",
		domain.ParticipationEvent{PlayerID: "p1", Role: domain.RoleCoop, Event: domain.EventLeft, Outcome: domain.OutcomeNeutral})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sns publish participation")
}

func TestSNSPublish_ErroDoSNS_Propaga(t *testing.T) {
	fake := &fakeSNS{err: errors.New("topic indisponível")}
	p := &SNS{client: fake, topicARN: "arn", log: logger.New("test")}

	err := p.Publish(context.Background(), "game-1", "match-1", "default",
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
	require.NoError(t, p.Publish(context.Background(), "game-1", "match-1", "default", outcome))

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

// C2: o envelope collective é discriminado por kind e carrega o outcome da
// SESSÃO + players[] (sem os campos results/won/entries dos outros kinds).
func TestSNSPublish_Collective_EnvelopeDiscriminadoPorKind(t *testing.T) {
	fake := &fakeSNS{}
	p := &SNS{client: fake, topicARN: "arn:aws:sns:local:match-results", log: logger.New("test")}

	outcome := domain.MatchOutcome{Kind: domain.OutcomeCollective, Collective: &domain.CollectiveResult{
		Outcome: domain.MissionSuccess,
		Players: []domain.CollectivePlayer{
			{PlayerID: "p1", TeamID: "team-1", Stats: map[string]int64{"kills": 12}},
			{PlayerID: "p2", TeamID: "team-1"},
		},
	}}
	require.NoError(t, p.Publish(context.Background(), "game-1", "match-1", "default", outcome))

	require.NotNil(t, fake.in)
	assert.Equal(t, "arn:aws:sns:local:match-results", *fake.in.TopicArn, "mesmo tópico dos demais kinds")

	var got collectiveEnvelope
	require.NoError(t, json.Unmarshal([]byte(*fake.in.Message), &got))
	assert.Equal(t, envelopeVersion, got.SchemaVersion)
	assert.Equal(t, kindCollective, got.Kind)
	assert.Equal(t, "match-1", got.MatchID)
	assert.Equal(t, "game-1", got.GameID)
	assert.Equal(t, domain.MissionSuccess, got.Outcome, "desfecho da sessão")
	require.Len(t, got.Players, 2)
	assert.Equal(t, collectivePlayerDTO{
		PlayerID: "p1", TeamID: "team-1", Stats: map[string]int64{"kills": 12},
	}, got.Players[0])
	assert.Empty(t, got.Players[1].Stats, "stats vazio omitido")

	// não carrega os campos dos outros kinds.
	var raw map[string]any
	require.NoError(t, json.Unmarshal([]byte(*fake.in.Message), &raw))
	assert.NotContains(t, raw, "results")
	assert.NotContains(t, raw, "entries")
	assert.Contains(t, raw, "players")
	assert.Contains(t, raw, "outcome")
}

// TestSNSPublishSessionClosed_EnvelopeCorreto (spec 028): kind, sessionId,
// archetype, closeReason e timestamps corretos.
func TestSNSPublishSessionClosed_EnvelopeCorreto(t *testing.T) {
	fake := &fakeSNS{}
	p := &SNS{client: fake, topicARN: "arn:aws:sns:local:match-results", log: logger.New("test")}

	opened := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	closed := opened.Add(6 * time.Minute)
	sess := &domain.Session{
		ID: "s1", GameID: "game-1", ModeID: "mundo-vivo",
		Profile:     &domain.ProfileSnapshot{Archetype: domain.ArchetypeHostWorldDropIn},
		CloseReason: domain.CloseHostLeft,
		OpenedAt:    opened, ClosedAt: closed,
	}
	require.NoError(t, p.PublishSessionClosed(context.Background(), sess))

	require.NotNil(t, fake.in)
	assert.Equal(t, "arn:aws:sns:local:match-results", *fake.in.TopicArn, "mesmo tópico dos demais kinds")

	var got sessionClosedEnvelope
	require.NoError(t, json.Unmarshal([]byte(*fake.in.Message), &got))
	assert.Equal(t, envelopeVersion, got.SchemaVersion)
	assert.Equal(t, kindSessionClosed, got.Kind)
	assert.Equal(t, "s1", got.SessionID)
	assert.Equal(t, "game-1", got.GameID)
	assert.Equal(t, "mundo-vivo", got.ModeID)
	assert.Equal(t, domain.ArchetypeHostWorldDropIn, got.Archetype)
	assert.Equal(t, "host_left", got.CloseReason)
	assert.Equal(t, opened.Format(time.RFC3339), got.OpenedAt)
	assert.Equal(t, closed.Format(time.RFC3339), got.ClosedAt)
}

func TestSNSPublishSessionClosed_SemProfile_ArchetypeVazio(t *testing.T) {
	fake := &fakeSNS{}
	p := &SNS{client: fake, topicARN: "arn", log: logger.New("test")}

	sess := &domain.Session{ID: "s1", GameID: "game-1", CloseReason: domain.CloseTTL}
	require.NoError(t, p.PublishSessionClosed(context.Background(), sess))

	var got sessionClosedEnvelope
	require.NoError(t, json.Unmarshal([]byte(*fake.in.Message), &got))
	assert.Empty(t, got.Archetype)
	assert.Equal(t, "ttl", got.CloseReason)
}

// win_loss_draw segue intacto após o dispatch por kind (sem regressão).
func TestSNSPublish_WinLossDraw_IntactoAposDispatch(t *testing.T) {
	fake := &fakeSNS{}
	p := &SNS{client: fake, topicARN: "arn", log: logger.New("test")}

	require.NoError(t, p.Publish(context.Background(), "game-1", "match-1", "default",
		wld(domain.PlayerOutcome{PlayerID: "p1", Outcome: domain.ResultWin, Score: 5})))

	var got envelope
	require.NoError(t, json.Unmarshal([]byte(*fake.in.Message), &got))
	assert.Equal(t, domain.OutcomeWinLossDraw, got.Kind)
	require.Len(t, got.Results, 1)
	assert.True(t, got.Results[0].Won)
}

// blockingSNS simula o SNS travado: só responde quando o ctx da chamada acaba.
type blockingSNS struct{}

func (blockingSNS) Publish(ctx context.Context, _ *sns.PublishInput, _ ...func(*sns.Options)) (*sns.PublishOutput, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

// Com o SNS travado e um ctx sem deadline (Dedicated Server sem timeout), o
// publish devolve erro no timeout próprio em vez de pendurar — o server recebe
// o erro e reenvia (spec 029 / contrato host_world_dropin).
func TestSNSPublishParticipation_SNSTravado_ErroNoTimeout(t *testing.T) {
	p := &SNS{client: blockingSNS{}, topicARN: "arn", log: logger.New("test"), timeout: 50 * time.Millisecond}

	start := time.Now()
	err := p.PublishParticipation(context.Background(), "game-1", "world-1", "default",
		domain.ParticipationEvent{PlayerID: "p1", Role: domain.RoleInvader, Event: domain.EventHostKilled, Outcome: domain.OutcomeSuccess})

	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), 2*time.Second)
}

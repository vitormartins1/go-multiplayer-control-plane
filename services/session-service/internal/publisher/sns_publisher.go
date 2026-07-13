package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/awsx"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/domain"
)

// publishTotal conta as publicações no tópico por resultado (ADR-0020 / RED).
var publishTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "fanout_publish_total",
	Help: "Resultados publicados no tópico match-results por status (ok/error).",
}, []string{"result"})

var (
	_ domain.ResultPublisher        = (*SNS)(nil)
	_ domain.ParticipationPublisher = (*SNS)(nil)
)

// kindParticipationEvent é o discriminador do envelope de participação (spec 013,
// decisão #3): mesmo tópico match-results, kind distinto do win_loss_draw.
const kindParticipationEvent = "participation_event"

// kindPlacement é o discriminador do envelope de colocação (battle_royale, BR2):
// mesmo tópico match-results, kind distinto do win_loss_draw.
const kindPlacement = "placement"

// kindCollective é o discriminador do envelope de resultado coletivo
// (coop_mission, C2): mesmo tópico match-results, kind distinto dos demais.
const kindCollective = "collective"

// envelopeVersion é a versão do schema da mensagem publicada no tópico. Os
// consumidores ignoram campos desconhecidos; bumps de major viram um novo campo.
//
// v2 (ADR-0023): outcome polimórfico. Ganha `kind` e, por jogador,
// `outcome`/`teamId`/`stats`, MANTENDO `won`/`score` para os consumidores atuais
// (retrocompat): won = (outcome == "win").
const envelopeVersion = 2

// envelope é o contrato JSON publicado no tópico match-results (resultado bruto
// da partida). Cada consumidor (achievements/leaderboard) desserializa só o que
// usa — os tipos não são compartilhados entre serviços (mensageria desacoplada).
type envelope struct {
	SchemaVersion int            `json:"schemaVersion"`
	MatchID       string         `json:"matchId"`
	GameID        string         `json:"gameId"`
	ModeID        string         `json:"modeId,omitempty"` // modo do jogo (ADR-0024, aditivo)
	Kind          string         `json:"kind"`             // "win_loss_draw" (ADR-0023)
	Results       []playerResult `json:"results"`
}

type playerResult struct {
	PlayerID string           `json:"playerId"`
	Won      bool             `json:"won"`   // compat: outcome == "win"
	Score    int64            `json:"score"` // compat
	Outcome  string           `json:"outcome"`
	TeamID   string           `json:"teamId,omitempty"`
	Stats    map[string]int64 `json:"stats,omitempty"`
}

// participationEnvelope é o envelope do host_world_dropin (spec 013) no mesmo
// tópico match-results. Discriminado por kind=participation_event; carrega o
// evento por participante (sem matchId/results, que são do win_loss_draw).
type participationEnvelope struct {
	SchemaVersion int    `json:"schemaVersion"`
	Kind          string `json:"kind"`
	GameID        string `json:"gameId"`
	ModeID        string `json:"modeId,omitempty"` // ADR-0024, aditivo
	SessionID     string `json:"sessionId"`
	PlayerID      string `json:"playerId"`
	Role          string `json:"role"`
	Event         string `json:"event"`
	Outcome       string `json:"outcome"`
	// EventID identifica a ocorrência (follow-up #4, spec 021); vazio = data
	// plane antigo, os sinks caem no fallback deles.
	EventID string `json:"eventId,omitempty"`
	// Stats são métricas opcionais do participante (spec 026) — mesmo padrão
	// dos outros 3 kinds. Ausente = data plane antigo (compat total).
	Stats map[string]int64 `json:"stats,omitempty"`
}

// placementEnvelope é o envelope do battle_royale (BR2) no mesmo tópico
// match-results. Discriminado por kind=placement; carrega uma colocação ordinal
// por participante (sem results/won, que são do win_loss_draw).
type placementEnvelope struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Kind          string              `json:"kind"`
	MatchID       string              `json:"matchId"`
	GameID        string              `json:"gameId"`
	ModeID        string              `json:"modeId,omitempty"` // ADR-0024, aditivo
	Entries       []placementEntryDTO `json:"entries"`
}

type placementEntryDTO struct {
	PlayerID   string           `json:"playerId"`
	TeamID     string           `json:"teamId,omitempty"`
	Placement  int              `json:"placement"`
	SurvivedMs int64            `json:"survivedMs"`
	Stats      map[string]int64 `json:"stats,omitempty"`
}

// collectiveEnvelope é o envelope do coop_mission (C2) no mesmo tópico
// match-results. Discriminado por kind=collective; o `outcome` é da SESSÃO
// inteira (não por jogador) e cada `players[]` carrega só o time e os stats.
type collectiveEnvelope struct {
	SchemaVersion int                   `json:"schemaVersion"`
	Kind          string                `json:"kind"`
	MatchID       string                `json:"matchId"`
	GameID        string                `json:"gameId"`
	ModeID        string                `json:"modeId,omitempty"` // ADR-0024, aditivo
	Outcome       string                `json:"outcome"`          // MISSION_SUCCESS | MISSION_FAILURE (da sessão)
	Players       []collectivePlayerDTO `json:"players"`
}

type collectivePlayerDTO struct {
	PlayerID string           `json:"playerId"`
	TeamID   string           `json:"teamId,omitempty"`
	Stats    map[string]int64 `json:"stats,omitempty"`
}

// snsClient é a fração da API do SNS que usamos (permite fake em teste).
type snsClient interface {
	Publish(ctx context.Context, in *sns.PublishInput, optFns ...func(*sns.Options)) (*sns.PublishOutput, error)
}

// SNS publica o resultado da partida no tópico match-results (ADR-0020). Falha
// de publicação propaga: o session não encerra a sessão e o Dedicated Server
// refaz o ReportResult (retry) — o resultado não se perde silenciosamente.
type SNS struct {
	client   snsClient
	topicARN string
	log      *slog.Logger
}

// NewSNS monta o publisher a partir do ambiente (AWS_ENDPOINT_URL → LocalStack).
func NewSNS(ctx context.Context, topicARN string, log *slog.Logger) (*SNS, error) {
	cfg, err := awsx.Config(ctx)
	if err != nil {
		return nil, fmt.Errorf("session publisher: aws config: %w", err)
	}
	client := sns.NewFromConfig(cfg, func(o *sns.Options) {
		o.BaseEndpoint = awsx.BaseEndpoint(o.BaseEndpoint)
	})
	return &SNS{client: client, topicARN: topicARN, log: log}, nil
}

// Publish serializa o envelope do kind ativo e publica no tópico (ADR-0023:
// despacha por outcome.Kind — win_loss_draw ou placement). Retorna erro em falha.
func (s *SNS) Publish(ctx context.Context, gameID, matchID, modeID string, outcome domain.MatchOutcome) error {
	switch outcome.Kind {
	case domain.OutcomePlacement:
		return s.publishPlacement(ctx, gameID, matchID, modeID, outcome.Placement)
	case domain.OutcomeCollective:
		return s.publishCollective(ctx, gameID, matchID, modeID, outcome.Collective)
	default:
		return s.publishWinLossDraw(ctx, gameID, matchID, modeID, outcome)
	}
}

// publishWinLossDraw publica o envelope do queue_arena (kind=win_loss_draw).
func (s *SNS) publishWinLossDraw(ctx context.Context, gameID, matchID, modeID string, outcome domain.MatchOutcome) error {
	players := outcome.WinLossDraw.Players
	env := envelope{
		SchemaVersion: envelopeVersion,
		MatchID:       matchID,
		GameID:        gameID,
		ModeID:        modeID,
		Kind:          outcome.Kind,
		Results:       make([]playerResult, 0, len(players)),
	}
	for _, p := range players {
		env.Results = append(env.Results, playerResult{
			PlayerID: p.PlayerID,
			Won:      p.Outcome == domain.ResultWin, // compat
			Score:    p.Score,
			Outcome:  p.Outcome,
			TeamID:   p.TeamID,
			Stats:    p.Stats,
		})
	}

	body, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("session publisher: marshal envelope: %w", err)
	}

	if err := s.publish(ctx, body); err != nil {
		return err
	}
	s.log.Info("result publicado no tópico",
		slog.String("game_id", gameID),
		slog.String("match_id", matchID),
		slog.Int("players", len(players)),
	)
	return nil
}

// publishPlacement publica o envelope do battle_royale (kind=placement, BR2).
func (s *SNS) publishPlacement(ctx context.Context, gameID, matchID, modeID string, result *domain.PlacementResult) error {
	env := placementEnvelope{
		SchemaVersion: envelopeVersion,
		Kind:          kindPlacement,
		MatchID:       matchID,
		GameID:        gameID,
		ModeID:        modeID,
		Entries:       make([]placementEntryDTO, 0, len(result.Entries)),
	}
	for _, e := range result.Entries {
		env.Entries = append(env.Entries, placementEntryDTO{
			PlayerID:   e.PlayerID,
			TeamID:     e.TeamID,
			Placement:  e.Placement,
			SurvivedMs: e.SurvivedMs,
			Stats:      e.Stats,
		})
	}

	body, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("session publisher: marshal placement envelope: %w", err)
	}

	if err := s.publish(ctx, body); err != nil {
		return err
	}
	s.log.Info("placement publicado no tópico",
		slog.String("game_id", gameID),
		slog.String("match_id", matchID),
		slog.Int("entries", len(result.Entries)),
	)
	return nil
}

// publishCollective publica o envelope do coop_mission (kind=collective, C2).
func (s *SNS) publishCollective(ctx context.Context, gameID, matchID, modeID string, result *domain.CollectiveResult) error {
	env := collectiveEnvelope{
		SchemaVersion: envelopeVersion,
		Kind:          kindCollective,
		MatchID:       matchID,
		GameID:        gameID,
		ModeID:        modeID,
		Outcome:       result.Outcome,
		Players:       make([]collectivePlayerDTO, 0, len(result.Players)),
	}
	for _, p := range result.Players {
		env.Players = append(env.Players, collectivePlayerDTO{
			PlayerID: p.PlayerID,
			TeamID:   p.TeamID,
			Stats:    p.Stats,
		})
	}

	body, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("session publisher: marshal collective envelope: %w", err)
	}

	if err := s.publish(ctx, body); err != nil {
		return err
	}
	s.log.Info("collective publicado no tópico",
		slog.String("game_id", gameID),
		slog.String("match_id", matchID),
		slog.String("outcome", result.Outcome),
		slog.Int("players", len(result.Players)),
	)
	return nil
}

// publish envia o corpo já serializado ao tópico e contabiliza o resultado
// (fanout_publish_total). Reuso entre os kinds.
func (s *SNS) publish(ctx context.Context, body []byte) error {
	if _, err := s.client.Publish(ctx, &sns.PublishInput{
		TopicArn: aws.String(s.topicARN),
		Message:  aws.String(string(body)),
	}); err != nil {
		publishTotal.WithLabelValues("error").Inc()
		return fmt.Errorf("session publisher: sns publish: %w", err)
	}
	publishTotal.WithLabelValues("ok").Inc()
	return nil
}

// PublishParticipation serializa o envelope participation_event e o publica no
// mesmo tópico match-results (spec 013). Falha propaga (o server refaz o report).
func (s *SNS) PublishParticipation(ctx context.Context, gameID, sessionID, modeID string, evt domain.ParticipationEvent) error {
	env := participationEnvelope{
		SchemaVersion: envelopeVersion,
		Kind:          kindParticipationEvent,
		GameID:        gameID,
		ModeID:        modeID,
		SessionID:     sessionID,
		PlayerID:      evt.PlayerID,
		Role:          evt.Role,
		Event:         evt.Event,
		Outcome:       evt.Outcome,
		EventID:       evt.EventID,
		Stats:         evt.Stats,
	}
	body, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("session publisher: marshal participation envelope: %w", err)
	}
	if _, err := s.client.Publish(ctx, &sns.PublishInput{
		TopicArn: aws.String(s.topicARN),
		Message:  aws.String(string(body)),
	}); err != nil {
		publishTotal.WithLabelValues("error").Inc()
		return fmt.Errorf("session publisher: sns publish participation: %w", err)
	}
	publishTotal.WithLabelValues("ok").Inc()
	s.log.Info("participation event publicado no tópico",
		slog.String("game_id", gameID),
		slog.String("session_id", sessionID),
		slog.String("player_id", evt.PlayerID),
		slog.String("role", evt.Role),
		slog.String("event", evt.Event),
		slog.String("outcome", evt.Outcome),
	)
	return nil
}

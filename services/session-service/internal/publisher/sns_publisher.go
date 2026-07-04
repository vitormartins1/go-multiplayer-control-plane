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

	"github.com/vitormartins1/go-game-backend/services/session-service/internal/awsx"
	"github.com/vitormartins1/go-game-backend/services/session-service/internal/domain"
)

// publishTotal conta as publicações no tópico por resultado (ADR-0020 / RED).
var publishTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "fanout_publish_total",
	Help: "Resultados publicados no tópico match-results por status (ok/error).",
}, []string{"result"})

var _ domain.ResultPublisher = (*SNS)(nil)

// envelopeVersion é a versão do schema da mensagem publicada no tópico. Os
// consumidores ignoram campos desconhecidos; bumps de major viram um novo campo.
const envelopeVersion = 1

// envelope é o contrato JSON publicado no tópico match-results (resultado bruto
// da partida). Cada consumidor (achievements/leaderboard) desserializa só o que
// usa — os tipos não são compartilhados entre serviços (mensageria desacoplada).
type envelope struct {
	SchemaVersion int            `json:"schemaVersion"`
	MatchID       string         `json:"matchId"`
	GameID        string         `json:"gameId"`
	Results       []playerResult `json:"results"`
}

type playerResult struct {
	PlayerID string `json:"playerId"`
	Won      bool   `json:"won"`
	Score    int64  `json:"score"`
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

// Publish serializa o envelope e publica no tópico. Retorna erro em falha.
func (s *SNS) Publish(ctx context.Context, gameID, matchID string, results []domain.PlayerResult) error {
	env := envelope{
		SchemaVersion: envelopeVersion,
		MatchID:       matchID,
		GameID:        gameID,
		Results:       make([]playerResult, 0, len(results)),
	}
	for _, r := range results {
		env.Results = append(env.Results, playerResult{PlayerID: r.PlayerID, Won: r.Won, Score: r.Score})
	}

	body, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("session publisher: marshal envelope: %w", err)
	}

	if _, err := s.client.Publish(ctx, &sns.PublishInput{
		TopicArn: aws.String(s.topicARN),
		Message:  aws.String(string(body)),
	}); err != nil {
		publishTotal.WithLabelValues("error").Inc()
		return fmt.Errorf("session publisher: sns publish: %w", err)
	}
	publishTotal.WithLabelValues("ok").Inc()
	s.log.Info("result publicado no tópico",
		slog.String("game_id", gameID),
		slog.String("match_id", matchID),
		slog.Int("players", len(results)),
	)
	return nil
}

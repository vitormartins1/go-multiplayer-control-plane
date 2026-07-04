// Package consumer é o driving adapter do analytics: faz long-poll da fila SQS
// match-results-analytics, desserializa o envelope do fan-out (ADR-0020/0021) e
// entrega cada partida ao worker pool (Ingestor). Deleta a mensagem só quando o
// pool confirma sucesso (at-least-once → DLQ em falha). Réplica do padrão da 006,
// adaptada ao worker pool.
package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/vitormartins1/go-game-backend/services/analytics-service/internal/domain"
)

// sqsAPI é a fração da API do SQS que usamos (permite fake em teste).
type sqsAPI interface {
	ReceiveMessage(ctx context.Context, in *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessage(ctx context.Context, in *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
}

// Consumer lê a fila e alimenta o worker pool.
type Consumer struct {
	client   sqsAPI
	queueURL string
	pool     domain.Ingestor
	log      *slog.Logger
}

// New cria o consumidor.
func New(client sqsAPI, queueURL string, pool domain.Ingestor, log *slog.Logger) *Consumer {
	return &Consumer{client: client, queueURL: queueURL, pool: pool, log: log}
}

// Run bloqueia consumindo até o ctx encerrar. Erros de receive não derrubam o
// loop: loga e tenta de novo após um backoff curto.
func (c *Consumer) Run(ctx context.Context) {
	c.log.Info("consumer iniciando", slog.String("queue", c.queueURL))
	for {
		if ctx.Err() != nil {
			return
		}
		out, err := c.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(c.queueURL),
			MaxNumberOfMessages: 10,
			WaitTimeSeconds:     20,
		})
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			c.log.Error("receive falhou", slog.String("err", err.Error()))
			c.sleep(ctx, time.Second)
			continue
		}
		c.handleBatch(ctx, out.Messages)
	}
}

// handleBatch faz fan-out do batch para o pool e fan-in dos resultados: submete
// todas as mensagens, depois espera cada uma e deleta as que agregaram com sucesso.
func (c *Consumer) handleBatch(ctx context.Context, msgs []types.Message) {
	type pending struct {
		msg  types.Message
		done <-chan error
	}
	var inflight []pending
	for i := range msgs {
		m := msgs[i]
		match, err := parse([]byte(aws.ToString(m.Body)))
		if err != nil {
			// mensagem malformada: não deleta (vai à DLQ), não trava as outras.
			c.log.Error("envelope inválido (vai à DLQ)", slog.String("err", err.Error()))
			continue
		}
		inflight = append(inflight, pending{msg: m, done: c.pool.Submit(ctx, match)})
	}
	for _, p := range inflight {
		if err := <-p.done; err != nil {
			c.log.Error("agregação falhou (mensagem volta à fila)", slog.String("err", err.Error()))
			continue
		}
		if _, err := c.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
			QueueUrl:      aws.String(c.queueURL),
			ReceiptHandle: p.msg.ReceiptHandle,
		}); err != nil {
			c.log.Error("delete falhou (idempotência cobre a reentrega)", slog.String("err", err.Error()))
		}
	}
}

// parse desserializa o envelope do fan-out (struct próprio, desacoplado).
func parse(body []byte) (domain.MatchResult, error) {
	var env struct {
		MatchID string `json:"matchId"`
		GameID  string `json:"gameId"`
		Results []struct {
			PlayerID string `json:"playerId"`
			Won      bool   `json:"won"`
			Score    int64  `json:"score"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return domain.MatchResult{}, fmt.Errorf("unmarshal envelope: %w", err)
	}
	m := domain.MatchResult{MatchID: env.MatchID, GameID: env.GameID}
	for _, r := range env.Results {
		m.Results = append(m.Results, domain.PlayerResult{PlayerID: r.PlayerID, Won: r.Won, Score: r.Score})
	}
	return m, nil
}

func (c *Consumer) sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

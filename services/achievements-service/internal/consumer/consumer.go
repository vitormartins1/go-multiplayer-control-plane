// Package consumer roda o loop de consumo de uma fila SQS do fan-out durável
// (ADR-0020): long-poll → handler → delete-após-sucesso. Em erro, a mensagem
// NÃO é deletada: volta à fila após o visibility timeout e, esgotado o
// maxReceiveCount, vai para a DLQ (redrive configurado na infra). Réplica local
// por serviço (não toca shared/, ver plan da spec 006).
package consumer

import (
	"context"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/prometheus/client_golang/prometheus"
)

// sqsMessage é uma mensagem recebida (alias do tipo do SDK).
type sqsMessage = types.Message

// Handler processa o corpo (raw) de uma mensagem. Retornar erro NÃO deleta a
// mensagem (segue para retry/DLQ). dup=true sinaliza reprocessamento já visto
// (no-op idempotente) — a mensagem é deletada normalmente. Deve ser idempotente:
// SQS é at-least-once.
type Handler func(ctx context.Context, body []byte) (dup bool, err error)

// sqsAPI é a fração da API do SQS que usamos (permite fake em teste).
type sqsAPI interface {
	ReceiveMessage(ctx context.Context, in *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessage(ctx context.Context, in *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
}

// Consumer faz long-poll numa fila e entrega cada mensagem ao handler.
type Consumer struct {
	client   sqsAPI
	queueURL string
	handle   Handler
	log      *slog.Logger
	metrics  *Metrics
}

// New cria o consumer. metrics pode ser nil (sem instrumentação).
func New(client sqsAPI, queueURL string, handle Handler, log *slog.Logger, metrics *Metrics) *Consumer {
	return &Consumer{client: client, queueURL: queueURL, handle: handle, log: log, metrics: metrics}
}

// Run bloqueia consumindo até o ctx encerrar (shutdown limpo). Erros de receive
// não derrubam o loop: loga e tenta de novo após um backoff curto.
func (c *Consumer) Run(ctx context.Context) {
	c.log.Info("consumer iniciando", slog.String("queue", c.queueURL))
	for {
		if ctx.Err() != nil {
			return
		}
		out, err := c.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(c.queueURL),
			MaxNumberOfMessages: 10,
			WaitTimeSeconds:     20, // long-poll: reduz chamadas vazias
		})
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			c.log.Error("receive falhou", slog.String("err", err.Error()))
			c.sleep(ctx, time.Second)
			continue
		}
		for i := range out.Messages {
			c.process(ctx, out.Messages[i])
		}
	}
}

func (c *Consumer) process(ctx context.Context, m sqsMessage) {
	start := time.Now()
	dup, err := c.handle(ctx, []byte(aws.ToString(m.Body)))
	c.observe(dup, err, time.Since(start))
	if err != nil {
		// não deleta: volta à fila (retry) e, esgotado o maxReceive, vai à DLQ.
		c.log.Error("handler falhou (mensagem volta à fila)", slog.String("err", err.Error()))
		return
	}
	if _, derr := c.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(c.queueURL),
		ReceiptHandle: m.ReceiptHandle,
	}); derr != nil {
		c.log.Error("delete falhou (mensagem pode reaparecer; idempotência cobre)", slog.String("err", derr.Error()))
	}
}

func (c *Consumer) observe(dup bool, err error, d time.Duration) {
	if c.metrics == nil {
		return
	}
	result := "ok"
	switch {
	case err != nil:
		result = "error"
	case dup:
		result = "dup"
	}
	c.metrics.processed.WithLabelValues(result).Inc()
	c.metrics.duration.Observe(d.Seconds())
}

// Metrics são as métricas RED do consumo (ADR-0020 / OBSERVABILITY).
type Metrics struct {
	processed *prometheus.CounterVec
	duration  prometheus.Histogram
}

// NewMetrics registra fanout_consume_total{result} e a duração do processamento.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		processed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "fanout_consume_total",
			Help: "Mensagens do fan-out consumidas por resultado (ok/dup/error).",
		}, []string{"result"}),
		duration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "fanout_consume_duration_seconds",
			Help:    "Latência do processamento de cada mensagem do fan-out.",
			Buckets: prometheus.DefBuckets,
		}),
	}
	reg.MustRegister(m.processed, m.duration)
	return m
}

func (c *Consumer) sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

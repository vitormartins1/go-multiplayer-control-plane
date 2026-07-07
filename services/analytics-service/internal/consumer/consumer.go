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
		body := []byte(aws.ToString(m.Body))
		kind, matchID := peekKind(body)
		switch {
		case kind == "participation_event":
			evt, err := parseParticipation(body)
			if err != nil {
				c.log.Error("participation inválido (vai à DLQ)", slog.String("err", err.Error()))
				continue
			}
			inflight = append(inflight, pending{msg: m, done: c.pool.SubmitParticipation(ctx, evt)})
		case kind == "placement":
			// battle_royale (session/015): tem matchId, então precede o ramo abaixo.
			pr, err := parsePlacement(body)
			if err != nil {
				c.log.Error("placement inválido (vai à DLQ)", slog.String("err", err.Error()))
				continue
			}
			inflight = append(inflight, pending{msg: m, done: c.pool.SubmitPlacement(ctx, pr)})
		case kind == "collective":
			// coop_mission (session/016): desfecho coletivo; tem matchId.
			cr, err := parseCollective(body)
			if err != nil {
				c.log.Error("collective inválido (vai à DLQ)", slog.String("err", err.Error()))
				continue
			}
			inflight = append(inflight, pending{msg: m, done: c.pool.SubmitCollective(ctx, cr)})
		case kind == "win_loss_draw" || matchID != "":
			match, err := parse(body)
			if err != nil {
				// malformado: não deleta (vai à DLQ), não trava as outras.
				c.log.Error("envelope inválido (vai à DLQ)", slog.String("err", err.Error()))
				continue
			}
			inflight = append(inflight, pending{msg: m, done: c.pool.Submit(ctx, match)})
		default:
			// kind desconhecido/ausente sem matchId: não é nosso -> descarta (deleta).
			c.log.Debug("envelope ignorado (kind não tratado)", slog.String("kind", kind))
			c.deleteMessage(ctx, m)
		}
	}
	for _, p := range inflight {
		if err := <-p.done; err != nil {
			c.log.Error("agregação falhou (mensagem volta à fila)", slog.String("err", err.Error()))
			continue
		}
		c.deleteMessage(ctx, p.msg)
	}
}

// deleteMessage remove a mensagem da fila (sucesso ou descarte). Falha de delete
// não é fatal: a idempotência (processed_matches/participations) cobre a reentrega.
func (c *Consumer) deleteMessage(ctx context.Context, m types.Message) {
	if _, err := c.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(c.queueURL),
		ReceiptHandle: m.ReceiptHandle,
	}); err != nil {
		c.log.Error("delete falhou (idempotência cobre a reentrega)", slog.String("err", err.Error()))
	}
}

// peekKind espia o kind e o matchId do envelope sem desserializar o corpo todo,
// para o dispatch (ADR-0023, spec 004): win_loss_draw / participation_event.
func peekKind(body []byte) (kind, matchID string) {
	var head struct {
		Kind    string `json:"kind"`
		MatchID string `json:"matchId"`
	}
	_ = json.Unmarshal(body, &head) // corpo malformado cai no parse específico (DLQ)
	return head.Kind, head.MatchID
}

// parseParticipation desserializa o envelope participation_event (host_world_dropin).
func parseParticipation(body []byte) (domain.ParticipationEvent, error) {
	var env struct {
		GameID    string `json:"gameId"`
		SessionID string `json:"sessionId"`
		PlayerID  string `json:"playerId"`
		Role      string `json:"role"`
		Event     string `json:"event"`
		Outcome   string `json:"outcome"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return domain.ParticipationEvent{}, fmt.Errorf("unmarshal participation: %w", err)
	}
	return domain.ParticipationEvent{
		GameID: env.GameID, SessionID: env.SessionID, PlayerID: env.PlayerID,
		Role: env.Role, Event: env.Event, Outcome: env.Outcome,
	}, nil
}

// parsePlacement desserializa o envelope placement (battle_royale, session/015).
// O analytics só agrega colocação + sobrevivência (ignora playerId/teamId/stats).
func parsePlacement(body []byte) (domain.PlacementResult, error) {
	var env struct {
		MatchID string `json:"matchId"`
		GameID  string `json:"gameId"`
		Entries []struct {
			Placement  int   `json:"placement"`
			SurvivedMs int64 `json:"survivedMs"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return domain.PlacementResult{}, fmt.Errorf("unmarshal placement: %w", err)
	}
	pr := domain.PlacementResult{MatchID: env.MatchID, GameID: env.GameID}
	for _, e := range env.Entries {
		pr.Entries = append(pr.Entries, domain.PlacementEntry{Placement: e.Placement, SurvivedMs: e.SurvivedMs})
	}
	return pr, nil
}

// parseCollective desserializa o envelope collective (coop_mission, session/016).
// O analytics só agrega o desfecho da SESSÃO (ignora players/stats).
func parseCollective(body []byte) (domain.CollectiveResult, error) {
	var env struct {
		MatchID string `json:"matchId"`
		GameID  string `json:"gameId"`
		Outcome string `json:"outcome"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return domain.CollectiveResult{}, fmt.Errorf("unmarshal collective: %w", err)
	}
	return domain.CollectiveResult{MatchID: env.MatchID, GameID: env.GameID, Outcome: env.Outcome}, nil
}

// parse desserializa o envelope do fan-out (struct próprio, desacoplado).
func parse(body []byte) (domain.MatchResult, error) {
	var env struct {
		MatchID string `json:"matchId"`
		GameID  string `json:"gameId"`
		Results []struct {
			PlayerID string `json:"playerId"`
			Outcome  string `json:"outcome"` // "win"|"loss"|"draw" (ADR-0023)
			Won      bool   `json:"won"`     // compat/fallback (envelope v1)
			Score    int64  `json:"score"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return domain.MatchResult{}, fmt.Errorf("unmarshal envelope: %w", err)
	}
	m := domain.MatchResult{MatchID: env.MatchID, GameID: env.GameID}
	for _, r := range env.Results {
		m.Results = append(m.Results, domain.PlayerResult{
			PlayerID: r.PlayerID,
			Outcome:  outcomeOf(r.Outcome, r.Won),
			Won:      r.Won,
			Score:    r.Score,
		})
	}
	return m, nil
}

// outcomeOf devolve o desfecho do envelope; se ausente (v1 legado), deriva de
// won (win/loss) — o empate só existe no envelope v2 (ADR-0023).
func outcomeOf(outcome string, won bool) string {
	if outcome != "" {
		return outcome
	}
	if won {
		return domain.OutcomeWin
	}
	return domain.OutcomeLoss
}

func (c *Consumer) sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

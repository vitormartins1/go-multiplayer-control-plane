package publisher

import (
	"context"
	"log/slog"

	"github.com/vitormartins1/go-game-backend/services/session-service/internal/domain"
)

var _ domain.ResultPublisher = (*Composite)(nil)

// Composite distribui o resultado para N publishers (fan-out para N sinks:
// achievements, leaderboard, ...). Best-effort entre sinks: a falha de um não
// impede os outros nem propaga erro (cada adapter já é best-effort; aqui só
// garantimos que um sink quebrado não derruba o conjunto).
type Composite struct {
	log        *slog.Logger
	publishers []domain.ResultPublisher
}

// NewComposite agrupa os publishers numa única Port.
func NewComposite(log *slog.Logger, publishers ...domain.ResultPublisher) *Composite {
	return &Composite{log: log, publishers: publishers}
}

// Publish chama todos os publishers; loga quem falhar e segue.
func (c *Composite) Publish(ctx context.Context, gameID string, results []domain.PlayerResult) error {
	for _, p := range c.publishers {
		if err := p.Publish(ctx, gameID, results); err != nil {
			c.log.Warn("fan-out: sink falhou (best-effort)", slog.String("err", err.Error()))
		}
	}
	return nil
}

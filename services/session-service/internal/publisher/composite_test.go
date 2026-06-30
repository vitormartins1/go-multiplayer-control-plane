package publisher

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c130n/game-backend/services/session-service/internal/domain"
)

type spyPublisher struct {
	called int
	gameID string
	err    error
}

func (s *spyPublisher) Publish(_ context.Context, gameID string, _ []domain.PlayerResult) error {
	s.called++
	s.gameID = gameID
	return s.err
}

func TestComposite_ChamaTodos(t *testing.T) {
	a, b := &spyPublisher{}, &spyPublisher{}
	c := NewComposite(discard(), a, b)

	err := c.Publish(context.Background(), "game-1", []domain.PlayerResult{{PlayerID: "p1"}})
	require.NoError(t, err)
	assert.Equal(t, 1, a.called)
	assert.Equal(t, 1, b.called)
	assert.Equal(t, "game-1", a.gameID)
	assert.Equal(t, "game-1", b.gameID)
}

func TestComposite_UmFalha_NaoImpedeOOutro(t *testing.T) {
	bad := &spyPublisher{err: errors.New("achievements fora")}
	good := &spyPublisher{}
	c := NewComposite(discard(), bad, good)

	err := c.Publish(context.Background(), "game-1", nil)
	require.NoError(t, err, "best-effort: composite não propaga erro de um sink")
	assert.Equal(t, 1, good.called, "o sink bom é chamado mesmo com o outro falhando")
}

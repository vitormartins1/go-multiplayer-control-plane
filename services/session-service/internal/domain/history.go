package domain

import (
	"context"
	"time"
)

// HistoryStore é a Port de saída que persiste sessões fechadas (spec 025): o
// session-service é o único serviço stateful-por-natureza sem banco — sem
// isto, o histórico (aba Sessions do jogo / aba History do player) some no
// restart (o ring in-memory é bounded e não sobrevive ao processo). SaveClosed
// grava DEPOIS do release do server em closeSession — o fim da sessão nunca
// depende de um destino externo, mesma filosofia do fan-out (ADR-0020).
type HistoryStore interface {
	// SaveClosed grava um registro imutável da sessão recém-fechada, incluindo
	// os participantes com papel. Chamado uma única vez por sessão (write-once).
	SaveClosed(ctx context.Context, sess *Session) error
	// ClosedByGame lista as sessões fechadas do jogo, da mais recente para a
	// mais antiga, paginadas por closed_at (before zero = sem cursor, começa do
	// topo; before não-zero = cursor exclusivo, a próxima página).
	ClosedByGame(ctx context.Context, gameID string, limit int, before time.Time) ([]Session, error)
	// ClosedByPlayer lista as sessões fechadas do jogo em que o player
	// participou, mesma paginação de ClosedByGame.
	ClosedByPlayer(ctx context.Context, gameID, playerID string, limit int, before time.Time) ([]Session, error)
}

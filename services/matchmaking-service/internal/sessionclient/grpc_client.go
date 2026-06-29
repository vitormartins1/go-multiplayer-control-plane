// Package sessionclient é o adapter de saída que fala o gRPC do session-service.
// Implementa domain.SessionProvisioner: ao formar um match, chama CreateSession
// para alocar um game server e obter um connect token por jogador. Ver ADR-0003
// (gRPC) e ADR-0010 (control plane interno). É monorepo de módulo único
// (ADR-0001), então reusa o client gerado do session.
package sessionclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/c130n/game-backend/services/matchmaking-service/internal/domain"
	sessionv1 "github.com/c130n/game-backend/services/session-service/gen/session/v1"
)

var _ domain.SessionProvisioner = (*Client)(nil)

// Client adapta o sessionv1.SessionClient para a Port domain.SessionProvisioner.
type Client struct {
	session sessionv1.SessionClient
}

// New cria o adapter a partir de um client gRPC já conectado ao session.
func New(session sessionv1.SessionClient) *Client {
	return &Client{session: session}
}

// Provision chama o CreateSession do session e traduz a resposta para o domínio.
// codes.ResourceExhausted (pool esgotado) vira domain.ErrNoServer.
func (c *Client) Provision(ctx context.Context, matchID string, players []string) (domain.Provisioned, error) {
	resp, err := c.session.CreateSession(ctx, &sessionv1.CreateSessionRequest{
		MatchId: matchID,
		Players: players,
	})
	if err != nil {
		if status.Code(err) == codes.ResourceExhausted {
			return domain.Provisioned{}, domain.ErrNoServer
		}
		return domain.Provisioned{}, fmt.Errorf("sessionclient: provision: %w", err)
	}

	tokens := make(map[string]string, len(resp.GetGrants()))
	for _, g := range resp.GetGrants() {
		tokens[g.GetPlayerId()] = g.GetConnectToken()
	}
	return domain.Provisioned{ServerAddress: resp.GetServerAddress(), Tokens: tokens}, nil
}

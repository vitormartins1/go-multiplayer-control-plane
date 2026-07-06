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

	"github.com/vitormartins1/go-game-backend/services/matchmaking-service/internal/domain"
	sessionv1 "github.com/vitormartins1/go-game-backend/services/session-service/gen/session/v1"
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
func (c *Client) Provision(ctx context.Context, matchID string, teams []domain.TeamAssignment, gameID string) (domain.Provisioned, error) {
	players, pbTeams := flattenTeams(teams)
	resp, err := c.session.CreateSession(ctx, &sessionv1.CreateSessionRequest{
		MatchId: matchID,
		Players: players,
		GameId:  gameID,
		Teams:   pbTeams,
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

// flattenTeams monta a lista completa de players (união, em ordem) e os Team do
// proto. Em FFA (nenhum TeamID) os teams são omitidos (nil) — o session trata a
// ausência como partida sem times.
func flattenTeams(teams []domain.TeamAssignment) ([]string, []*sessionv1.Team) {
	var players []string
	hasTeams := false
	for _, t := range teams {
		players = append(players, t.Players...)
		if t.TeamID != "" {
			hasTeams = true
		}
	}
	if !hasTeams {
		return players, nil
	}
	pbTeams := make([]*sessionv1.Team, len(teams))
	for i, t := range teams {
		pbTeams[i] = &sessionv1.Team{TeamId: t.TeamID, Players: t.Players}
	}
	return players, pbTeams
}

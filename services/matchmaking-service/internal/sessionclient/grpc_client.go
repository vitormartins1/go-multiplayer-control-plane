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

	"github.com/vitormartins1/go-multiplayer-control-plane/services/matchmaking-service/internal/domain"
	sessionv1 "github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/gen/session/v1"
)

var (
	_ domain.SessionProvisioner = (*Client)(nil)
	_ domain.WorldDirectory     = (*Client)(nil)
	_ domain.WorldInjector      = (*Client)(nil)
)

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
func (c *Client) Provision(ctx context.Context, matchID string, teams []domain.TeamAssignment, gameID, modeID string) (domain.Provisioned, error) {
	players, pbTeams := flattenTeams(teams)
	resp, err := c.session.CreateSession(ctx, &sessionv1.CreateSessionRequest{
		MatchId: matchID,
		Players: players,
		GameId:  gameID,
		ModeId:  modeID, // o session congela o snapshot do perfil do modo (ADR-0024)
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

// ListInvadableWorlds chama o diretório do session (session/012) e traduz os
// mundos invadíveis para o domínio. A política de faixa é aplicada no service,
// não aqui — este adapter é passthrough.
func (c *Client) ListInvadableWorlds(ctx context.Context, gameID, modeID string) ([]domain.InvadableWorld, error) {
	resp, err := c.session.ListInvadableWorlds(ctx, &sessionv1.ListInvadableWorldsRequest{GameId: gameID, ModeId: modeID})
	if err != nil {
		return nil, fmt.Errorf("sessionclient: list invadable worlds: %w", err)
	}
	worlds := make([]domain.InvadableWorld, 0, len(resp.GetWorlds()))
	for _, w := range resp.GetWorlds() {
		worlds = append(worlds, domain.InvadableWorld{
			SessionID:        w.GetSessionId(),
			HostSoulLevel:    int(w.GetHostSoulLevel()),
			WeaponTier:       int(w.GetWeaponTier()),
			FreeInvaderSlots: int(w.GetFreeInvaderSlots()),
			PasswordHash:     w.GetPasswordHash(),
		})
	}
	return worlds, nil
}

// Inject injeta o invasor num mundo via JoinWorld do session (H0). Devolve o
// endereço do server (JoinWorldResponse.server_address, spec 012) + o token.
// codes.ResourceExhausted (slot cheio na corrida) → ErrRoleFull (dirige o retry);
// codes.NotFound (mundo fechou) → ErrWorldNotFound.
func (c *Client) Inject(ctx context.Context, sessionID, playerID, role, summonerID string) (domain.Injected, error) {
	resp, err := c.session.JoinWorld(ctx, &sessionv1.JoinWorldRequest{
		SessionId:        sessionID,
		PlayerId:         playerID,
		Role:             role,
		SummonerPlayerId: summonerID,
	})
	if err != nil {
		switch status.Code(err) {
		case codes.ResourceExhausted:
			return domain.Injected{}, domain.ErrRoleFull
		case codes.NotFound:
			return domain.Injected{}, domain.ErrWorldNotFound
		case codes.PermissionDenied:
			return domain.Injected{}, domain.ErrNotWorldHost
		default:
			return domain.Injected{}, fmt.Errorf("sessionclient: inject: %w", err)
		}
	}
	return domain.Injected{
		ServerAddress: resp.GetServerAddress(),
		ConnectToken:  resp.GetGrant().GetConnectToken(),
	}, nil
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

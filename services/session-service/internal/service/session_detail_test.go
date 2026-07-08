package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/domain"
)

// Spec 018: o outcome publicado fica gravado na sessão fechada (com team_id
// carimbado) — é o que o admin mostra no detalhe.
func TestReportResult_GravaResultNaSessaoFechada(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	pub := &mockPublisher{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	alloc.On("Release", mock.Anything, "server-0").Return(nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	pub.On("Publish", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	svc := New(alloc, iss, pub, WithProfileProvider(stubProfiles{}))

	teams := []domain.TeamAssignment{
		{TeamID: "team-1", Players: []string{"p1", "p2"}},
		{TeamID: "team-2", Players: []string{"p3", "p4"}},
	}
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", "teams-2v2", teams)
	require.NoError(t, err)

	// o server ecoa team_id errado de propósito — o gravado deve ser o carimbado
	outcome := wldOutcome(
		domain.PlayerOutcome{PlayerID: "p1", TeamID: "spoofed", Outcome: domain.ResultWin, Score: 4},
		domain.PlayerOutcome{PlayerID: "p3", Outcome: domain.ResultLoss, Score: 1},
	)
	require.NoError(t, svc.ReportResult(context.Background(), "match-1", outcome))

	_, closed := svc.SessionsByGame("game-1")
	require.Len(t, closed, 1)
	res := closed[0].Result
	require.NotNil(t, res, "outcome gravado na sessão fechada")
	require.NotNil(t, res.WinLossDraw)
	require.Len(t, res.WinLossDraw.Players, 2)
	assert.Equal(t, "team-1", res.WinLossDraw.Players[0].TeamID, "team_id carimbado, não o ecoado")
	assert.Equal(t, "team-2", res.WinLossDraw.Players[1].TeamID)
	assert.Equal(t, domain.CloseResult, closed[0].CloseReason)
}

// Publish falho: nada é gravado (a sessão segue aberta e sem Result).
func TestReportResult_PublishFalha_NaoGravaResult(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	pub := &mockPublisher{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	pub.On("Publish", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("sns fora"))
	svc := New(alloc, iss, pub, WithProfileProvider(stubProfiles{}))
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", "default", ffaTeams("p1"))
	require.NoError(t, err)

	require.Error(t, svc.ReportResult(context.Background(), "match-1", validOutcome()))

	active, _ := svc.SessionsByGame("game-1")
	require.Len(t, active, 1)
	assert.Nil(t, active[0].Result, "publish falho não grava resultado")
}

// Fechamento sem resultado (admin) fica com Result nil.
func TestCloseByAdmin_SemResult(t *testing.T) {
	alloc := &mockAllocator{}
	iss := &mockIssuer{}
	alloc.On("Allocate", mock.Anything, mock.Anything, mock.Anything).Return(domain.Server{ID: "server-0", Address: "10.0.0.1:7777"}, nil)
	alloc.On("Release", mock.Anything, "server-0").Return(nil)
	iss.On("Issue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	svc := New(alloc, iss, &mockPublisher{}, WithProfileProvider(stubProfiles{}))
	_, _, err := svc.Create(context.Background(), "match-1", "game-1", "default", ffaTeams("p1"))
	require.NoError(t, err)

	_, err = svc.CloseByAdmin(context.Background(), "match-1")
	require.NoError(t, err)

	_, closed := svc.SessionsByGame("game-1")
	require.Len(t, closed, 1)
	assert.Nil(t, closed[0].Result)
}

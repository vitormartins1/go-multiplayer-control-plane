package service

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/game-registry/internal/domain"
)

// Testes dos modos nomeados (ADR-0024, spec 009). O mockRepo vive em
// game_service_test.go (mesmo package).

func gameWithModes(modes ...domain.MultiplayerMode) *domain.Game {
	return &domain.Game{ID: "g1", MultiplayerModes: modes}
}

func TestSetMode_Novo_CriaEDevolve(t *testing.T) {
	repo := new(mockRepo)
	repo.On("FindByID", mock.Anything, "g1").Return(gameWithModes(), nil)
	repo.On("SetModes", mock.Anything, "g1", mock.MatchedBy(func(modes []domain.MultiplayerMode) bool {
		return len(modes) == 1 && modes[0].ModeID == "tdm" && modes[0].DisplayName == "Team Deathmatch"
	})).Return(nil)

	m, err := New(repo).SetMode(context.Background(), "g1", "tdm", "  Team Deathmatch  ", validProfile())
	require.NoError(t, err)
	assert.Equal(t, "tdm", m.ModeID)
	assert.Equal(t, "Team Deathmatch", m.DisplayName, "displayName trimmed")
	repo.AssertExpectations(t)
}

func TestSetMode_Existente_SubstituiSemDuplicar(t *testing.T) {
	repo := new(mockRepo)
	existing := domain.MultiplayerMode{ModeID: "tdm", DisplayName: "Old", Profile: validProfile()}
	repo.On("FindByID", mock.Anything, "g1").Return(gameWithModes(existing), nil)
	repo.On("SetModes", mock.Anything, "g1", mock.MatchedBy(func(modes []domain.MultiplayerMode) bool {
		return len(modes) == 1 && modes[0].ModeID == "tdm" && modes[0].DisplayName == "New"
	})).Return(nil)

	_, err := New(repo).SetMode(context.Background(), "g1", "tdm", "New", validProfile())
	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestSetMode_ModeIDInvalido_ErrInvalidModeID(t *testing.T) {
	casos := []string{"", "Tdm", "1abc", "-abc", "com_underscore", strings.Repeat("a", 33)}
	for _, id := range casos {
		t.Run(fmt.Sprintf("%q", id), func(t *testing.T) {
			_, err := New(new(mockRepo)).SetMode(context.Background(), "g1", id, "", validProfile())
			assert.ErrorIs(t, err, domain.ErrInvalidModeID)
		})
	}
}

func TestSetMode_DisplayNameLongo_ErrInvalidProfileParams(t *testing.T) {
	_, err := New(new(mockRepo)).SetMode(context.Background(), "g1", "tdm",
		strings.Repeat("x", 61), validProfile())
	assert.ErrorIs(t, err, domain.ErrInvalidProfileParams)
}

func TestSetMode_PerfilInvalido_ReusaValidacao(t *testing.T) {
	p := validProfile()
	p.QueueArena = nil // params ausentes -> regra da spec 006, reusada por modo
	_, err := New(new(mockRepo)).SetMode(context.Background(), "g1", "tdm", "", p)
	assert.ErrorIs(t, err, domain.ErrInvalidProfileParams)
}

func TestSetMode_LimiteDeModos_ErrTooManyModes(t *testing.T) {
	full := make([]domain.MultiplayerMode, maxModesPerGame)
	for i := range full {
		full[i] = domain.MultiplayerMode{ModeID: fmt.Sprintf("m-%02d", i), Profile: validProfile()}
	}
	repo := new(mockRepo)
	repo.On("FindByID", mock.Anything, "g1").Return(gameWithModes(full...), nil)

	_, err := New(repo).SetMode(context.Background(), "g1", "novo", "", validProfile())
	assert.ErrorIs(t, err, domain.ErrTooManyModes)
}

func TestSetMode_LimiteNaoBloqueiaUpdate(t *testing.T) {
	full := make([]domain.MultiplayerMode, maxModesPerGame)
	for i := range full {
		full[i] = domain.MultiplayerMode{ModeID: fmt.Sprintf("m-%02d", i), Profile: validProfile()}
	}
	repo := new(mockRepo)
	repo.On("FindByID", mock.Anything, "g1").Return(gameWithModes(full...), nil)
	repo.On("SetModes", mock.Anything, "g1", mock.Anything).Return(nil)

	_, err := New(repo).SetMode(context.Background(), "g1", "m-00", "atualizado", validProfile())
	assert.NoError(t, err, "upsert de modo existente não conta contra o cap")
}

func TestSetMode_JogoInexistente_ErrGameNotFound(t *testing.T) {
	repo := new(mockRepo)
	repo.On("FindByID", mock.Anything, "nao").Return(nil, domain.ErrGameNotFound)
	_, err := New(repo).SetMode(context.Background(), "nao", "tdm", "", validProfile())
	assert.ErrorIs(t, err, domain.ErrGameNotFound)
}

func TestGetMode_Existe_Devolve(t *testing.T) {
	repo := new(mockRepo)
	repo.On("FindByID", mock.Anything, "g1").Return(
		gameWithModes(domain.MultiplayerMode{ModeID: "tdm", DisplayName: "TDM", Profile: validProfile()}), nil)

	m, err := New(repo).GetMode(context.Background(), "g1", "tdm")
	require.NoError(t, err)
	assert.Equal(t, "TDM", m.DisplayName)
}

func TestGetMode_Inexistente_ErrModeNotFound(t *testing.T) {
	repo := new(mockRepo)
	repo.On("FindByID", mock.Anything, "g1").Return(gameWithModes(), nil)
	_, err := New(repo).GetMode(context.Background(), "g1", "nao")
	assert.ErrorIs(t, err, domain.ErrModeNotFound)
}

func TestListModes_DevolveColecao(t *testing.T) {
	repo := new(mockRepo)
	repo.On("FindByID", mock.Anything, "g1").Return(gameWithModes(
		domain.MultiplayerMode{ModeID: domain.DefaultModeID, Profile: validProfile()},
		domain.MultiplayerMode{ModeID: "tdm", Profile: validProfile()},
	), nil)

	modes, err := New(repo).ListModes(context.Background(), "g1")
	require.NoError(t, err)
	require.Len(t, modes, 2)
	assert.Equal(t, domain.DefaultModeID, modes[0].ModeID, "ordem do decode preservada (default primeiro)")
}

func TestDeleteMode_Default_ErrDefaultModeProtected(t *testing.T) {
	repo := new(mockRepo)
	repo.On("FindByID", mock.Anything, "g1").Return(
		gameWithModes(domain.MultiplayerMode{ModeID: domain.DefaultModeID, Profile: validProfile()}), nil)

	err := New(repo).DeleteMode(context.Background(), "g1", domain.DefaultModeID)
	assert.ErrorIs(t, err, domain.ErrDefaultModeProtected)
}

func TestDeleteMode_Inexistente_ErrModeNotFound(t *testing.T) {
	repo := new(mockRepo)
	repo.On("FindByID", mock.Anything, "g1").Return(gameWithModes(), nil)
	err := New(repo).DeleteMode(context.Background(), "g1", "nao")
	assert.ErrorIs(t, err, domain.ErrModeNotFound)
}

func TestDeleteMode_Valido_RemoveEPersiste(t *testing.T) {
	repo := new(mockRepo)
	repo.On("FindByID", mock.Anything, "g1").Return(gameWithModes(
		domain.MultiplayerMode{ModeID: domain.DefaultModeID, Profile: validProfile()},
		domain.MultiplayerMode{ModeID: "tdm", Profile: validProfile()},
	), nil)
	repo.On("SetModes", mock.Anything, "g1", mock.MatchedBy(func(modes []domain.MultiplayerMode) bool {
		return len(modes) == 1 && modes[0].ModeID == domain.DefaultModeID
	})).Return(nil)

	require.NoError(t, New(repo).DeleteMode(context.Background(), "g1", "tdm"))
	repo.AssertExpectations(t)
}

// O alias legado (specs 006-008) opera o modo default e PRESERVA o displayName
// existente — o corpo legado não carrega displayName (ADR-0024, plan §5).
func TestSetMultiplayerProfile_Legado_PreservaDisplayNameDoDefault(t *testing.T) {
	repo := new(mockRepo)
	repo.On("FindByID", mock.Anything, "g1").Return(gameWithModes(
		domain.MultiplayerMode{ModeID: domain.DefaultModeID, DisplayName: "Partida Padrão", Profile: validProfile()},
	), nil)
	repo.On("SetModes", mock.Anything, "g1", mock.MatchedBy(func(modes []domain.MultiplayerMode) bool {
		return len(modes) == 1 && modes[0].DisplayName == "Partida Padrão"
	})).Return(nil)

	_, err := New(repo).SetMultiplayerProfile(context.Background(), "g1", validProfile())
	require.NoError(t, err)
	repo.AssertExpectations(t)
}

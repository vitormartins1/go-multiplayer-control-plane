package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/achievements-service/internal/domain"
)

// crudEventRules registra o que as operações de admin de regras (ADR-0027)
// entregam ao repositório, e devolve um erro programado.
type crudEventRules struct {
	fakeEventRules
	err        error
	effective  []domain.EventRule
	gotCreate  *domain.EventRule
	gotGameID  string
	gotRuleID  string
	listCalled bool
}

func (c *crudEventRules) ListEffective(_ context.Context, gameID string) ([]domain.EventRule, error) {
	c.listCalled, c.gotGameID = true, gameID
	return c.effective, c.err
}

func (c *crudEventRules) CreateRule(_ context.Context, r *domain.EventRule) error {
	c.gotCreate = r
	return c.err
}

func (c *crudEventRules) DeleteRule(_ context.Context, gameID, id string) error {
	c.gotGameID, c.gotRuleID = gameID, id
	return c.err
}

var errRepo = errors.New("db fora")

func TestCreateEventRule_GameIDVemDoPath_NormalizaMatchNil(t *testing.T) {
	rules := &crudEventRules{}
	svc := New(new(mockRepo), rules)
	outro := "outro-jogo"

	got, err := svc.CreateEventRule(context.Background(), gameID, domain.EventRule{
		GameID:     &outro, // payload tentando escopar em outro jogo
		Archetype:  domain.ArchetypeHostWorldDropIn,
		EventTypes: []string{"boss_boitata_derrotado"},
	})

	require.NoError(t, err)
	require.NotNil(t, got.GameID)
	assert.Equal(t, gameID, *got.GameID, "gameID do path sobrescreve o do payload")
	assert.NotNil(t, got.Match, "match nil vira regra \"sempre bate\"")
	assert.Same(t, got, rules.gotCreate)
}

func TestCreateEventRule_Invalida_NaoTocaRepo(t *testing.T) {
	cases := map[string]domain.EventRule{
		"arquétipo desconhecido": {Archetype: "mmo", EventTypes: []string{"x"}},
		"sem eventTypes":         {Archetype: domain.ArchetypeQueueArena},
		"eventType malformado":   {Archetype: domain.ArchetypeQueueArena, EventTypes: []string{"Match Won"}},
		"chave de match vazia":   {Archetype: domain.ArchetypeQueueArena, EventTypes: []string{"x"}, Match: map[string]domain.MatchCond{" ": {Value: "win"}}},
	}
	for name, r := range cases {
		t.Run(name, func(t *testing.T) {
			rules := &crudEventRules{}
			_, err := New(new(mockRepo), rules).CreateEventRule(context.Background(), gameID, r)
			require.ErrorIs(t, err, domain.ErrInvalidEventRule)
			assert.Nil(t, rules.gotCreate)
		})
	}
}

func TestCreateEventRule_ErroDoRepo_Propaga(t *testing.T) {
	rules := &crudEventRules{err: errRepo}
	_, err := New(new(mockRepo), rules).CreateEventRule(context.Background(), gameID, domain.EventRule{
		Archetype: domain.ArchetypeQueueArena, EventTypes: []string{"match_won"},
	})
	require.ErrorIs(t, err, errRepo)
}

func TestListEffectiveRules_DelegaEPropagaErro(t *testing.T) {
	rules := &crudEventRules{effective: []domain.EventRule{{ID: "r1"}}}
	got, err := New(new(mockRepo), rules).ListEffectiveRules(context.Background(), gameID)
	require.NoError(t, err)
	assert.Equal(t, gameID, rules.gotGameID)
	assert.Len(t, got, 1)

	rules.err = errRepo
	_, err = New(new(mockRepo), rules).ListEffectiveRules(context.Background(), gameID)
	require.ErrorIs(t, err, errRepo)
}

func TestDeleteEventRule_RepassaGameIDEIdEPropagaErro(t *testing.T) {
	rules := &crudEventRules{}
	require.NoError(t, New(new(mockRepo), rules).DeleteEventRule(context.Background(), gameID, "r9"))
	assert.Equal(t, gameID, rules.gotGameID)
	assert.Equal(t, "r9", rules.gotRuleID)

	rules.err = domain.ErrEventRuleNotFound
	err := New(new(mockRepo), rules).DeleteEventRule(context.Background(), gameID, "global-1")
	require.ErrorIs(t, err, domain.ErrEventRuleNotFound, "regra global/inexistente não é apagável pelo jogo")
}

func TestListCatalog_DelegaEPropagaErro(t *testing.T) {
	repo := new(mockRepo)
	repo.On("ListCatalog", mock.Anything, gameID).Return([]domain.Achievement{{ID: "first_win"}}, nil).Once()
	repo.On("ListCatalog", mock.Anything, gameID).Return([]domain.Achievement(nil), errRepo).Once()
	svc := New(repo, fakeEventRules{})

	got, err := svc.ListCatalog(context.Background(), gameID)
	require.NoError(t, err)
	assert.Len(t, got, 1)

	_, err = svc.ListCatalog(context.Background(), gameID)
	require.ErrorIs(t, err, errRepo)
}

func TestProgressForAchievement_DelegaEPropagaErro(t *testing.T) {
	repo := new(mockRepo)
	repo.On("ProgressForAchievement", mock.Anything, gameID, "a1").Return([]domain.AchievementProgressEntry{{PlayerID: "p1"}}, nil).Once()
	repo.On("ProgressForAchievement", mock.Anything, gameID, "a1").Return(nil, errRepo).Once()
	svc := New(repo, fakeEventRules{})

	got, err := svc.ProgressForAchievement(context.Background(), gameID, "a1")
	require.NoError(t, err)
	assert.Len(t, got, 1)

	_, err = svc.ProgressForAchievement(context.Background(), gameID, "a1")
	require.ErrorIs(t, err, errRepo)
}

func TestDeleteAchievement_ErroDoRepo_Propaga(t *testing.T) {
	repo := new(mockRepo)
	repo.On("DeleteAchievement", mock.Anything, gameID, "a1").Return(errRepo)
	err := New(repo, fakeEventRules{}).DeleteAchievement(context.Background(), gameID, "a1")
	require.ErrorIs(t, err, errRepo)
}

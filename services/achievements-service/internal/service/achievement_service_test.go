package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/achievements-service/internal/domain"
)

const gameID = "game-1"

type mockRepo struct{ mock.Mock }

func (m *mockRepo) ListCatalog(ctx context.Context, gameID string) ([]domain.Achievement, error) {
	args := m.Called(ctx, gameID)
	return args.Get(0).([]domain.Achievement), args.Error(1)
}

func (m *mockRepo) FindByEventType(ctx context.Context, gameID, eventType string) ([]domain.Achievement, error) {
	args := m.Called(ctx, gameID, eventType)
	return args.Get(0).([]domain.Achievement), args.Error(1)
}

func (m *mockRepo) FindByStatKey(ctx context.Context, gameID, statKey string) ([]domain.Achievement, error) {
	args := m.Called(ctx, gameID, statKey)
	return args.Get(0).([]domain.Achievement), args.Error(1)
}

func (m *mockRepo) ListPlayerProgress(ctx context.Context, gameID, playerID string) ([]domain.PlayerProgress, error) {
	args := m.Called(ctx, gameID, playerID)
	return args.Get(0).([]domain.PlayerProgress), args.Error(1)
}

func (m *mockRepo) RecordProgress(ctx context.Context, gameID, playerID, achievementID string, target int, at time.Time) (bool, error) {
	args := m.Called(ctx, gameID, playerID, achievementID, target, at)
	return args.Bool(0), args.Error(1)
}

func (m *mockRepo) ApplyMatchResult(ctx context.Context, gameID, matchID string, players []domain.PlayerMatchInput, at time.Time) (bool, error) {
	args := m.Called(ctx, gameID, matchID, players, at)
	return args.Bool(0), args.Error(1)
}

func (m *mockRepo) ApplyParticipationEvent(ctx context.Context, gameID, sessionID, playerID, event, eventID string, eventTypes []string, stats map[string]int64, at time.Time) (bool, error) {
	args := m.Called(ctx, gameID, sessionID, playerID, event, eventID, eventTypes, stats, at)
	return args.Bool(0), args.Error(1)
}

func (m *mockRepo) ProgressForAchievement(ctx context.Context, gameID, achievementID string) ([]domain.AchievementProgressEntry, error) {
	args := m.Called(ctx, gameID, achievementID)
	e, _ := args.Get(0).([]domain.AchievementProgressEntry)
	return e, args.Error(1)
}

func (m *mockRepo) CreateAchievement(ctx context.Context, a *domain.Achievement) error {
	return m.Called(ctx, a).Error(0)
}
func (m *mockRepo) UpdateAchievement(ctx context.Context, a *domain.Achievement) error {
	return m.Called(ctx, a).Error(0)
}
func (m *mockRepo) DeleteAchievement(ctx context.Context, gameID, id string) error {
	return m.Called(ctx, gameID, id).Error(0)
}

var (
	firstWin = domain.Achievement{GameID: gameID, ID: "first_win", Name: "Primeira Vitória", Description: "Vença uma", EventType: "match_won", Target: 1}
	tenWins  = domain.Achievement{GameID: gameID, ID: "ten_wins", Name: "Veterano", Description: "Vença 10", EventType: "match_won", Target: 10}
)

// fakeEventRules é um domain.EventRuleRepository fixo (fixture de teste) com
// as MESMAS regras do seed da migration 008 — mantém estes testes validando a
// orquestração do Service (buscar regras, casar facts via EvaluateRules,
// delegar ao repo de catálogo), não a tabela em si (isso é testado em
// repository/postgres_event_rule_test.go).
type fakeEventRules struct{}

func (fakeEventRules) RulesForArchetype(_ context.Context, _, archetype string) ([]domain.EventRule, error) {
	switch archetype {
	case domain.ArchetypeQueueArena:
		return []domain.EventRule{
			{Archetype: archetype, Match: domain.EqMatch(map[string]string{}), EventTypes: []string{"match_played"}},
			{Archetype: archetype, Match: domain.EqMatch(map[string]string{"outcome": "win"}), EventTypes: []string{"match_won"}},
			{Archetype: archetype, Match: domain.EqMatch(map[string]string{"outcome": "loss"}), EventTypes: []string{"match_lost"}},
			{Archetype: archetype, Match: domain.EqMatch(map[string]string{"outcome": "draw"}), EventTypes: []string{"match_drawn"}},
		}, nil
	case domain.ArchetypeBattleRoyale:
		return []domain.EventRule{
			{Archetype: archetype, Match: domain.EqMatch(map[string]string{}), EventTypes: []string{"battle_royale_played"}},
			{Archetype: archetype, Match: domain.EqMatch(map[string]string{"placement": "1"}), EventTypes: []string{"battle_royale_win"}},
			{Archetype: archetype, Match: map[string]domain.MatchCond{"placement": {Op: domain.OpLte, Value: float64(10)}}, EventTypes: []string{"battle_royale_top_n"}},
		}, nil
	case domain.ArchetypeCoopMission:
		return []domain.EventRule{
			{Archetype: archetype, Match: domain.EqMatch(map[string]string{}), EventTypes: []string{"coop_mission_played"}},
			{Archetype: archetype, Match: domain.EqMatch(map[string]string{"outcome": domain.MissionSuccess}), EventTypes: []string{"coop_mission_success"}},
			{Archetype: archetype, Match: domain.EqMatch(map[string]string{"outcome": domain.MissionFailure}), EventTypes: []string{"coop_mission_failed"}},
		}, nil
	case domain.ArchetypeHostWorldDropIn:
		return []domain.EventRule{
			{Archetype: archetype, Match: domain.EqMatch(map[string]string{"event": "JOINED"}), EventTypes: []string{domain.EventWorldJoined}},
			{Archetype: archetype, Match: domain.EqMatch(map[string]string{"role": "coop", "event": "BOSS_DEFEATED", "outcome": "SUCCESS"}), EventTypes: []string{domain.EventCoopBossHelped}},
			{Archetype: archetype, Match: domain.EqMatch(map[string]string{"role": "invader", "event": "HOST_KILLED", "outcome": "SUCCESS"}), EventTypes: []string{domain.EventInvasionWon}},
		}, nil
	}
	return nil, nil
}

func (fakeEventRules) ListEffective(context.Context, string) ([]domain.EventRule, error) {
	return nil, nil
}
func (fakeEventRules) CreateRule(context.Context, *domain.EventRule) error { return nil }
func (fakeEventRules) UpdateRule(context.Context, *domain.EventRule) error { return nil }
func (fakeEventRules) DeleteRule(context.Context, string, string) error    { return nil }
func (fakeEventRules) Suppress(context.Context, string, string) error      { return nil }
func (fakeEventRules) Unsuppress(context.Context, string, string) error    { return nil }

// spyEventRules estende o fake capturando o que chega no UpdateRule (spec
// 016) — valida a orquestração do service (id/gameID do path, validação de
// conteúdo ANTES de tocar o repo).
type spyEventRules struct {
	fakeEventRules
	updateErr error
	gotUpdate *domain.EventRule
}

func (s *spyEventRules) UpdateRule(_ context.Context, r *domain.EventRule) error {
	s.gotUpdate = r
	if s.updateErr != nil {
		return s.updateErr
	}
	r.Archetype = "coop_mission" // RETURNING do repo real preenche
	return nil
}

func TestUpdateEventRule_SubstituiConteudoPreservandoID(t *testing.T) {
	rules := &spyEventRules{}
	svc := New(new(mockRepo), rules)

	got, err := svc.UpdateEventRule(context.Background(), gameID, "r7", domain.EventRule{
		Match:      map[string]domain.MatchCond{"stat.revives": {Op: domain.OpGte, Value: float64(6)}},
		EventTypes: []string{"mestre_curativo"},
	})
	require.NoError(t, err)
	assert.Equal(t, "r7", got.ID)
	require.NotNil(t, got.GameID)
	assert.Equal(t, gameID, *got.GameID)
	assert.Equal(t, "coop_mission", got.Archetype, "archetype vem do banco, não do payload")
}

func TestUpdateEventRule_ConteudoInvalido_NaoTocaRepo(t *testing.T) {
	rules := &spyEventRules{}
	svc := New(new(mockRepo), rules)

	_, err := svc.UpdateEventRule(context.Background(), gameID, "r7", domain.EventRule{
		Match:      map[string]domain.MatchCond{"placement": {Op: "entre", Value: float64(1)}}, // op desconhecido
		EventTypes: []string{"x"},
	})
	require.ErrorIs(t, err, domain.ErrInvalidEventRule)
	assert.Nil(t, rules.gotUpdate, "validação estrita acontece antes do repo")

	_, err = svc.UpdateEventRule(context.Background(), gameID, "r7", domain.EventRule{EventTypes: nil})
	require.ErrorIs(t, err, domain.ErrInvalidEventRule)
}

func TestUpdateEventRule_GlobalOuInexistente_PropagaNotFound(t *testing.T) {
	rules := &spyEventRules{updateErr: domain.ErrEventRuleNotFound}
	svc := New(new(mockRepo), rules)

	_, err := svc.UpdateEventRule(context.Background(), gameID, "global-1", domain.EventRule{
		EventTypes: []string{"x"},
	})
	require.ErrorIs(t, err, domain.ErrEventRuleNotFound)
}

// spySuppressRules estende o fake capturando o que chega em
// Suppress/Unsuppress (spec 017) — valida que o service repassa gameID/id do
// path sem alteração (sem conteúdo pra validar, ao contrário de Create/
// Update) e propaga o erro do repo tal como veio.
type spySuppressRules struct {
	fakeEventRules
	suppressErr, unsuppressErr         error
	gotSuppressGame, gotSuppressID     string
	gotUnsuppressGame, gotUnsuppressID string
}

func (s *spySuppressRules) Suppress(_ context.Context, gameID, id string) error {
	s.gotSuppressGame, s.gotSuppressID = gameID, id
	return s.suppressErr
}

func (s *spySuppressRules) Unsuppress(_ context.Context, gameID, id string) error {
	s.gotUnsuppressGame, s.gotUnsuppressID = gameID, id
	return s.unsuppressErr
}

func TestSuppressEventRule_RepassaGameIDEId(t *testing.T) {
	rules := &spySuppressRules{}
	svc := New(new(mockRepo), rules)

	err := svc.SuppressEventRule(context.Background(), gameID, "global-1")
	require.NoError(t, err)
	assert.Equal(t, gameID, rules.gotSuppressGame)
	assert.Equal(t, "global-1", rules.gotSuppressID)
}

func TestSuppressEventRule_PropagaErroDoRepo(t *testing.T) {
	rules := &spySuppressRules{suppressErr: domain.ErrEventRuleNotFound}
	svc := New(new(mockRepo), rules)

	err := svc.SuppressEventRule(context.Background(), gameID, "override-1")
	require.ErrorIs(t, err, domain.ErrEventRuleNotFound)
}

func TestUnsuppressEventRule_RepassaGameIDEId(t *testing.T) {
	rules := &spySuppressRules{}
	svc := New(new(mockRepo), rules)

	err := svc.UnsuppressEventRule(context.Background(), gameID, "global-1")
	require.NoError(t, err)
	assert.Equal(t, gameID, rules.gotUnsuppressGame)
	assert.Equal(t, "global-1", rules.gotUnsuppressID)
}

func TestUnsuppressEventRule_PropagaErroDoRepo(t *testing.T) {
	rules := &spySuppressRules{unsuppressErr: domain.ErrEventRuleNotFound}
	svc := New(new(mockRepo), rules)

	err := svc.UnsuppressEventRule(context.Background(), gameID, "global-1")
	require.ErrorIs(t, err, domain.ErrEventRuleNotFound)
}

// ApplyMatchResult traduz o resultado bruto em eventos: match_played p/ todos,
// match_won só p/ quem venceu. A idempotência é do repo (tx + processed_matches);
// aqui validamos só a tradução delegada.
func TestApplyMatchResult_TraduzEventosPorVitoria(t *testing.T) {
	repo := new(mockRepo)
	svc := New(repo, fakeEventRules{})

	results := []domain.MatchResult{
		{PlayerID: "p1", Won: true},
		{PlayerID: "p2", Won: false},
	}
	want := []domain.PlayerMatchInput{
		{PlayerID: "p1", EventTypes: []string{"match_played", "match_won"}},
		{PlayerID: "p2", EventTypes: []string{"match_played"}},
	}
	repo.On("ApplyMatchResult", mock.Anything, gameID, "match-1", want, mock.Anything).Return(false, nil)

	dup, err := svc.ApplyMatchResult(context.Background(), gameID, "match-1", "", results)
	require.NoError(t, err)
	assert.False(t, dup)
	repo.AssertExpectations(t)
}

// Traduz o outcome (ADR-0023) em eventos, com fallback para Won quando ausente
// e sem falhar em valor desconhecido.
func TestApplyMatchResult_OutcomeParaEventos(t *testing.T) {
	cases := map[string]struct {
		result domain.MatchResult
		want   []string
	}{
		"win":           {domain.MatchResult{PlayerID: "p", Outcome: "win"}, []string{"match_played", "match_won"}},
		"loss":          {domain.MatchResult{PlayerID: "p", Outcome: "loss"}, []string{"match_played", "match_lost"}},
		"draw":          {domain.MatchResult{PlayerID: "p", Outcome: "draw"}, []string{"match_played", "match_drawn"}},
		"fallback won":  {domain.MatchResult{PlayerID: "p", Won: true}, []string{"match_played", "match_won"}},
		"fallback loss": {domain.MatchResult{PlayerID: "p", Won: false}, []string{"match_played"}},
		"invalido":      {domain.MatchResult{PlayerID: "p", Outcome: "victory"}, []string{"match_played"}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			repo := new(mockRepo)
			svc := New(repo, fakeEventRules{})
			want := []domain.PlayerMatchInput{{PlayerID: "p", EventTypes: c.want}}
			repo.On("ApplyMatchResult", mock.Anything, gameID, "m", want, mock.Anything).Return(false, nil)

			_, err := svc.ApplyMatchResult(context.Background(), gameID, "m", "", []domain.MatchResult{c.result})
			require.NoError(t, err)
			repo.AssertExpectations(t)
		})
	}
}

// ApplyPlacementResult (battle_royale, BR3) traduz a colocação em event types e
// injeta survived_ms nos stats; a idempotência é do repo (validamos a tradução).
func TestApplyPlacementResult_TraduzEventTypesEInjetaSurvival(t *testing.T) {
	repo := new(mockRepo)
	svc := New(repo, fakeEventRules{})

	entries := []domain.PlacementEntry{
		{PlayerID: "p1", Placement: 1, SurvivedMs: 900000, Stats: map[string]int64{"kills": 4}},
		{PlayerID: "p2", Placement: 30, SurvivedMs: 300000},
	}
	want := []domain.PlayerMatchInput{
		{PlayerID: "p1", EventTypes: []string{"battle_royale_played", "battle_royale_win", "battle_royale_top_n"}, Stats: map[string]int64{"kills": 4, "survived_ms": 900000}},
		{PlayerID: "p2", EventTypes: []string{"battle_royale_played"}, Stats: map[string]int64{"survived_ms": 300000}},
	}
	repo.On("ApplyMatchResult", mock.Anything, gameID, "match-br", want, mock.Anything).Return(false, nil)

	dup, err := svc.ApplyPlacementResult(context.Background(), gameID, "match-br", "", entries)
	require.NoError(t, err)
	assert.False(t, dup)
	repo.AssertExpectations(t)
}

// survived_ms explícito nos stats não é sobrescrito pela sobrevivência de topo.
func TestApplyPlacementResult_SurvivedMsExplicito_NaoSobrescreve(t *testing.T) {
	repo := new(mockRepo)
	svc := New(repo, fakeEventRules{})

	entries := []domain.PlacementEntry{
		{PlayerID: "p1", Placement: 5, SurvivedMs: 111, Stats: map[string]int64{"survived_ms": 999}},
	}
	want := []domain.PlayerMatchInput{
		{PlayerID: "p1", EventTypes: []string{"battle_royale_played", "battle_royale_top_n"}, Stats: map[string]int64{"survived_ms": 999}},
	}
	repo.On("ApplyMatchResult", mock.Anything, gameID, "m", want, mock.Anything).Return(false, nil)

	_, err := svc.ApplyPlacementResult(context.Background(), gameID, "m", "", entries)
	require.NoError(t, err)
	repo.AssertExpectations(t)
}

// ApplyCollectiveResult (coop_mission, C3): o desfecho coletivo vira played +
// success/failed IGUAL para todos; os stats seguem por jogador.
func TestApplyCollectiveResult_DesfechoColetivoParaTodos(t *testing.T) {
	repo := new(mockRepo)
	svc := New(repo, fakeEventRules{})

	players := []domain.CollectivePlayer{
		{PlayerID: "p1", Stats: map[string]int64{"kills": 12}},
		{PlayerID: "p2", Stats: map[string]int64{"revives": 3}},
	}
	want := []domain.PlayerMatchInput{
		{PlayerID: "p1", EventTypes: []string{"coop_mission_played", "coop_mission_success"}, Stats: map[string]int64{"kills": 12}},
		{PlayerID: "p2", EventTypes: []string{"coop_mission_played", "coop_mission_success"}, Stats: map[string]int64{"revives": 3}},
	}
	repo.On("ApplyMatchResult", mock.Anything, gameID, "match-coop", want, mock.Anything).Return(false, nil)

	dup, err := svc.ApplyCollectiveResult(context.Background(), gameID, "match-coop", "", domain.MissionSuccess, players)
	require.NoError(t, err)
	assert.False(t, dup)
	repo.AssertExpectations(t)
}

// Falha da missão: todos recebem played + failed.
func TestApplyCollectiveResult_Falha(t *testing.T) {
	repo := new(mockRepo)
	svc := New(repo, fakeEventRules{})

	players := []domain.CollectivePlayer{{PlayerID: "p1"}}
	want := []domain.PlayerMatchInput{
		{PlayerID: "p1", EventTypes: []string{"coop_mission_played", "coop_mission_failed"}, Stats: nil},
	}
	repo.On("ApplyMatchResult", mock.Anything, gameID, "m", want, mock.Anything).Return(true, nil)

	dup, err := svc.ApplyCollectiveResult(context.Background(), gameID, "m", "", domain.MissionFailure, players)
	require.NoError(t, err)
	assert.True(t, dup, "reprocessar mesmo matchId é no-op")
	repo.AssertExpectations(t)
}

func TestApplyParticipationEvent_TraduzPorPapel(t *testing.T) {
	cases := map[string]struct {
		role, event, outcome string
		want                 []string
	}{
		"invasor vence":  {"invader", "HOST_KILLED", "SUCCESS", []string{domain.EventInvasionWon}},
		"coop ajuda":     {"coop", "BOSS_DEFEATED", "SUCCESS", []string{domain.EventCoopBossHelped}},
		"joined":         {"coop", "JOINED", "NEUTRAL", []string{domain.EventWorldJoined}},
		"left sem trad.": {"coop", "LEFT", "NEUTRAL", nil},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			repo := new(mockRepo)
			svc := New(repo, fakeEventRules{})
			repo.On("ApplyParticipationEvent", mock.Anything, gameID, "world-1", "p1", c.event, mock.Anything, c.want, mock.Anything, mock.Anything).Return(false, nil)

			dup, err := svc.ApplyParticipationEvent(context.Background(), gameID, "world-1", "p1", "", c.role, c.event, c.outcome, "evt-1", nil)
			require.NoError(t, err)
			assert.False(t, dup)
			repo.AssertExpectations(t)
		})
	}
}

func TestApplyParticipationEvent_Duplicado_PropagaDup(t *testing.T) {
	repo := new(mockRepo)
	svc := New(repo, fakeEventRules{})
	repo.On("ApplyParticipationEvent", mock.Anything, gameID, "world-1", "p1", "HOST_KILLED", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	dup, err := svc.ApplyParticipationEvent(context.Background(), gameID, "world-1", "p1", "", "invader", "HOST_KILLED", "SUCCESS", "evt-2", nil)
	require.NoError(t, err)
	assert.True(t, dup)
}

// spyStatRules captura os facts recebidos por EvaluateRules indiretamente:
// aqui usamos uma regra real com comparador (spec 018) pra provar que
// stat.deaths chega como fact — a MESMA regra global de battle_royale
// (RulesForArchetype) devolveria host_world_dropin vazio no fake, então o
// teste usa um fakeEventRules que devolve uma regra HWD com comparador.
type hwdStatRules struct{ fakeEventRules }

func (hwdStatRules) RulesForArchetype(_ context.Context, _, archetype string) ([]domain.EventRule, error) {
	if archetype != domain.ArchetypeHostWorldDropIn {
		return nil, nil
	}
	return []domain.EventRule{
		{Archetype: archetype, Match: map[string]domain.MatchCond{
			"role": domain.Eq("invader"), "event": domain.Eq("HOST_KILLED"), "outcome": domain.Eq("SUCCESS"),
			"stat.deaths": {Op: domain.OpLte, Value: float64(0)},
		}, EventTypes: []string{"flawless_invasion"}},
	}, nil
}

// stats (spec 018) viram facts stat.* — uma regra com comparador em
// host_world_dropin passa a casar (antes desta spec, HWD nunca recebia
// stats, então essa regra nunca teria como bater).
func TestApplyParticipationEvent_StatsViramFacts_RegraComComparadorCasa(t *testing.T) {
	repo := new(mockRepo)
	svc := New(repo, hwdStatRules{})
	repo.On("ApplyParticipationEvent", mock.Anything, gameID, "world-1", "p1", "HOST_KILLED",
		mock.Anything, []string{"flawless_invasion"}, map[string]int64{"deaths": 0}, mock.Anything).Return(false, nil)

	dup, err := svc.ApplyParticipationEvent(context.Background(), gameID, "world-1", "p1", "",
		"invader", "HOST_KILLED", "SUCCESS", "evt-3", map[string]int64{"deaths": 0})
	require.NoError(t, err)
	assert.False(t, dup)
	repo.AssertExpectations(t)
}

// Partida já processada -> o repo devolve dup=true; o service propaga (no-op).
func TestApplyMatchResult_Duplicada_PropagaDup(t *testing.T) {
	repo := new(mockRepo)
	svc := New(repo, fakeEventRules{})
	repo.On("ApplyMatchResult", mock.Anything, gameID, "match-1", mock.Anything, mock.Anything).Return(true, nil)

	dup, err := svc.ApplyMatchResult(context.Background(), gameID, "match-1", "", []domain.MatchResult{{PlayerID: "p1", Won: true}})
	require.NoError(t, err)
	assert.True(t, dup)
	repo.AssertExpectations(t)
}

func TestUpdateAchievement_Valido_DelegaComChave(t *testing.T) {
	repo := new(mockRepo)
	var captured *domain.Achievement
	repo.On("UpdateAchievement", mock.Anything, mock.AnythingOfType("*domain.Achievement")).
		Return(nil).
		Run(func(args mock.Arguments) { captured = args.Get(1).(*domain.Achievement) })

	a, err := New(repo, fakeEventRules{}).UpdateAchievement(context.Background(), gameID, "first_win",
		domain.Achievement{Name: "  Novo Nome  ", EventType: "match_won", Target: 3})
	require.NoError(t, err)
	assert.Equal(t, gameID, captured.GameID)
	assert.Equal(t, "first_win", captured.ID, "id/gameID vêm do path, não do corpo")
	assert.Equal(t, "Novo Nome", a.Name, "name trimado")
	assert.Equal(t, 3, a.Target)
}

func TestUpdateAchievement_NameVazio_ErrInvalid(t *testing.T) {
	_, err := New(new(mockRepo), fakeEventRules{}).UpdateAchievement(context.Background(), gameID, "first_win",
		domain.Achievement{Name: "  ", EventType: "match_won", Target: 1})
	assert.ErrorIs(t, err, domain.ErrInvalidAchievement)
}

func TestUpdateAchievement_Inexistente_PropagaNotFound(t *testing.T) {
	repo := new(mockRepo)
	repo.On("UpdateAchievement", mock.Anything, mock.Anything).Return(domain.ErrAchievementNotFound)
	_, err := New(repo, fakeEventRules{}).UpdateAchievement(context.Background(), gameID, "fantasma",
		domain.Achievement{Name: "X", EventType: "e", Target: 1})
	assert.ErrorIs(t, err, domain.ErrAchievementNotFound)
}

func TestDeleteAchievement_DelegaAoRepo(t *testing.T) {
	repo := new(mockRepo)
	repo.On("DeleteAchievement", mock.Anything, gameID, "first_win").Return(nil)
	err := New(repo, fakeEventRules{}).DeleteAchievement(context.Background(), gameID, "first_win")
	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestRecordEvent_AtingeMeta_RetornaDesbloqueada(t *testing.T) {
	repo := &mockRepo{}
	repo.On("FindByEventType", mock.Anything, gameID, "match_won").Return([]domain.Achievement{firstWin}, nil)
	repo.On("RecordProgress", mock.Anything, gameID, "player-1", "first_win", 1, mock.Anything).Return(true, nil)

	unlocked, err := New(repo, fakeEventRules{}).RecordEvent(context.Background(), gameID, "player-1", domain.Event{Type: "match_won"})

	require.NoError(t, err)
	require.Len(t, unlocked, 1)
	require.Equal(t, "first_win", unlocked[0].ID)
	repo.AssertExpectations(t)
}

func TestRecordEvent_ProgrediuSemDesbloquear_ListaVazia(t *testing.T) {
	repo := &mockRepo{}
	repo.On("FindByEventType", mock.Anything, gameID, "match_won").Return([]domain.Achievement{tenWins}, nil)
	repo.On("RecordProgress", mock.Anything, gameID, "player-1", "ten_wins", 10, mock.Anything).Return(false, nil)

	unlocked, err := New(repo, fakeEventRules{}).RecordEvent(context.Background(), gameID, "player-1", domain.Event{Type: "match_won"})

	require.NoError(t, err)
	require.Empty(t, unlocked)
	repo.AssertExpectations(t)
}

func TestRecordEvent_SemMatch_ListaVazia(t *testing.T) {
	repo := &mockRepo{}
	repo.On("FindByEventType", mock.Anything, gameID, "evento_solto").Return([]domain.Achievement{}, nil)

	unlocked, err := New(repo, fakeEventRules{}).RecordEvent(context.Background(), gameID, "player-1", domain.Event{Type: "evento_solto"})

	require.NoError(t, err)
	require.Empty(t, unlocked)
	repo.AssertNotCalled(t, "RecordProgress", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestRecordEvent_SemType_ErrInvalidEvent(t *testing.T) {
	repo := &mockRepo{}

	_, err := New(repo, fakeEventRules{}).RecordEvent(context.Background(), gameID, "player-1", domain.Event{Type: ""})

	require.ErrorIs(t, err, domain.ErrInvalidEvent)
	repo.AssertNotCalled(t, "FindByEventType", mock.Anything, mock.Anything, mock.Anything)
}

func TestListForPlayer_RefleteProgressoEUnlock(t *testing.T) {
	at := time.Now().UTC()
	repo := &mockRepo{}
	repo.On("ListCatalog", mock.Anything, gameID).Return([]domain.Achievement{firstWin, tenWins}, nil)
	repo.On("ListPlayerProgress", mock.Anything, gameID, "player-1").Return([]domain.PlayerProgress{
		{AchievementID: "first_win", Progress: 1, UnlockedAt: at}, // desbloqueada
		{AchievementID: "ten_wins", Progress: 7},                  // em progresso 7/10
	}, nil)

	list, err := New(repo, fakeEventRules{}).ListForPlayer(context.Background(), gameID, "player-1")

	require.NoError(t, err)
	require.Len(t, list, 2)
	require.True(t, list[0].Unlocked)
	require.Equal(t, 1, list[0].Progress)
	require.Equal(t, 1, list[0].Target)
	require.Equal(t, at, list[0].UnlockedAt)
	require.False(t, list[1].Unlocked)
	require.Equal(t, 7, list[1].Progress)
	require.Equal(t, 10, list[1].Target)
	require.True(t, list[1].UnlockedAt.IsZero())
}

func TestRecordEvent_ErroNoRepo_Propaga(t *testing.T) {
	repo := &mockRepo{}
	repo.On("FindByEventType", mock.Anything, gameID, "match_won").Return([]domain.Achievement{}, errors.New("db down"))

	_, err := New(repo, fakeEventRules{}).RecordEvent(context.Background(), gameID, "player-1", domain.Event{Type: "match_won"})

	require.Error(t, err)
}

func TestCreateAchievement_Valido_PersisteComGameID(t *testing.T) {
	repo := &mockRepo{}
	var captured *domain.Achievement
	repo.On("CreateAchievement", mock.Anything, mock.AnythingOfType("*domain.Achievement")).
		Return(nil).
		Run(func(args mock.Arguments) { captured = args.Get(1).(*domain.Achievement) })

	a, err := New(repo, fakeEventRules{}).CreateAchievement(context.Background(), gameID, domain.Achievement{
		ID: "first_blood", Name: "  Primeiro Sangue  ", EventType: "kill", Target: 1,
	})
	require.NoError(t, err)
	assert.Equal(t, gameID, a.GameID)
	assert.Equal(t, "first_blood", a.ID)
	assert.Equal(t, "Primeiro Sangue", a.Name, "name trimado")
	assert.Equal(t, gameID, captured.GameID)
}

func TestCreateAchievement_Invalido(t *testing.T) {
	repo := &mockRepo{}
	cases := []domain.Achievement{
		{ID: "X", Name: "n", EventType: "kill"},      // slug inválido (1 char/maiúsc)
		{ID: "ok-slug", Name: "", EventType: "kill"}, // name vazio
		{ID: "ok-slug", Name: "n", EventType: ""},    // event_type vazio
	}
	for _, c := range cases {
		_, err := New(repo, fakeEventRules{}).CreateAchievement(context.Background(), gameID, c)
		assert.ErrorIs(t, err, domain.ErrInvalidAchievement)
	}
	repo.AssertNotCalled(t, "CreateAchievement", mock.Anything, mock.Anything)
}

func TestCreateAchievement_StatValido_PersisteCriterio(t *testing.T) {
	repo := &mockRepo{}
	var captured *domain.Achievement
	repo.On("CreateAchievement", mock.Anything, mock.AnythingOfType("*domain.Achievement")).
		Return(nil).
		Run(func(args mock.Arguments) { captured = args.Get(1).(*domain.Achievement) })

	a, err := New(repo, fakeEventRules{}).CreateAchievement(context.Background(), gameID, domain.Achievement{
		ID: "sharpshooter", Name: "Franco-atirador", StatKey: "kills", StatMode: "cumulative", Target: 100,
	})
	require.NoError(t, err)
	assert.Equal(t, "kills", a.StatKey)
	assert.Equal(t, "cumulative", captured.StatMode)
	assert.Empty(t, captured.EventType)
}

func TestCreateAchievement_StatInvalido(t *testing.T) {
	repo := &mockRepo{}
	cases := map[string]domain.Achievement{
		"stat_key mau":          {ID: "ok-slug", Name: "n", StatKey: "Kills", StatMode: "cumulative"},
		"stat_mode mau":         {ID: "ok-slug", Name: "n", StatKey: "kills", StatMode: "total"},
		"stat + event juntos":   {ID: "ok-slug", Name: "n", StatKey: "kills", StatMode: "per_match", EventType: "kill"},
		"stat_mode sem statkey": {ID: "ok-slug", Name: "n", EventType: "kill", StatMode: "cumulative"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := New(repo, fakeEventRules{}).CreateAchievement(context.Background(), gameID, c)
			assert.ErrorIs(t, err, domain.ErrInvalidAchievement)
		})
	}
	repo.AssertNotCalled(t, "CreateAchievement", mock.Anything, mock.Anything)
}

// ApplyMatchResult repassa os stats crus no PlayerMatchInput (a aplicação por
// métrica é do repo; aqui validamos a passagem).
func TestApplyMatchResult_PropagaStats(t *testing.T) {
	repo := new(mockRepo)
	want := []domain.PlayerMatchInput{
		{PlayerID: "p1", EventTypes: []string{"match_played", "match_won"}, Stats: map[string]int64{"kills": 7}},
	}
	repo.On("ApplyMatchResult", mock.Anything, gameID, "m", want, mock.Anything).Return(false, nil)

	_, err := New(repo, fakeEventRules{}).ApplyMatchResult(context.Background(), gameID, "m", "",
		[]domain.MatchResult{{PlayerID: "p1", Outcome: "win", Stats: map[string]int64{"kills": 7}}})
	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestCreateAchievement_JaExiste_Propaga(t *testing.T) {
	repo := &mockRepo{}
	repo.On("CreateAchievement", mock.Anything, mock.Anything).Return(domain.ErrAchievementExists)

	_, err := New(repo, fakeEventRules{}).CreateAchievement(context.Background(), gameID, domain.Achievement{
		ID: "first_win", Name: "Primeira", EventType: "match_won", Target: 1,
	})
	assert.ErrorIs(t, err, domain.ErrAchievementExists)
}

// --- Stats como facts (spec 015) ---

// mvpRules estende o seed do coop com uma regra por stat: quem fizer 10+
// kills numa missão cumprida ganha mission_mvp (a avaliação é por jogador).
type mvpRules struct{ fakeEventRules }

func (m mvpRules) RulesForArchetype(ctx context.Context, gameID, archetype string) ([]domain.EventRule, error) {
	rules, _ := m.fakeEventRules.RulesForArchetype(ctx, gameID, archetype)
	if archetype == domain.ArchetypeCoopMission {
		rules = append(rules, domain.EventRule{
			Archetype: archetype,
			Match: map[string]domain.MatchCond{
				"outcome":    domain.Eq(domain.MissionSuccess),
				"stat.kills": {Op: domain.OpGte, Value: float64(10)},
			},
			EventTypes: []string{"mission_mvp"},
		})
	}
	return rules, nil
}

// Critério 2 da spec 015: regra com stat.* casa só pros jogadores cujo stat
// satisfaz — os demais seguem recebendo apenas os event types coletivos.
func TestApplyCollectiveResult_StatPorJogador_MVP(t *testing.T) {
	repo := new(mockRepo)
	svc := New(repo, mvpRules{})

	players := []domain.CollectivePlayer{
		{PlayerID: "mvp", Stats: map[string]int64{"kills": 12}},
		{PlayerID: "suporte", Stats: map[string]int64{"kills": 3, "revives": 5}},
		{PlayerID: "sem-stats"},
	}
	want := []domain.PlayerMatchInput{
		{PlayerID: "mvp", EventTypes: []string{"coop_mission_played", "coop_mission_success", "mission_mvp"}, Stats: map[string]int64{"kills": 12}},
		{PlayerID: "suporte", EventTypes: []string{"coop_mission_played", "coop_mission_success"}, Stats: map[string]int64{"kills": 3, "revives": 5}},
		{PlayerID: "sem-stats", EventTypes: []string{"coop_mission_played", "coop_mission_success"}, Stats: nil},
	}
	repo.On("ApplyMatchResult", mock.Anything, gameID, "m-mvp", want, mock.Anything).Return(false, nil)

	_, err := svc.ApplyCollectiveResult(context.Background(), gameID, "m-mvp", "", domain.MissionSuccess, players)
	require.NoError(t, err)
	repo.AssertExpectations(t)
}

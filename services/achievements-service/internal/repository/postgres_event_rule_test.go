//go:build integration

// Testes de integração com Postgres real (testcontainers). Rodar com:
//
//	go test -tags=integration ./services/achievements-service/internal/repository/
//
// Requer Docker em execução.
package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/achievements-service/internal/domain"
)

// A migration 008 semeia regras globais (game_id NULL) pros 4 arquétipos —
// RulesForArchetype devolve todas elas pra QUALQUER gameID (sem override
// específico ainda).
func TestRulesForArchetype_DevolveGlobaisDoSeed(t *testing.T) {
	repo := NewPostgresEventRuleRepository(newTestPool(t))
	ctx := context.Background()

	rules, err := repo.RulesForArchetype(ctx, sandbox, domain.ArchetypeHostWorldDropIn)
	require.NoError(t, err)
	require.Len(t, rules, 3, "3 regras globais semeadas pra host_world_dropin")
	for _, r := range rules {
		assert.Nil(t, r.GameID, "seed é global")
	}
}

// Um override específico do jogo aparece JUNTO das globais (união, não
// substituição) — RulesForArchetype filtra por game_id IS NULL OR = $2.
func TestRulesForArchetype_OverrideDoJogoSomaAsGlobais(t *testing.T) {
	repo := NewPostgresEventRuleRepository(newTestPool(t))
	ctx := context.Background()
	gameID := "game-evt-1"

	created := &domain.EventRule{
		GameID: &gameID, Archetype: domain.ArchetypeBattleRoyale,
		Match: domain.EqMatch(map[string]string{"placement": "2"}), EventTypes: []string{"custom_runner_up"},
	}
	require.NoError(t, repo.CreateRule(ctx, created))
	require.NotEmpty(t, created.ID, "CreateRule preenche o ID gerado")

	rules, err := repo.RulesForArchetype(ctx, gameID, domain.ArchetypeBattleRoyale)
	require.NoError(t, err)
	assert.Len(t, rules, 4, "3 globais do seed + 1 override do jogo")

	rulesOutroJogo, err := repo.RulesForArchetype(ctx, "outro-jogo", domain.ArchetypeBattleRoyale)
	require.NoError(t, err)
	assert.Len(t, rulesOutroJogo, 3, "override não vaza pra outro jogo")
}

// ListEffective devolve globais + overrides do jogo de TODOS os arquétipos
// (usado pelo admin — painel de Event Rules).
func TestListEffective_TodosArquetipos(t *testing.T) {
	repo := NewPostgresEventRuleRepository(newTestPool(t))
	ctx := context.Background()
	gameID := "game-evt-2"

	require.NoError(t, repo.CreateRule(ctx, &domain.EventRule{
		GameID: &gameID, Archetype: domain.ArchetypeCoopMission,
		Match: domain.EqMatch(map[string]string{"outcome": "MISSION_SUCCESS"}), EventTypes: []string{"custom_mission_medal"},
	}))

	all, err := repo.ListEffective(ctx, gameID)
	require.NoError(t, err)
	// 13 globais do seed (3+3+3+4) + 1 override deste jogo.
	assert.Len(t, all, 14)
}

// DeleteRule remove só a regra do jogo — nunca uma global (WHERE game_id =
// $2 nunca casa NULL); zero linhas -> ErrEventRuleNotFound.
func TestDeleteRule_SoRemoveDoJogo_GlobalDaErrEventRuleNotFound(t *testing.T) {
	repo := NewPostgresEventRuleRepository(newTestPool(t))
	ctx := context.Background()
	gameID := "game-evt-3"

	created := &domain.EventRule{
		GameID: &gameID, Archetype: domain.ArchetypeQueueArena,
		Match: domain.EqMatch(map[string]string{"outcome": "win"}), EventTypes: []string{"custom_win_badge"},
	}
	require.NoError(t, repo.CreateRule(ctx, created))

	require.NoError(t, repo.DeleteRule(ctx, gameID, created.ID))

	rulesAfter, err := repo.RulesForArchetype(ctx, gameID, domain.ArchetypeQueueArena)
	require.NoError(t, err)
	assert.Len(t, rulesAfter, 4, "só as globais do seed sobraram")

	globals, err := repo.RulesForArchetype(ctx, gameID, domain.ArchetypeQueueArena)
	require.NoError(t, err)
	require.NotEmpty(t, globals)
	err = repo.DeleteRule(ctx, gameID, globals[0].ID)
	assert.ErrorIs(t, err, domain.ErrEventRuleNotFound, "regra global não é deletável por este endpoint")

	err = repo.DeleteRule(ctx, gameID, "id-que-nao-existe")
	assert.ErrorIs(t, err, domain.ErrEventRuleNotFound)
}

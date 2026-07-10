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

// Suprimir uma regra global some com ela em RulesForArchetype SÓ pro jogo que
// suprimiu — o outro jogo do mesmo arquétipo continua recebendo (spec 017).
func TestSuppress_ExcluiDaAvaliacaoSoDoJogoQueSuprimiu(t *testing.T) {
	repo := NewPostgresEventRuleRepository(newTestPool(t))
	ctx := context.Background()
	gameA, gameB := "game-supp-a", "game-supp-b"

	globals, err := repo.RulesForArchetype(ctx, gameA, domain.ArchetypeBattleRoyale)
	require.NoError(t, err)
	require.Len(t, globals, 3, "3 globais do seed pra battle_royale")
	target := globals[0].ID

	require.NoError(t, repo.Suppress(ctx, gameA, target))

	afterA, err := repo.RulesForArchetype(ctx, gameA, domain.ArchetypeBattleRoyale)
	require.NoError(t, err)
	assert.Len(t, afterA, 2, "regra suprimida some da avaliação do jogo A")
	for _, r := range afterA {
		assert.NotEqual(t, target, r.ID)
	}

	afterB, err := repo.RulesForArchetype(ctx, gameB, domain.ArchetypeBattleRoyale)
	require.NoError(t, err)
	assert.Len(t, afterB, 3, "jogo B não suprimiu nada — continua recebendo as 3 globais")
}

// ListEffective reflete suppressed=true só pro jogo que suprimiu.
func TestSuppress_ListEffectiveMarcaSuppressedPorJogo(t *testing.T) {
	repo := NewPostgresEventRuleRepository(newTestPool(t))
	ctx := context.Background()
	gameA, gameB := "game-supp-c", "game-supp-d"

	globals, err := repo.RulesForArchetype(ctx, gameA, domain.ArchetypeCoopMission)
	require.NoError(t, err)
	target := globals[0].ID

	require.NoError(t, repo.Suppress(ctx, gameA, target))

	effA, err := repo.ListEffective(ctx, gameA)
	require.NoError(t, err)
	found := false
	for _, r := range effA {
		if r.ID == target {
			found = true
			assert.True(t, r.Suppressed)
		}
	}
	assert.True(t, found, "regra suprimida continua aparecendo em ListEffective (marcada)")

	effB, err := repo.ListEffective(ctx, gameB)
	require.NoError(t, err)
	for _, r := range effB {
		if r.ID == target {
			assert.False(t, r.Suppressed, "supressão do jogo A não vaza pro jogo B")
		}
	}
}

// Suprimir uma regra de JOGO (override, não global) é rejeitado — override já
// é removível via DeleteRule, suprimir não se aplica a ele.
func TestSuppress_RegraDeJogo_ErrEventRuleNotFound(t *testing.T) {
	repo := NewPostgresEventRuleRepository(newTestPool(t))
	ctx := context.Background()
	gameID := "game-supp-e"

	created := &domain.EventRule{
		GameID: &gameID, Archetype: domain.ArchetypeQueueArena,
		Match: domain.EqMatch(map[string]string{"outcome": "win"}), EventTypes: []string{"custom_win_badge"},
	}
	require.NoError(t, repo.CreateRule(ctx, created))

	err := repo.Suppress(ctx, gameID, created.ID)
	assert.ErrorIs(t, err, domain.ErrEventRuleNotFound, "override de jogo não é suprimível")
}

// Suprimir id inexistente também é ErrEventRuleNotFound.
func TestSuppress_IdInexistente_ErrEventRuleNotFound(t *testing.T) {
	repo := NewPostgresEventRuleRepository(newTestPool(t))
	ctx := context.Background()

	err := repo.Suppress(ctx, "game-supp-f", "id-que-nao-existe")
	assert.ErrorIs(t, err, domain.ErrEventRuleNotFound)
}

// Suprimir a mesma regra duas vezes é idempotente (sem erro).
func TestSuppress_Idempotente(t *testing.T) {
	repo := NewPostgresEventRuleRepository(newTestPool(t))
	ctx := context.Background()
	gameID := "game-supp-g"

	globals, err := repo.RulesForArchetype(ctx, gameID, domain.ArchetypeQueueArena)
	require.NoError(t, err)
	target := globals[0].ID

	require.NoError(t, repo.Suppress(ctx, gameID, target))
	require.NoError(t, repo.Suppress(ctx, gameID, target), "suprimir 2x é no-op, não erro")

	after, err := repo.RulesForArchetype(ctx, gameID, domain.ArchetypeQueueArena)
	require.NoError(t, err)
	assert.Len(t, after, 3, "continua excluída uma única vez, não duplicada")
}

// Unsuppress restaura a regra — volta a aparecer em RulesForArchetype.
func TestUnsuppress_Restaura(t *testing.T) {
	repo := NewPostgresEventRuleRepository(newTestPool(t))
	ctx := context.Background()
	gameID := "game-supp-h"

	globals, err := repo.RulesForArchetype(ctx, gameID, domain.ArchetypeCoopMission)
	require.NoError(t, err)
	target := globals[0].ID

	require.NoError(t, repo.Suppress(ctx, gameID, target))
	suppressed, err := repo.RulesForArchetype(ctx, gameID, domain.ArchetypeCoopMission)
	require.NoError(t, err)
	assert.Len(t, suppressed, 2)

	require.NoError(t, repo.Unsuppress(ctx, gameID, target))
	restored, err := repo.RulesForArchetype(ctx, gameID, domain.ArchetypeCoopMission)
	require.NoError(t, err)
	assert.Len(t, restored, 3, "regra volta a contribuir depois de restaurada")
}

// Unsuppress sem supressão ativa (nunca suprimida, ou id inexistente) é
// ErrEventRuleNotFound — mesma semântica de "não existe" do DeleteRule.
func TestUnsuppress_SemSupressaoAtiva_ErrEventRuleNotFound(t *testing.T) {
	repo := NewPostgresEventRuleRepository(newTestPool(t))
	ctx := context.Background()
	gameID := "game-supp-i"

	globals, err := repo.RulesForArchetype(ctx, gameID, domain.ArchetypeQueueArena)
	require.NoError(t, err)

	err = repo.Unsuppress(ctx, gameID, globals[0].ID)
	assert.ErrorIs(t, err, domain.ErrEventRuleNotFound, "nunca foi suprimida")

	err = repo.Unsuppress(ctx, gameID, "id-que-nao-existe")
	assert.ErrorIs(t, err, domain.ErrEventRuleNotFound)
}

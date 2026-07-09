package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// seedRules espelha o seed da migration 008 (event_rules), pra testar
// EvaluateRules com o MESMO comportamento que as funções hardcoded removidas
// (ParticipationEventTypes/PlacementEventTypes/CollectiveEventTypes) tinham.
func seedRules(archetype string) []EventRule {
	switch archetype {
	case ArchetypeHostWorldDropIn:
		return []EventRule{
			{Archetype: archetype, Match: map[string]string{"role": "invader", "event": "HOST_KILLED", "outcome": "SUCCESS"}, EventTypes: []string{EventInvasionWon}},
			{Archetype: archetype, Match: map[string]string{"role": "coop", "event": "BOSS_DEFEATED", "outcome": "SUCCESS"}, EventTypes: []string{EventCoopBossHelped}},
			{Archetype: archetype, Match: map[string]string{"event": "JOINED"}, EventTypes: []string{EventWorldJoined}},
		}
	case ArchetypeBattleRoyale:
		return []EventRule{
			{Archetype: archetype, Match: map[string]string{}, EventTypes: []string{EventBattleRoyalePlayed}},
			{Archetype: archetype, Match: map[string]string{"placement": "1"}, EventTypes: []string{EventBattleRoyaleWin}},
			{Archetype: archetype, Match: map[string]string{"tier": "top_n"}, EventTypes: []string{EventBattleRoyaleTopN}},
		}
	case ArchetypeCoopMission:
		return []EventRule{
			{Archetype: archetype, Match: map[string]string{}, EventTypes: []string{EventCoopMissionPlayed}},
			{Archetype: archetype, Match: map[string]string{"outcome": MissionSuccess}, EventTypes: []string{EventCoopMissionSuccess}},
			{Archetype: archetype, Match: map[string]string{"outcome": MissionFailure}, EventTypes: []string{EventCoopMissionFailed}},
		}
	case ArchetypeQueueArena:
		return []EventRule{
			{Archetype: archetype, Match: map[string]string{}, EventTypes: []string{"match_played"}},
			{Archetype: archetype, Match: map[string]string{"outcome": "win"}, EventTypes: []string{"match_won"}},
			{Archetype: archetype, Match: map[string]string{"outcome": "loss"}, EventTypes: []string{"match_lost"}},
			{Archetype: archetype, Match: map[string]string{"outcome": "draw"}, EventTypes: []string{"match_drawn"}},
		}
	}
	return nil
}

func TestEvaluateRules_HostWorldDropIn_Tabela(t *testing.T) {
	rules := seedRules(ArchetypeHostWorldDropIn)
	tests := []struct {
		name                  string
		role, event, outcome string
		want                  []string
	}{
		{"invasor mata host com sucesso", "invader", "HOST_KILLED", "SUCCESS", []string{EventInvasionWon}},
		{"invasor mata host sem sucesso não casa", "invader", "HOST_KILLED", "FAIL", nil},
		{"co-op ajuda no boss", "coop", "BOSS_DEFEATED", "SUCCESS", []string{EventCoopBossHelped}},
		{"co-op boss sem sucesso não casa", "coop", "BOSS_DEFEATED", "NEUTRAL", nil},
		{"joined de qualquer papel", "invader", "JOINED", "NEUTRAL", []string{EventWorldJoined}},
		{"joined de co-op", "coop", "JOINED", "NEUTRAL", []string{EventWorldJoined}},
		{"left não gera event type", "coop", "LEFT", "NEUTRAL", nil},
		{"died não gera event type", "invader", "DIED", "FAIL", nil},
		{"host_killed reportado por co-op não casa invasion_won", "coop", "HOST_KILLED", "SUCCESS", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateRules(rules, "game-1", ParticipationFacts(tt.role, tt.event, tt.outcome))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEvaluateRules_BattleRoyale_Tabela(t *testing.T) {
	rules := seedRules(ArchetypeBattleRoyale)
	tests := []struct {
		name      string
		placement int
		want      []string
	}{
		{"1º lugar acumula played/win/top_n", 1, []string{EventBattleRoyalePlayed, EventBattleRoyaleWin, EventBattleRoyaleTopN}},
		{"top 10 (não 1º) gera played/top_n", 7, []string{EventBattleRoyalePlayed, EventBattleRoyaleTopN}},
		{"limiar exato conta como top_n", 10, []string{EventBattleRoyalePlayed, EventBattleRoyaleTopN}},
		{"fora do top N só played", 50, []string{EventBattleRoyalePlayed}},
		{"logo abaixo do 1º ainda é top_n", 2, []string{EventBattleRoyalePlayed, EventBattleRoyaleTopN}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateRules(rules, "game-1", PlacementFacts(tt.placement))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEvaluateRules_CoopMission_Tabela(t *testing.T) {
	rules := seedRules(ArchetypeCoopMission)
	tests := []struct {
		name    string
		outcome string
		want    []string
	}{
		{"sucesso gera played/success", MissionSuccess, []string{EventCoopMissionPlayed, EventCoopMissionSuccess}},
		{"falha gera played/failed", MissionFailure, []string{EventCoopMissionPlayed, EventCoopMissionFailed}},
		{"desfecho desconhecido só played", "MISSION_ABORTED", []string{EventCoopMissionPlayed}},
		{"vazio só played", "", []string{EventCoopMissionPlayed}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateRules(rules, "game-1", CollectiveFacts(tt.outcome))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEvaluateRules_QueueArena_Tabela(t *testing.T) {
	rules := seedRules(ArchetypeQueueArena)
	tests := []struct {
		name    string
		outcome string
		want    []string
	}{
		{"win", "win", []string{"match_played", "match_won"}},
		{"loss", "loss", []string{"match_played", "match_lost"}},
		{"draw", "draw", []string{"match_played", "match_drawn"}},
		{"outcome desconhecido só played", "victory", []string{"match_played"}},
		{"vazio só played", "", []string{"match_played"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateRules(rules, "game-1", WinLossDrawFacts(tt.outcome))
			assert.Equal(t, tt.want, got)
		})
	}
}

// EvaluateRules deve unir (não substituir) as regras globais (GameID nil) com
// as específicas do jogo, sem duplicar event types repetidos entre as duas.
func TestEvaluateRules_GlobalMaisOverrideDoJogo_Uniao(t *testing.T) {
	gameA := "game-a"
	rules := []EventRule{
		{Archetype: "x", GameID: nil, Match: map[string]string{"event": "JOINED"}, EventTypes: []string{"world_joined"}},
		{Archetype: "x", GameID: &gameA, Match: map[string]string{"event": "JOINED"}, EventTypes: []string{"world_joined", "custom_joined_badge"}},
	}
	got := EvaluateRules(rules, gameA, Facts{"event": "JOINED"})
	assert.Equal(t, []string{"world_joined", "custom_joined_badge"}, got, "união sem duplicar world_joined")
}

// Uma regra específica de OUTRO jogo não deve vazar pro jogo atual.
func TestEvaluateRules_OverrideDeOutroJogo_NaoVaza(t *testing.T) {
	gameA, gameB := "game-a", "game-b"
	rules := []EventRule{
		{Archetype: "x", GameID: nil, Match: map[string]string{"event": "JOINED"}, EventTypes: []string{"world_joined"}},
		{Archetype: "x", GameID: &gameB, Match: map[string]string{"event": "JOINED"}, EventTypes: []string{"custom_only_b"}},
	}
	got := EvaluateRules(rules, gameA, Facts{"event": "JOINED"})
	assert.Equal(t, []string{"world_joined"}, got, "override do jogo B não aparece pro jogo A")
}

func TestMatchSubset_VazioSempreBate(t *testing.T) {
	assert.True(t, matchSubset(map[string]string{}, Facts{"a": "1", "b": "2"}))
	assert.True(t, matchSubset(nil, Facts{}))
}

func TestMatchSubset_ChaveAusenteNaoBate(t *testing.T) {
	assert.False(t, matchSubset(map[string]string{"a": "1"}, Facts{"b": "1"}))
}

// --- Fact `mode` (spec 013, F1 do ADR-0028) ---

// WithMode soma o fact `mode` quando o envelope traz modeId; vazio (envelope
// antigo/data plane sem ADR-0024) NÃO gera a chave — regra com mode nunca
// casa com vazio.
func TestFactsWithMode_VazioNaoGeraChave(t *testing.T) {
	f := WinLossDrawFacts("win").WithMode("")
	_, ok := f[FactMode]
	assert.False(t, ok, "modeID vazio não pode criar a chave mode")

	f = WinLossDrawFacts("win").WithMode("ranked")
	assert.Equal(t, "ranked", f[FactMode])
	assert.Equal(t, "win", f["outcome"], "facts do kind preservados")
}

// Regra do jogo com `mode` só casa quando o envelope traz aquele modo — a
// global (sem mode) segue casando nos dois casos (zero regressão).
func TestEvaluateRules_RegraPorModo(t *testing.T) {
	gameA := "game-a"
	rules := []EventRule{
		{Archetype: ArchetypeQueueArena, GameID: nil, Match: map[string]string{"outcome": "win"}, EventTypes: []string{"match_won"}},
		{Archetype: ArchetypeQueueArena, GameID: &gameA, Match: map[string]string{"outcome": "win", FactMode: "ranked"}, EventTypes: []string{"ranked_won"}},
	}

	got := EvaluateRules(rules, gameA, WinLossDrawFacts("win").WithMode("ranked"))
	assert.Equal(t, []string{"match_won", "ranked_won"}, got, "modo ranked: global + regra do modo")

	got = EvaluateRules(rules, gameA, WinLossDrawFacts("win").WithMode("casual"))
	assert.Equal(t, []string{"match_won"}, got, "outro modo: só a global")

	got = EvaluateRules(rules, gameA, WinLossDrawFacts("win").WithMode(""))
	assert.Equal(t, []string{"match_won"}, got, "envelope sem modeId: só a global")
}

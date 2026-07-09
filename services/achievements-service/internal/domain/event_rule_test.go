package domain

import (
	"encoding/json"
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
			{Archetype: archetype, Match: EqMatch(map[string]string{"role": "invader", "event": "HOST_KILLED", "outcome": "SUCCESS"}), EventTypes: []string{EventInvasionWon}},
			{Archetype: archetype, Match: EqMatch(map[string]string{"role": "coop", "event": "BOSS_DEFEATED", "outcome": "SUCCESS"}), EventTypes: []string{EventCoopBossHelped}},
			{Archetype: archetype, Match: EqMatch(map[string]string{"event": "JOINED"}), EventTypes: []string{EventWorldJoined}},
		}
	case ArchetypeBattleRoyale:
		return []EventRule{
			{Archetype: archetype, Match: EqMatch(map[string]string{}), EventTypes: []string{EventBattleRoyalePlayed}},
			{Archetype: archetype, Match: EqMatch(map[string]string{"placement": "1"}), EventTypes: []string{EventBattleRoyaleWin}},
			// migration 009 (spec 014): o top_n deixou de ser fact derivado e
			// virou comparador editável — placement <= 10.
			{Archetype: archetype, Match: map[string]MatchCond{"placement": {Op: OpLte, Value: float64(10)}}, EventTypes: []string{EventBattleRoyaleTopN}},
		}
	case ArchetypeCoopMission:
		return []EventRule{
			{Archetype: archetype, Match: EqMatch(map[string]string{}), EventTypes: []string{EventCoopMissionPlayed}},
			{Archetype: archetype, Match: EqMatch(map[string]string{"outcome": MissionSuccess}), EventTypes: []string{EventCoopMissionSuccess}},
			{Archetype: archetype, Match: EqMatch(map[string]string{"outcome": MissionFailure}), EventTypes: []string{EventCoopMissionFailed}},
		}
	case ArchetypeQueueArena:
		return []EventRule{
			{Archetype: archetype, Match: EqMatch(map[string]string{}), EventTypes: []string{"match_played"}},
			{Archetype: archetype, Match: EqMatch(map[string]string{"outcome": "win"}), EventTypes: []string{"match_won"}},
			{Archetype: archetype, Match: EqMatch(map[string]string{"outcome": "loss"}), EventTypes: []string{"match_lost"}},
			{Archetype: archetype, Match: EqMatch(map[string]string{"outcome": "draw"}), EventTypes: []string{"match_drawn"}},
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
		{Archetype: "x", GameID: nil, Match: EqMatch(map[string]string{"event": "JOINED"}), EventTypes: []string{"world_joined"}},
		{Archetype: "x", GameID: &gameA, Match: EqMatch(map[string]string{"event": "JOINED"}), EventTypes: []string{"world_joined", "custom_joined_badge"}},
	}
	got := EvaluateRules(rules, gameA, Facts{"event": "JOINED"})
	assert.Equal(t, []string{"world_joined", "custom_joined_badge"}, got, "união sem duplicar world_joined")
}

// Uma regra específica de OUTRO jogo não deve vazar pro jogo atual.
func TestEvaluateRules_OverrideDeOutroJogo_NaoVaza(t *testing.T) {
	gameA, gameB := "game-a", "game-b"
	rules := []EventRule{
		{Archetype: "x", GameID: nil, Match: EqMatch(map[string]string{"event": "JOINED"}), EventTypes: []string{"world_joined"}},
		{Archetype: "x", GameID: &gameB, Match: EqMatch(map[string]string{"event": "JOINED"}), EventTypes: []string{"custom_only_b"}},
	}
	got := EvaluateRules(rules, gameA, Facts{"event": "JOINED"})
	assert.Equal(t, []string{"world_joined"}, got, "override do jogo B não aparece pro jogo A")
}

func TestMatchSubset_VazioSempreBate(t *testing.T) {
	assert.True(t, matchSubset(map[string]MatchCond{}, Facts{"a": "1", "b": "2"}))
	assert.True(t, matchSubset(nil, Facts{}))
}

func TestMatchSubset_ChaveAusenteNaoBate(t *testing.T) {
	assert.False(t, matchSubset(map[string]MatchCond{"a": Eq("1")}, Facts{"b": "1"}))
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
		{Archetype: ArchetypeQueueArena, GameID: nil, Match: EqMatch(map[string]string{"outcome": "win"}), EventTypes: []string{"match_won"}},
		{Archetype: ArchetypeQueueArena, GameID: &gameA, Match: EqMatch(map[string]string{"outcome": "win", FactMode: "ranked"}), EventTypes: []string{"ranked_won"}},
	}

	got := EvaluateRules(rules, gameA, WinLossDrawFacts("win").WithMode("ranked"))
	assert.Equal(t, []string{"match_won", "ranked_won"}, got, "modo ranked: global + regra do modo")

	got = EvaluateRules(rules, gameA, WinLossDrawFacts("win").WithMode("casual"))
	assert.Equal(t, []string{"match_won"}, got, "outro modo: só a global")

	got = EvaluateRules(rules, gameA, WinLossDrawFacts("win").WithMode(""))
	assert.Equal(t, []string{"match_won"}, got, "envelope sem modeId: só a global")
}

// --- Comparadores no match (spec 014, V2a do ADR-0028) ---

// O JSON aceita as duas formas (string = eq; objeto = comparador) e o eq
// volta a serializar como string pura (compat total com o ADR-0027).
func TestMatchCond_JSONDuasFormas(t *testing.T) {
	var m map[string]MatchCond
	blob := []byte(`{"outcome":"win","placement":{"op":"lte","value":10},"mode":{"op":"in","value":["ranked","arena"]}}`)
	assert.NoError(t, json.Unmarshal(blob, &m))
	assert.Equal(t, Eq("win"), m["outcome"])
	assert.Equal(t, MatchCond{Op: OpLte, Value: float64(10)}, m["placement"])
	assert.Equal(t, MatchCond{Op: OpIn, Value: []string{"ranked", "arena"}}, m["mode"])

	out, err := json.Marshal(map[string]MatchCond{"outcome": Eq("win")})
	assert.NoError(t, err)
	assert.JSONEq(t, `{"outcome":"win"}`, string(out), "eq serializa como string pura")

	out, err = json.Marshal(map[string]MatchCond{"placement": {Op: OpLte, Value: float64(10)}})
	assert.NoError(t, err)
	assert.JSONEq(t, `{"placement":{"op":"lte","value":10}}`, string(out))
}

func TestCondMatches_Operadores(t *testing.T) {
	tests := []struct {
		name    string
		cond    MatchCond
		fact    string
		present bool
		want    bool
	}{
		{"eq casa", Eq("win"), "win", true, true},
		{"eq não casa", Eq("win"), "loss", true, false},
		{"ne casa", MatchCond{Op: OpNe, Value: "win"}, "loss", true, true},
		{"ne exige chave presente", MatchCond{Op: OpNe, Value: "win"}, "", false, false},
		{"lte no limiar", MatchCond{Op: OpLte, Value: float64(10)}, "10", true, true},
		{"lte acima não casa", MatchCond{Op: OpLte, Value: float64(10)}, "11", true, false},
		{"gt casa", MatchCond{Op: OpGt, Value: float64(5)}, "6", true, true},
		{"lt casa", MatchCond{Op: OpLt, Value: float64(5)}, "4", true, true},
		{"gte no limiar", MatchCond{Op: OpGte, Value: float64(5)}, "5", true, true},
		{"numérico com fact não-numérico não casa", MatchCond{Op: OpLte, Value: float64(10)}, "abc", true, false},
		{"in casa", MatchCond{Op: OpIn, Value: []string{"a", "b"}}, "b", true, true},
		{"in não casa", MatchCond{Op: OpIn, Value: []string{"a", "b"}}, "c", true, false},
		{"op desconhecido nunca casa", MatchCond{Op: "regex", Value: ".*"}, "x", true, false},
		{"value nil (decode leniente) nunca casa", MatchCond{Op: OpLte, Value: nil}, "1", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, condMatches(tt.cond, tt.fact, tt.present))
		})
	}
}

// Critério 2 da spec 014: o top_n como regra editável — placement 10 casa,
// 11 não, sem nenhum fact derivado.
func TestEvaluateRules_TopNViaComparador(t *testing.T) {
	rules := []EventRule{
		{Archetype: ArchetypeBattleRoyale, Match: map[string]MatchCond{"placement": {Op: OpLte, Value: float64(10)}}, EventTypes: []string{EventBattleRoyaleTopN}},
	}
	assert.Equal(t, []string{EventBattleRoyaleTopN}, EvaluateRules(rules, "g", PlacementFacts(10)))
	assert.Nil(t, EvaluateRules(rules, "g", PlacementFacts(11)))
}

func TestMatchCondValid(t *testing.T) {
	assert.True(t, Eq("x").Valid())
	assert.True(t, MatchCond{Op: OpIn, Value: []string{"a"}}.Valid())
	assert.False(t, MatchCond{Op: OpIn, Value: []string{}}.Valid(), "in vazio é inválido")
	assert.False(t, MatchCond{Op: "regex", Value: "x"}.Valid(), "op desconhecido é inválido")
	assert.False(t, MatchCond{Op: OpLte, Value: "10"}.Valid(), "numérico exige number, não string")
	assert.False(t, MatchCond{Op: OpLte, Value: nil}.Valid(), "value nil (decode leniente) é inválido")
}

// --- Stats como facts (spec 015) ---

func TestFactsWithStats_PrefixoEChaves(t *testing.T) {
	f := WinLossDrawFacts("win").WithStats(map[string]int64{"deaths": 0, "kills": 7})
	assert.Equal(t, "0", f["stat.deaths"])
	assert.Equal(t, "7", f["stat.kills"])
	assert.Equal(t, "win", f["outcome"], "facts estruturais preservados")

	f = WinLossDrawFacts("win").WithStats(nil)
	_, ok := f["stat.deaths"]
	assert.False(t, ok, "sem stats no envelope, nenhum fact stat.*")
}

// Critério 1 da spec 015: flawless victory — outcome=win E stat.deaths <= 0.
func TestEvaluateRules_FlawlessVictory(t *testing.T) {
	rules := []EventRule{
		{Archetype: ArchetypeQueueArena, Match: map[string]MatchCond{
			"outcome":     Eq("win"),
			"stat.deaths": {Op: OpLte, Value: float64(0)},
		}, EventTypes: []string{"flawless_victory"}},
	}

	got := EvaluateRules(rules, "g", WinLossDrawFacts("win").WithStats(map[string]int64{"deaths": 0}))
	assert.Equal(t, []string{"flawless_victory"}, got, "win sem mortes casa")

	got = EvaluateRules(rules, "g", WinLossDrawFacts("win").WithStats(map[string]int64{"deaths": 2}))
	assert.Nil(t, got, "win com mortes não casa")

	got = EvaluateRules(rules, "g", WinLossDrawFacts("loss").WithStats(map[string]int64{"deaths": 0}))
	assert.Nil(t, got, "loss sem mortes não casa")

	got = EvaluateRules(rules, "g", WinLossDrawFacts("win"))
	assert.Nil(t, got, "envelope sem a stat: fact ausente, não casa")
}

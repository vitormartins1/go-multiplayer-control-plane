package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/game-registry/internal/domain"
)

func validQA() domain.QueueArenaParams {
	return domain.QueueArenaParams{
		Structure: "teams", TeamCount: 2, TeamSize: 3,
		WinCondition: "first_to_score", ScoreTarget: 50,
		MatchStructure: "single", AllowDraw: true,
		ScoringStats: []string{"kills", "assists"},
	}
}

func validProfile() domain.MultiplayerProfile {
	qa := validQA()
	return domain.MultiplayerProfile{
		Archetype: domain.ArchetypeQueueArena, Topology: domain.TopologyDedicated, QueueArena: &qa,
	}
}

func mut(base domain.QueueArenaParams, f func(*domain.QueueArenaParams)) domain.QueueArenaParams {
	f(&base)
	return base
}

func TestSetMultiplayerProfile_Valido_UpsertEDevolve(t *testing.T) {
	repo := new(mockRepo)
	repo.On("UpsertMultiplayerProfile", mock.Anything, "g1", mock.Anything).Return(nil)

	p, err := New(repo).SetMultiplayerProfile(context.Background(), "g1", validProfile())
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, domain.ArchetypeQueueArena, p.Archetype)
	repo.AssertExpectations(t)
}

func TestSetMultiplayerProfile_ArchetypeNaoSuportado_Erro(t *testing.T) {
	p := validProfile()
	p.Archetype = "async_turns" // arquétipo fora do conjunto fechado (ADR-0023)
	_, err := New(new(mockRepo)).SetMultiplayerProfile(context.Background(), "g1", p)
	assert.ErrorIs(t, err, domain.ErrUnsupportedArchetype)
}

func TestSetMultiplayerProfile_TopologiaNaoSuportada_Erro(t *testing.T) {
	p := validProfile()
	p.Topology = "p2p"
	_, err := New(new(mockRepo)).SetMultiplayerProfile(context.Background(), "g1", p)
	assert.ErrorIs(t, err, domain.ErrUnsupportedTopology)
}

func TestSetMultiplayerProfile_ParamsAusentes_ErrInvalidProfileParams(t *testing.T) {
	p := validProfile()
	p.QueueArena = nil
	_, err := New(new(mockRepo)).SetMultiplayerProfile(context.Background(), "g1", p)
	assert.ErrorIs(t, err, domain.ErrInvalidProfileParams)
}

func TestSetMultiplayerProfile_ParamsInvalidos_ErrInvalidProfileParams(t *testing.T) {
	base := validQA()
	cases := map[string]domain.QueueArenaParams{
		"structure desconhecido":         mut(base, func(q *domain.QueueArenaParams) { q.Structure = "coop" }),
		"teams com teamCount<2":          mut(base, func(q *domain.QueueArenaParams) { q.TeamCount = 1 }),
		"teams com teamSize<1":           mut(base, func(q *domain.QueueArenaParams) { q.TeamSize = 0 }),
		"teams com playerCount!=0":       mut(base, func(q *domain.QueueArenaParams) { q.PlayerCount = 4 }),
		"ffa com playerCount<2":          {Structure: "ffa", PlayerCount: 1, WinCondition: "objective", MatchStructure: "single"},
		"ffa com teamCount preenchido":   {Structure: "ffa", PlayerCount: 4, TeamCount: 2, WinCondition: "objective", MatchStructure: "single"},
		"winCondition desconhecida":      mut(base, func(q *domain.QueueArenaParams) { q.WinCondition = "domination_x" }),
		"first_to_score sem scoreTarget": mut(base, func(q *domain.QueueArenaParams) { q.ScoreTarget = 0 }),
		"highest_at_time sem duration":   {Structure: "ffa", PlayerCount: 8, WinCondition: "highest_at_time", MatchStructure: "single"},
		"matchStructure desconhecido":    mut(base, func(q *domain.QueueArenaParams) { q.MatchStructure = "ladder" }),
		"single com roundsToWin!=0":      mut(base, func(q *domain.QueueArenaParams) { q.RoundsToWin = 2 }),
		"best_of_rounds sem roundsToWin": mut(base, func(q *domain.QueueArenaParams) { q.MatchStructure = "best_of_rounds"; q.RoundsToWin = 0 }),
		"scoringStats invalido":          mut(base, func(q *domain.QueueArenaParams) { q.ScoringStats = []string{"Kills"} }),
	}
	for name, qa := range cases {
		t.Run(name, func(t *testing.T) {
			qa := qa
			p := domain.MultiplayerProfile{Archetype: domain.ArchetypeQueueArena, Topology: domain.TopologyDedicated, QueueArena: &qa}
			_, err := New(new(mockRepo)).SetMultiplayerProfile(context.Background(), "g1", p)
			assert.ErrorIs(t, err, domain.ErrInvalidProfileParams)
		})
	}
}

func TestSetMultiplayerProfile_VariantesValidas_Ok(t *testing.T) {
	cases := map[string]domain.QueueArenaParams{
		"ffa first_to_score":   {Structure: "ffa", PlayerCount: 8, WinCondition: "first_to_score", ScoreTarget: 25, MatchStructure: "single"},
		"teams best_of_rounds": {Structure: "teams", TeamCount: 2, TeamSize: 5, WinCondition: "objective", MatchStructure: "best_of_rounds", RoundsToWin: 3},
		"elimination sem alvo": {Structure: "ffa", PlayerCount: 2, WinCondition: "elimination", MatchStructure: "single"},
		"scoringStats vazio":   {Structure: "ffa", PlayerCount: 2, WinCondition: "objective", MatchStructure: "single", ScoringStats: nil},
	}
	for name, qa := range cases {
		t.Run(name, func(t *testing.T) {
			qa := qa
			repo := new(mockRepo)
			repo.On("UpsertMultiplayerProfile", mock.Anything, "g1", mock.Anything).Return(nil)
			p := domain.MultiplayerProfile{Archetype: domain.ArchetypeQueueArena, Topology: domain.TopologyDedicated, QueueArena: &qa}
			_, err := New(repo).SetMultiplayerProfile(context.Background(), "g1", p)
			assert.NoError(t, err)
		})
	}
}

func TestSetMultiplayerProfile_JogoInexistente_ErrGameNotFound(t *testing.T) {
	repo := new(mockRepo)
	repo.On("UpsertMultiplayerProfile", mock.Anything, "nao", mock.Anything).Return(domain.ErrGameNotFound)
	_, err := New(repo).SetMultiplayerProfile(context.Background(), "nao", validProfile())
	assert.ErrorIs(t, err, domain.ErrGameNotFound)
}

func TestGetMultiplayerProfile_Existe_Devolve(t *testing.T) {
	p := validProfile()
	repo := new(mockRepo)
	repo.On("FindByID", mock.Anything, "g1").Return(&domain.Game{ID: "g1", MultiplayerProfile: &p}, nil)

	got, err := New(repo).GetMultiplayerProfile(context.Background(), "g1")
	require.NoError(t, err)
	assert.Equal(t, domain.ArchetypeQueueArena, got.Archetype)
}

func TestGetMultiplayerProfile_SemPerfil_ErrProfileNotFound(t *testing.T) {
	repo := new(mockRepo)
	repo.On("FindByID", mock.Anything, "g1").Return(&domain.Game{ID: "g1"}, nil)
	_, err := New(repo).GetMultiplayerProfile(context.Background(), "g1")
	assert.ErrorIs(t, err, domain.ErrProfileNotFound)
}

func TestGetMultiplayerProfile_JogoInexistente_ErrGameNotFound(t *testing.T) {
	repo := new(mockRepo)
	repo.On("FindByID", mock.Anything, "nao").Return(nil, domain.ErrGameNotFound)
	_, err := New(repo).GetMultiplayerProfile(context.Background(), "nao")
	assert.ErrorIs(t, err, domain.ErrGameNotFound)
}

// --- host_world_dropin ---

func validHWD() domain.HostWorldDropInParams {
	return domain.HostWorldDropInParams{
		EnabledRoles:      []string{"coop", "invader"},
		CoopSlots:         2,
		InvaderSlots:      1,
		LevelRangeEnabled: true,
		PasswordEnabled:   true,
		InvasionEnabled:   true,
	}
}

func validHWDProfile() domain.MultiplayerProfile {
	hw := validHWD()
	return domain.MultiplayerProfile{
		Archetype:       domain.ArchetypeHostWorldDropIn,
		Topology:        domain.TopologyDedicated,
		HostWorldDropIn: &hw,
	}
}

func mutHWD(base domain.HostWorldDropInParams, f func(*domain.HostWorldDropInParams)) domain.HostWorldDropInParams {
	f(&base)
	return base
}

func TestSetMultiplayerProfile_HostWorldDropIn_Valido_UpsertEDevolve(t *testing.T) {
	repo := new(mockRepo)
	repo.On("UpsertMultiplayerProfile", mock.Anything, "g1", mock.Anything).Return(nil)

	p, err := New(repo).SetMultiplayerProfile(context.Background(), "g1", validHWDProfile())
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, domain.ArchetypeHostWorldDropIn, p.Archetype)
	repo.AssertExpectations(t)
}

func TestSetMultiplayerProfile_HostWorldDropIn_ParamsAusentes_Erro(t *testing.T) {
	p := validHWDProfile()
	p.HostWorldDropIn = nil
	_, err := New(new(mockRepo)).SetMultiplayerProfile(context.Background(), "g1", p)
	assert.ErrorIs(t, err, domain.ErrInvalidProfileParams)
}

func TestSetMultiplayerProfile_HostWorldDropIn_ParamsInvalidos_Erro(t *testing.T) {
	base := validHWD()
	cases := map[string]domain.HostWorldDropInParams{
		"roles vazio":                    mutHWD(base, func(h *domain.HostWorldDropInParams) { h.EnabledRoles = nil }),
		"role desconhecido":              mutHWD(base, func(h *domain.HostWorldDropInParams) { h.EnabledRoles = []string{"host"} }),
		"role duplicado":                 mutHWD(base, func(h *domain.HostWorldDropInParams) { h.EnabledRoles = []string{"coop", "coop"} }),
		"coop sem coopSlots":             mutHWD(base, func(h *domain.HostWorldDropInParams) { h.CoopSlots = 0 }),
		"coopSlots sem papel coop":       {EnabledRoles: []string{"invader"}, InvaderSlots: 1, CoopSlots: 2},
		"invader sem invaderSlots":       mutHWD(base, func(h *domain.HostWorldDropInParams) { h.InvaderSlots = 0 }),
		"invaderSlots sem papel invader": {EnabledRoles: []string{"coop"}, CoopSlots: 2, InvaderSlots: 1},
	}
	for name, hw := range cases {
		t.Run(name, func(t *testing.T) {
			hw := hw
			p := domain.MultiplayerProfile{
				Archetype:       domain.ArchetypeHostWorldDropIn,
				Topology:        domain.TopologyDedicated,
				HostWorldDropIn: &hw,
			}
			_, err := New(new(mockRepo)).SetMultiplayerProfile(context.Background(), "g1", p)
			assert.ErrorIs(t, err, domain.ErrInvalidProfileParams, name)
		})
	}
}

func TestSetMultiplayerProfile_HostWorldDropIn_VariantesValidas_Ok(t *testing.T) {
	cases := map[string]domain.HostWorldDropInParams{
		"só co-op":         {EnabledRoles: []string{"coop"}, CoopSlots: 3},
		"só invasão":       {EnabledRoles: []string{"invader"}, InvaderSlots: 2, InvasionEnabled: true},
		"coop+invader":     {EnabledRoles: []string{"coop", "invader"}, CoopSlots: 2, InvaderSlots: 1},
		"todos os filtros": {EnabledRoles: []string{"coop", "invader"}, CoopSlots: 2, InvaderSlots: 1, LevelRangeEnabled: true, WeaponTierEnabled: true, PasswordEnabled: true, InvasionEnabled: true},
	}
	for name, hw := range cases {
		t.Run(name, func(t *testing.T) {
			hw := hw
			repo := new(mockRepo)
			repo.On("UpsertMultiplayerProfile", mock.Anything, "g1", mock.Anything).Return(nil)
			p := domain.MultiplayerProfile{
				Archetype:       domain.ArchetypeHostWorldDropIn,
				Topology:        domain.TopologyDedicated,
				HostWorldDropIn: &hw,
			}
			_, err := New(repo).SetMultiplayerProfile(context.Background(), "g1", p)
			assert.NoError(t, err, name)
		})
	}
}

// --- battle_royale ---

func validBR() domain.BattleRoyaleParams {
	return domain.BattleRoyaleParams{
		LobbySize:    60,
		TeamSize:     4,
		RespawnMode:  domain.RespawnOneLife,
		ScoringStats: []string{"kills"},
	}
}

func validBRProfile() domain.MultiplayerProfile {
	br := validBR()
	return domain.MultiplayerProfile{
		Archetype:    domain.ArchetypeBattleRoyale,
		Topology:     domain.TopologyDedicated,
		BattleRoyale: &br,
	}
}

func mutBR(base domain.BattleRoyaleParams, f func(*domain.BattleRoyaleParams)) domain.BattleRoyaleParams {
	f(&base)
	return base
}

func TestSetMultiplayerProfile_BattleRoyale_Valido_UpsertEDevolve(t *testing.T) {
	repo := new(mockRepo)
	repo.On("UpsertMultiplayerProfile", mock.Anything, "g1", mock.Anything).Return(nil)

	p, err := New(repo).SetMultiplayerProfile(context.Background(), "g1", validBRProfile())
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, domain.ArchetypeBattleRoyale, p.Archetype)
	repo.AssertExpectations(t)
}

func TestSetMultiplayerProfile_BattleRoyale_ParamsAusentes_Erro(t *testing.T) {
	p := validBRProfile()
	p.BattleRoyale = nil
	_, err := New(new(mockRepo)).SetMultiplayerProfile(context.Background(), "g1", p)
	assert.ErrorIs(t, err, domain.ErrInvalidProfileParams)
}

func TestSetMultiplayerProfile_BattleRoyale_ParamsInvalidos_Erro(t *testing.T) {
	base := validBR()
	cases := map[string]domain.BattleRoyaleParams{
		"teamSize<1":               mutBR(base, func(b *domain.BattleRoyaleParams) { b.TeamSize = 0 }),
		"lobbySize<2":              mutBR(base, func(b *domain.BattleRoyaleParams) { b.LobbySize = 1; b.TeamSize = 1 }),
		"lobbySize<2*teamSize":     mutBR(base, func(b *domain.BattleRoyaleParams) { b.LobbySize = 6; b.TeamSize = 4 }),
		"respawnMode desconhecido": mutBR(base, func(b *domain.BattleRoyaleParams) { b.RespawnMode = "phoenix" }),
		"respawnMode vazio":        mutBR(base, func(b *domain.BattleRoyaleParams) { b.RespawnMode = "" }),
		"scoringStats invalido":    mutBR(base, func(b *domain.BattleRoyaleParams) { b.ScoringStats = []string{"Kills"} }),
	}
	for name, br := range cases {
		t.Run(name, func(t *testing.T) {
			br := br
			p := domain.MultiplayerProfile{
				Archetype:    domain.ArchetypeBattleRoyale,
				Topology:     domain.TopologyDedicated,
				BattleRoyale: &br,
			}
			_, err := New(new(mockRepo)).SetMultiplayerProfile(context.Background(), "g1", p)
			assert.ErrorIs(t, err, domain.ErrInvalidProfileParams, name)
		})
	}
}

func TestSetMultiplayerProfile_BattleRoyale_VariantesValidas_Ok(t *testing.T) {
	cases := map[string]domain.BattleRoyaleParams{
		"solo one_life":         {LobbySize: 100, TeamSize: 1, RespawnMode: domain.RespawnOneLife},
		"duo teammate_revive":   {LobbySize: 50, TeamSize: 2, RespawnMode: domain.RespawnTeammateRevive, ScoringStats: []string{"kills", "revives"}},
		"squad arena_1v1":       {LobbySize: 60, TeamSize: 4, RespawnMode: domain.RespawnArena1v1},
		"squad reboot_timer":    {LobbySize: 8, TeamSize: 4, RespawnMode: domain.RespawnRebootTimer},
		"lobby não múltiplo ok": {LobbySize: 30, TeamSize: 4, RespawnMode: domain.RespawnOneLife},
		"scoringStats vazio":    {LobbySize: 20, TeamSize: 1, RespawnMode: domain.RespawnOneLife, ScoringStats: nil},
	}
	for name, br := range cases {
		t.Run(name, func(t *testing.T) {
			br := br
			repo := new(mockRepo)
			repo.On("UpsertMultiplayerProfile", mock.Anything, "g1", mock.Anything).Return(nil)
			p := domain.MultiplayerProfile{
				Archetype:    domain.ArchetypeBattleRoyale,
				Topology:     domain.TopologyDedicated,
				BattleRoyale: &br,
			}
			_, err := New(repo).SetMultiplayerProfile(context.Background(), "g1", p)
			assert.NoError(t, err, name)
		})
	}
}

// --- coop_mission ---

func validCM() domain.CoopMissionParams {
	return domain.CoopMissionParams{
		TeamSize:       4,
		Missions:       []string{"salvage_op", "escort"},
		Difficulties:   []string{"hazard_1", "hazard_5"},
		BotfillEnabled: true,
		ScoringStats:   []string{"kills", "revives"},
	}
}

func validCMProfile() domain.MultiplayerProfile {
	cm := validCM()
	return domain.MultiplayerProfile{
		Archetype:   domain.ArchetypeCoopMission,
		Topology:    domain.TopologyDedicated,
		CoopMission: &cm,
	}
}

func mutCM(base domain.CoopMissionParams, f func(*domain.CoopMissionParams)) domain.CoopMissionParams {
	f(&base)
	return base
}

func TestSetMultiplayerProfile_CoopMission_Valido_UpsertEDevolve(t *testing.T) {
	repo := new(mockRepo)
	repo.On("UpsertMultiplayerProfile", mock.Anything, "g1", mock.Anything).Return(nil)

	p, err := New(repo).SetMultiplayerProfile(context.Background(), "g1", validCMProfile())
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, domain.ArchetypeCoopMission, p.Archetype)
	repo.AssertExpectations(t)
}

func TestSetMultiplayerProfile_CoopMission_ParamsAusentes_Erro(t *testing.T) {
	p := validCMProfile()
	p.CoopMission = nil
	_, err := New(new(mockRepo)).SetMultiplayerProfile(context.Background(), "g1", p)
	assert.ErrorIs(t, err, domain.ErrInvalidProfileParams)
}

func TestSetMultiplayerProfile_CoopMission_ParamsInvalidos_Erro(t *testing.T) {
	base := validCM()
	cases := map[string]domain.CoopMissionParams{
		"teamSize<2":            mutCM(base, func(c *domain.CoopMissionParams) { c.TeamSize = 1 }),
		"missions invalido":     mutCM(base, func(c *domain.CoopMissionParams) { c.Missions = []string{"Salvage"} }),
		"missions duplicado":    mutCM(base, func(c *domain.CoopMissionParams) { c.Missions = []string{"escort", "escort"} }),
		"difficulties invalido": mutCM(base, func(c *domain.CoopMissionParams) { c.Difficulties = []string{"Hazard 5"} }),
		"difficulties duplicado": mutCM(base, func(c *domain.CoopMissionParams) {
			c.Difficulties = []string{"hazard_1", "hazard_1"}
		}),
		"scoringStats invalido": mutCM(base, func(c *domain.CoopMissionParams) { c.ScoringStats = []string{"Kills"} }),
	}
	for name, cm := range cases {
		t.Run(name, func(t *testing.T) {
			cm := cm
			p := domain.MultiplayerProfile{
				Archetype:   domain.ArchetypeCoopMission,
				Topology:    domain.TopologyDedicated,
				CoopMission: &cm,
			}
			_, err := New(new(mockRepo)).SetMultiplayerProfile(context.Background(), "g1", p)
			assert.ErrorIs(t, err, domain.ErrInvalidProfileParams, name)
		})
	}
}

func TestSetMultiplayerProfile_CoopMission_VariantesValidas_Ok(t *testing.T) {
	cases := map[string]domain.CoopMissionParams{
		"mínimo (só teamSize)":  {TeamSize: 2},
		"catálogos vazios":      {TeamSize: 4, Missions: nil, Difficulties: nil, ScoringStats: nil},
		"botfill on":            {TeamSize: 4, BotfillEnabled: true, Missions: []string{"escort"}},
		"scoringStats múltiplo": {TeamSize: 3, ScoringStats: []string{"kills", "revives", "ore_mined"}},
	}
	for name, cm := range cases {
		t.Run(name, func(t *testing.T) {
			cm := cm
			repo := new(mockRepo)
			repo.On("UpsertMultiplayerProfile", mock.Anything, "g1", mock.Anything).Return(nil)
			p := domain.MultiplayerProfile{
				Archetype:   domain.ArchetypeCoopMission,
				Topology:    domain.TopologyDedicated,
				CoopMission: &cm,
			}
			_, err := New(repo).SetMultiplayerProfile(context.Background(), "g1", p)
			assert.NoError(t, err, name)
		})
	}
}

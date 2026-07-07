package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vitormartins1/go-game-backend/services/game-registry/internal/domain"
)

// Persistência do perfil de multiplayer (ADR-0023, spec 006): coluna JSONB
// multiplayer_profile na tabela games. O formato armazenado é
// { archetype, topology, params }, onde params é o objeto do arquétipo — o que
// mantém archetype/topology no topo e acomoda params polimórficos por arquétipo.

// storedProfile é o envelope persistido; params fica cru (RawMessage) para ser
// decodificado conforme o archetype.
type storedProfile struct {
	Archetype string          `json:"archetype"`
	Topology  string          `json:"topology"`
	Params    json.RawMessage `json:"params"`
}

// storedQueueArena é o shape persistido dos params do arquétipo queue_arena.
type storedQueueArena struct {
	Structure       string   `json:"structure"`
	PlayerCount     int      `json:"playerCount,omitempty"`
	TeamCount       int      `json:"teamCount,omitempty"`
	TeamSize        int      `json:"teamSize,omitempty"`
	WinCondition    string   `json:"winCondition"`
	ScoreTarget     int      `json:"scoreTarget,omitempty"`
	DurationSeconds int      `json:"durationSeconds,omitempty"`
	MatchStructure  string   `json:"matchStructure"`
	RoundsToWin     int      `json:"roundsToWin,omitempty"`
	AllowDraw       bool     `json:"allowDraw"`
	ScoringStats    []string `json:"scoringStats,omitempty"`
}

// storedBattleRoyale é o shape persistido dos params do arquétipo battle_royale.
type storedBattleRoyale struct {
	LobbySize    int      `json:"lobbySize,omitempty"`
	TeamSize     int      `json:"teamSize,omitempty"`
	RespawnMode  string   `json:"respawnMode"`
	ScoringStats []string `json:"scoringStats,omitempty"`
}

// storedCoopMission é o shape persistido dos params do arquétipo coop_mission.
type storedCoopMission struct {
	TeamSize       int      `json:"teamSize,omitempty"`
	Missions       []string `json:"missions,omitempty"`
	Difficulties   []string `json:"difficulties,omitempty"`
	BotfillEnabled bool     `json:"botfillEnabled"`
	ScoringStats   []string `json:"scoringStats,omitempty"`
}

// storedHostWorldDropIn é o shape persistido dos params do arquétipo host_world_dropin.
type storedHostWorldDropIn struct {
	EnabledRoles      []string `json:"enabledRoles"`
	CoopSlots         int      `json:"coopSlots,omitempty"`
	InvaderSlots      int      `json:"invaderSlots,omitempty"`
	LevelRangeEnabled bool     `json:"levelRangeEnabled"`
	WeaponTierEnabled bool     `json:"weaponTierEnabled"`
	PasswordEnabled   bool     `json:"passwordEnabled"`
	InvasionEnabled   bool     `json:"invasionEnabled"`
}

func (r *PostgresGameRepository) UpsertMultiplayerProfile(ctx context.Context, gameID string, p domain.MultiplayerProfile) error {
	raw, err := encodeProfile(&p)
	if err != nil {
		return fmt.Errorf("game-registry repository: upsert profile: encode: %w", err)
	}
	tag, err := r.pool.Exec(ctx, `UPDATE games SET multiplayer_profile = $2 WHERE id = $1`, gameID, raw)
	if err != nil {
		return fmt.Errorf("game-registry repository: upsert profile: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrGameNotFound
	}
	return nil
}

// encodeProfile serializa o perfil de domínio para o JSONB armazenado.
func encodeProfile(p *domain.MultiplayerProfile) ([]byte, error) {
	sp := storedProfile{Archetype: p.Archetype, Topology: p.Topology}
	switch p.Archetype {
	case domain.ArchetypeQueueArena:
		if p.QueueArena == nil {
			return nil, fmt.Errorf("perfil %q sem params", p.Archetype)
		}
		qa := p.QueueArena
		params, err := json.Marshal(storedQueueArena{
			Structure:       qa.Structure,
			PlayerCount:     qa.PlayerCount,
			TeamCount:       qa.TeamCount,
			TeamSize:        qa.TeamSize,
			WinCondition:    qa.WinCondition,
			ScoreTarget:     qa.ScoreTarget,
			DurationSeconds: qa.DurationSeconds,
			MatchStructure:  qa.MatchStructure,
			RoundsToWin:     qa.RoundsToWin,
			AllowDraw:       qa.AllowDraw,
			ScoringStats:    qa.ScoringStats,
		})
		if err != nil {
			return nil, err
		}
		sp.Params = params
	case domain.ArchetypeHostWorldDropIn:
		if p.HostWorldDropIn == nil {
			return nil, fmt.Errorf("perfil %q sem params", p.Archetype)
		}
		hw := p.HostWorldDropIn
		params, err := json.Marshal(storedHostWorldDropIn{
			EnabledRoles:      hw.EnabledRoles,
			CoopSlots:         hw.CoopSlots,
			InvaderSlots:      hw.InvaderSlots,
			LevelRangeEnabled: hw.LevelRangeEnabled,
			WeaponTierEnabled: hw.WeaponTierEnabled,
			PasswordEnabled:   hw.PasswordEnabled,
			InvasionEnabled:   hw.InvasionEnabled,
		})
		if err != nil {
			return nil, err
		}
		sp.Params = params
	case domain.ArchetypeBattleRoyale:
		if p.BattleRoyale == nil {
			return nil, fmt.Errorf("perfil %q sem params", p.Archetype)
		}
		br := p.BattleRoyale
		params, err := json.Marshal(storedBattleRoyale{
			LobbySize:    br.LobbySize,
			TeamSize:     br.TeamSize,
			RespawnMode:  br.RespawnMode,
			ScoringStats: br.ScoringStats,
		})
		if err != nil {
			return nil, err
		}
		sp.Params = params
	case domain.ArchetypeCoopMission:
		if p.CoopMission == nil {
			return nil, fmt.Errorf("perfil %q sem params", p.Archetype)
		}
		cm := p.CoopMission
		params, err := json.Marshal(storedCoopMission{
			TeamSize:       cm.TeamSize,
			Missions:       cm.Missions,
			Difficulties:   cm.Difficulties,
			BotfillEnabled: cm.BotfillEnabled,
			ScoringStats:   cm.ScoringStats,
		})
		if err != nil {
			return nil, err
		}
		sp.Params = params
	default:
		return nil, fmt.Errorf("arquétipo não persistível: %q", p.Archetype)
	}
	return json.Marshal(sp)
}

// decodeProfile desserializa o JSONB armazenado para o perfil de domínio.
// Coluna NULL/vazia -> nil (jogo sem perfil).
func decodeProfile(raw []byte) (*domain.MultiplayerProfile, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var sp storedProfile
	if err := json.Unmarshal(raw, &sp); err != nil {
		return nil, err
	}
	p := &domain.MultiplayerProfile{Archetype: sp.Archetype, Topology: sp.Topology}
	switch sp.Archetype {
	case domain.ArchetypeQueueArena:
		var sq storedQueueArena
		if len(sp.Params) > 0 {
			if err := json.Unmarshal(sp.Params, &sq); err != nil {
				return nil, err
			}
		}
		p.QueueArena = &domain.QueueArenaParams{
			Structure:       sq.Structure,
			PlayerCount:     sq.PlayerCount,
			TeamCount:       sq.TeamCount,
			TeamSize:        sq.TeamSize,
			WinCondition:    sq.WinCondition,
			ScoreTarget:     sq.ScoreTarget,
			DurationSeconds: sq.DurationSeconds,
			MatchStructure:  sq.MatchStructure,
			RoundsToWin:     sq.RoundsToWin,
			AllowDraw:       sq.AllowDraw,
			ScoringStats:    sq.ScoringStats,
		}
	case domain.ArchetypeHostWorldDropIn:
		var sh storedHostWorldDropIn
		if len(sp.Params) > 0 {
			if err := json.Unmarshal(sp.Params, &sh); err != nil {
				return nil, err
			}
		}
		p.HostWorldDropIn = &domain.HostWorldDropInParams{
			EnabledRoles:      sh.EnabledRoles,
			CoopSlots:         sh.CoopSlots,
			InvaderSlots:      sh.InvaderSlots,
			LevelRangeEnabled: sh.LevelRangeEnabled,
			WeaponTierEnabled: sh.WeaponTierEnabled,
			PasswordEnabled:   sh.PasswordEnabled,
			InvasionEnabled:   sh.InvasionEnabled,
		}
	case domain.ArchetypeBattleRoyale:
		var sb storedBattleRoyale
		if len(sp.Params) > 0 {
			if err := json.Unmarshal(sp.Params, &sb); err != nil {
				return nil, err
			}
		}
		p.BattleRoyale = &domain.BattleRoyaleParams{
			LobbySize:    sb.LobbySize,
			TeamSize:     sb.TeamSize,
			RespawnMode:  sb.RespawnMode,
			ScoringStats: sb.ScoringStats,
		}
	case domain.ArchetypeCoopMission:
		var sc storedCoopMission
		if len(sp.Params) > 0 {
			if err := json.Unmarshal(sp.Params, &sc); err != nil {
				return nil, err
			}
		}
		p.CoopMission = &domain.CoopMissionParams{
			TeamSize:       sc.TeamSize,
			Missions:       sc.Missions,
			Difficulties:   sc.Difficulties,
			BotfillEnabled: sc.BotfillEnabled,
			ScoringStats:   sc.ScoringStats,
		}
	}
	return p, nil
}

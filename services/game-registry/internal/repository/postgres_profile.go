package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/game-registry/internal/domain"
	"github.com/vitormartins1/go-multiplayer-control-plane/services/game-registry/internal/jsonparams"
)

// hostWorldDropInKnownKeys é o vocabulário tipado do arquétipo (tags JSON de
// storedHostWorldDropIn) — mesma lista do transport (DTO diferente, mesmas
// chaves); qualquer chave fora daqui é preservada crua via jsonparams
// (MultiplayerProfile.ExtraParams, follow-up #14).
var hostWorldDropInKnownKeys = map[string]struct{}{
	"enabledRoles":      {},
	"coopSlots":         {},
	"invaderSlots":      {},
	"levelRangeEnabled": {},
	"weaponTierEnabled": {},
	"passwordEnabled":   {},
	"invasionEnabled":   {},
	"scoringStats":      {},
}

// Persistência dos modos de multiplayer (ADR-0023 + ADR-0024, specs 006/009):
// coluna JSONB multiplayer_profile na tabela games. Formato v2 (spec 009):
// { "modes": { "<modeId>": { displayName?, archetype, topology, params } } }.
// O formato v1 (um único objeto { archetype, topology, params } no topo) é
// aceito na leitura e interpretado como o modo "default" (upgrade lazy); todo
// write persiste v2. Params fica cru (RawMessage), decodificado por archetype.

// storedProfile é o corpo de um perfil persistido (formato do ADR-0023).
type storedProfile struct {
	Archetype string          `json:"archetype"`
	Topology  string          `json:"topology"`
	Params    json.RawMessage `json:"params"`
}

// storedMode é um modo persistido (v2): o perfil + o displayName. O modeId é a
// chave do mapa, não um campo do valor.
type storedMode struct {
	DisplayName string `json:"displayName,omitempty"`
	storedProfile
}

// storedModes é o envelope v2 da coluna.
type storedModes struct {
	Modes map[string]storedMode `json:"modes"`
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
	ScoringStats      []string `json:"scoringStats,omitempty"`
}

// SetModes persiste com concorrência otimista (achado no /code-review —
// revisão do "last-write-wins aceito" da spec 009/plan.md): o UPDATE só
// aplica se multiplayer_profile_version ainda bater com expectedVersion (a
// versão lida pelo FindByID que originou este write); a versão avança junto.
// RowsAffected 0 é ambíguo (jogo sumiu OU perdeu a corrida) — uma segunda
// leitura, fora da janela de escrita, distingue as duas causas.
func (r *PostgresGameRepository) SetModes(ctx context.Context, gameID string, modes []domain.MultiplayerMode, expectedVersion int64) error {
	raw, err := encodeModes(modes)
	if err != nil {
		return fmt.Errorf("game-registry repository: set modes: encode: %w", err)
	}
	tag, err := r.pool.Exec(ctx,
		`UPDATE games SET multiplayer_profile = $2, multiplayer_profile_version = multiplayer_profile_version + 1
		 WHERE id = $1 AND deleted_at IS NULL AND multiplayer_profile_version = $3`,
		gameID, raw, expectedVersion)
	if err != nil {
		return fmt.Errorf("game-registry repository: set modes: %w", err)
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		if err := r.pool.QueryRow(ctx,
			`SELECT true FROM games WHERE id = $1 AND deleted_at IS NULL`, gameID,
		).Scan(&exists); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrGameNotFound
			}
			return fmt.Errorf("game-registry repository: set modes: check exists: %w", err)
		}
		return domain.ErrModeConflict
	}
	return nil
}

// encodeModes serializa a coleção de modos no formato v2.
func encodeModes(modes []domain.MultiplayerMode) ([]byte, error) {
	sm := storedModes{Modes: make(map[string]storedMode, len(modes))}
	for i := range modes {
		m := &modes[i]
		raw, err := encodeProfile(&m.Profile)
		if err != nil {
			return nil, fmt.Errorf("modo %q: %w", m.ModeID, err)
		}
		var sp storedProfile
		if err := json.Unmarshal(raw, &sp); err != nil {
			return nil, fmt.Errorf("modo %q: %w", m.ModeID, err)
		}
		sm.Modes[m.ModeID] = storedMode{DisplayName: m.DisplayName, storedProfile: sp}
	}
	return json.Marshal(sm)
}

// decodeModes desserializa a coluna para a coleção de modos, ordenada com
// "default" primeiro e os demais por modeId asc (determinístico para transport
// e service). Aceita o formato v1 (objeto único) como o modo "default" —
// upgrade lazy: o próximo write persiste v2. Coluna NULL/vazia -> nil.
func decodeModes(raw []byte) ([]domain.MultiplayerMode, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var probe struct {
		Modes json.RawMessage `json:"modes"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, err
	}
	if probe.Modes == nil {
		// v1: um único perfil no topo -> modo "default" sem displayName.
		p, err := decodeProfile(raw)
		if err != nil || p == nil {
			return nil, err
		}
		return []domain.MultiplayerMode{{ModeID: domain.DefaultModeID, Profile: *p}}, nil
	}
	var sm storedModes
	if err := json.Unmarshal(raw, &sm); err != nil {
		return nil, err
	}
	out := make([]domain.MultiplayerMode, 0, len(sm.Modes))
	for modeID, stored := range sm.Modes {
		body, err := json.Marshal(stored.storedProfile)
		if err != nil {
			return nil, err
		}
		p, err := decodeProfile(body)
		if err != nil {
			return nil, fmt.Errorf("modo %q: %w", modeID, err)
		}
		if p == nil {
			continue
		}
		out = append(out, domain.MultiplayerMode{ModeID: modeID, DisplayName: stored.DisplayName, Profile: *p})
	}
	sortModes(out)
	return out, nil
}

// sortModes ordena in-place: "default" primeiro, demais por ModeID asc.
func sortModes(modes []domain.MultiplayerMode) {
	sort.Slice(modes, func(i, j int) bool {
		if modes[i].ModeID == domain.DefaultModeID {
			return true
		}
		if modes[j].ModeID == domain.DefaultModeID {
			return false
		}
		return modes[i].ModeID < modes[j].ModeID
	})
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
			ScoringStats:      hw.ScoringStats,
		})
		if err != nil {
			return nil, err
		}
		sp.Params = jsonparams.Merge(params, p.ExtraParams)
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
			ScoringStats:      sh.ScoringStats,
		}
		p.ExtraParams = jsonparams.Extract(sp.Params, hostWorldDropInKnownKeys)
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

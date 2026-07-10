package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/game-registry/internal/domain"
)

// Perfil de multiplayer (ADR-0023, spec 006). A validação é regra de negócio
// pura (aqui, no service); o repositório só persiste. No MVP o único arquétipo
// suportado é queue_arena e a única topologia é dedicated.

// Conjuntos suportados. O de arquétipos/topologias é fechado e cresce por
// evidência (ADR-0023). Os demais são os vocabulários por arquétipo.
var (
	supportedArchetypes = map[string]bool{
		domain.ArchetypeQueueArena:      true,
		domain.ArchetypeHostWorldDropIn: true,
		domain.ArchetypeBattleRoyale:    true,
		domain.ArchetypeCoopMission:     true,
	}
	supportedTopologies = map[string]bool{domain.TopologyDedicated: true}

	// queue_arena
	qaStructures    = map[string]bool{"ffa": true, "teams": true}
	qaWinConditions = map[string]bool{"first_to_score": true, "highest_at_time": true, "objective": true, "elimination": true}
	qaMatchStructs  = map[string]bool{"single": true, "best_of_rounds": true}
	// scoringStatRE: identificador estável de métrica (ex.: "kills", "flag_caps").
	scoringStatRE = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

	// host_world_dropin
	hwdRoles = map[string]bool{"coop": true, "invader": true}

	// battle_royale
	brRespawnModes = map[string]bool{
		domain.RespawnOneLife:        true,
		domain.RespawnTeammateRevive: true,
		domain.RespawnArena1v1:       true,
		domain.RespawnRebootTimer:    true,
	}
)

// Modos nomeados (ADR-0024, spec 009).
var modeIDRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

const (
	maxModesPerGame   = 16
	maxDisplayNameLen = 60
)

// SetMultiplayerProfile é o alias legado (specs 006–008): opera o modo
// "default", preservando o displayName existente (o corpo legado não o tem).
func (s *service) SetMultiplayerProfile(ctx context.Context, gameID string, p domain.MultiplayerProfile) (*domain.MultiplayerProfile, error) {
	m, err := s.setMode(ctx, gameID, domain.DefaultModeID, nil, p)
	if err != nil {
		return nil, err
	}
	return &m.Profile, nil
}

func (s *service) GetMultiplayerProfile(ctx context.Context, gameID string) (*domain.MultiplayerProfile, error) {
	g, err := s.repo.FindByID(ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("game-registry service: get profile: %w", err)
	}
	if g.MultiplayerProfile == nil {
		return nil, domain.ErrProfileNotFound
	}
	return g.MultiplayerProfile, nil
}

func (s *service) SetMode(ctx context.Context, gameID, modeID, displayName string, p domain.MultiplayerProfile) (*domain.MultiplayerMode, error) {
	return s.setMode(ctx, gameID, modeID, &displayName, p)
}

// setMode valida e faz upsert de um modo. displayName == nil preserva o do modo
// existente (caminho do alias legado). Read-modify-write via FindByID +
// SetModes — last-write-wins aceito (plan da spec 009).
func (s *service) setMode(ctx context.Context, gameID, modeID string, displayName *string, p domain.MultiplayerProfile) (*domain.MultiplayerMode, error) {
	if !modeIDRE.MatchString(modeID) {
		return nil, domain.ErrInvalidModeID
	}
	if displayName != nil {
		trimmed := strings.TrimSpace(*displayName)
		if len(trimmed) > maxDisplayNameLen {
			return nil, domain.ErrInvalidProfileParams
		}
		displayName = &trimmed
	}
	if err := validateProfile(p); err != nil {
		return nil, err
	}

	g, err := s.repo.FindByID(ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("game-registry service: set mode: %w", err)
	}
	modes := g.MultiplayerModes
	idx := -1
	for i := range modes {
		if modes[i].ModeID == modeID {
			idx = i
			break
		}
	}

	name := ""
	switch {
	case displayName != nil:
		name = *displayName
	case idx >= 0:
		name = modes[idx].DisplayName
	}
	mode := domain.MultiplayerMode{ModeID: modeID, DisplayName: name, Profile: p}
	if idx >= 0 {
		modes[idx] = mode
	} else {
		if len(modes) >= maxModesPerGame {
			return nil, domain.ErrTooManyModes
		}
		modes = append(modes, mode)
	}

	if err := s.repo.SetModes(ctx, gameID, modes); err != nil {
		return nil, fmt.Errorf("game-registry service: set mode: %w", err)
	}
	return &mode, nil
}

// GetMode também devolve a Fleet do jogo (ADR-0026, spec 012) — já carregada
// no mesmo FindByID, sem query extra.
func (s *service) GetMode(ctx context.Context, gameID, modeID string) (*domain.MultiplayerMode, string, error) {
	g, err := s.repo.FindByID(ctx, gameID)
	if err != nil {
		return nil, "", fmt.Errorf("game-registry service: get mode: %w", err)
	}
	for i := range g.MultiplayerModes {
		if g.MultiplayerModes[i].ModeID == modeID {
			return &g.MultiplayerModes[i], g.Fleet, nil
		}
	}
	return nil, "", domain.ErrModeNotFound
}

func (s *service) ListModes(ctx context.Context, gameID string) ([]domain.MultiplayerMode, error) {
	g, err := s.repo.FindByID(ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("game-registry service: list modes: %w", err)
	}
	// Já ordenada pelo decode (default primeiro, demais por modeId asc).
	return g.MultiplayerModes, nil
}

func (s *service) DeleteMode(ctx context.Context, gameID, modeID string) error {
	g, err := s.repo.FindByID(ctx, gameID)
	if err != nil {
		return fmt.Errorf("game-registry service: delete mode: %w", err)
	}
	if modeID == domain.DefaultModeID {
		return domain.ErrDefaultModeProtected
	}
	modes := g.MultiplayerModes
	idx := -1
	for i := range modes {
		if modes[i].ModeID == modeID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return domain.ErrModeNotFound
	}
	modes = append(modes[:idx], modes[idx+1:]...)
	if err := s.repo.SetModes(ctx, gameID, modes); err != nil {
		return fmt.Errorf("game-registry service: delete mode: %w", err)
	}
	return nil
}

// validateProfile valida arquétipo, topologia e os params do arquétipo (regras
// das specs 006–008, reusadas intactas por modo — ADR-0024).
func validateProfile(p domain.MultiplayerProfile) error {
	if !supportedArchetypes[p.Archetype] {
		return domain.ErrUnsupportedArchetype
	}
	if !supportedTopologies[p.Topology] {
		return domain.ErrUnsupportedTopology
	}
	switch p.Archetype {
	case domain.ArchetypeQueueArena:
		if p.QueueArena == nil {
			return domain.ErrInvalidProfileParams
		}
		return validateQueueArena(*p.QueueArena)
	case domain.ArchetypeHostWorldDropIn:
		if p.HostWorldDropIn == nil {
			return domain.ErrInvalidProfileParams
		}
		return validateHostWorldDropIn(*p.HostWorldDropIn)
	case domain.ArchetypeBattleRoyale:
		if p.BattleRoyale == nil {
			return domain.ErrInvalidProfileParams
		}
		return validateBattleRoyale(*p.BattleRoyale)
	case domain.ArchetypeCoopMission:
		if p.CoopMission == nil {
			return domain.ErrInvalidProfileParams
		}
		return validateCoopMission(*p.CoopMission)
	}
	return nil
}

// validateQueueArena aplica as regras condicionais da spec 006. Rejeita também
// campos incoerentes com o discriminador (ex.: teamCount num perfil ffa) para
// não persistir perfis ambíguos. Erro único: ErrInvalidProfileParams.
func validateQueueArena(p domain.QueueArenaParams) error {
	if !qaStructures[p.Structure] {
		return domain.ErrInvalidProfileParams
	}
	switch p.Structure {
	case "ffa":
		if p.PlayerCount < 2 || p.TeamCount != 0 || p.TeamSize != 0 {
			return domain.ErrInvalidProfileParams
		}
	case "teams":
		if p.TeamCount < 2 || p.TeamSize < 1 || p.PlayerCount != 0 {
			return domain.ErrInvalidProfileParams
		}
	}

	if !qaWinConditions[p.WinCondition] {
		return domain.ErrInvalidProfileParams
	}
	switch p.WinCondition {
	case "first_to_score":
		if p.ScoreTarget <= 0 {
			return domain.ErrInvalidProfileParams
		}
	case "highest_at_time":
		if p.DurationSeconds <= 0 {
			return domain.ErrInvalidProfileParams
		}
	}

	if !qaMatchStructs[p.MatchStructure] {
		return domain.ErrInvalidProfileParams
	}
	switch p.MatchStructure {
	case "single":
		if p.RoundsToWin != 0 {
			return domain.ErrInvalidProfileParams
		}
	case "best_of_rounds":
		if p.RoundsToWin <= 0 {
			return domain.ErrInvalidProfileParams
		}
	}

	for _, stat := range p.ScoringStats {
		if !scoringStatRE.MatchString(stat) {
			return domain.ErrInvalidProfileParams
		}
	}
	return nil
}

// validateCoopMission aplica as regras do arquétipo coop_mission (ADR-0023,
// ficha 04). Exige um time co-op válido (TeamSize >= 2). Missions e Difficulties
// são catálogos declarados: identificadores estáveis sem duplicata (podem ser
// vazios — o catálogo real mora no data plane). ScoringStats segue a convenção
// dos demais arquétipos (regex-only, sem checagem de duplicata).
func validateCoopMission(p domain.CoopMissionParams) error {
	if p.TeamSize < 2 {
		return domain.ErrInvalidProfileParams
	}
	if err := validateStableCatalog(p.Missions); err != nil {
		return err
	}
	if err := validateStableCatalog(p.Difficulties); err != nil {
		return err
	}
	for _, stat := range p.ScoringStats {
		if !scoringStatRE.MatchString(stat) {
			return domain.ErrInvalidProfileParams
		}
	}
	return nil
}

// validateStableCatalog valida uma lista de identificadores estáveis sem
// duplicata (missions/difficulties). Lista vazia é válida. Erro:
// ErrInvalidProfileParams.
func validateStableCatalog(items []string) error {
	seen := make(map[string]bool, len(items))
	for _, it := range items {
		if !scoringStatRE.MatchString(it) || seen[it] {
			return domain.ErrInvalidProfileParams
		}
		seen[it] = true
	}
	return nil
}

// validateHostWorldDropIn aplica as regras do arquétipo host_world_dropin
// (ADR-0023, ficha 01). Exige pelo menos um papel habilitado; coopSlots e
// invaderSlots são obrigatórios (>= 1) apenas quando o papel correspondente
// está em EnabledRoles e devem ser zero quando o papel está ausente.
func validateHostWorldDropIn(p domain.HostWorldDropInParams) error {
	if len(p.EnabledRoles) == 0 {
		return domain.ErrInvalidProfileParams
	}
	seen := make(map[string]bool, len(p.EnabledRoles))
	hasCoop, hasInvader := false, false
	for _, r := range p.EnabledRoles {
		if !hwdRoles[r] || seen[r] {
			return domain.ErrInvalidProfileParams
		}
		seen[r] = true
		if r == "coop" {
			hasCoop = true
		}
		if r == "invader" {
			hasInvader = true
		}
	}
	if hasCoop && p.CoopSlots < 1 {
		return domain.ErrInvalidProfileParams
	}
	if !hasCoop && p.CoopSlots != 0 {
		return domain.ErrInvalidProfileParams
	}
	if hasInvader && p.InvaderSlots < 1 {
		return domain.ErrInvalidProfileParams
	}
	if !hasInvader && p.InvaderSlots != 0 {
		return domain.ErrInvalidProfileParams
	}
	for _, stat := range p.ScoringStats {
		if !scoringStatRE.MatchString(stat) {
			return domain.ErrInvalidProfileParams
		}
	}
	return nil
}

// validateBattleRoyale aplica as regras do arquétipo battle_royale (ADR-0023,
// ficha 03). Exige um squad válido (TeamSize >= 1), um lobby que comporte ao
// menos dois lados (LobbySize >= 2 e >= 2*TeamSize, para haver "último de pé"),
// um respawnMode do vocabulário e scoringStats bem-formados. Não exige LobbySize
// múltiplo de TeamSize: squads incompletos são completados no matchmaking (BR1).
func validateBattleRoyale(p domain.BattleRoyaleParams) error {
	if p.TeamSize < 1 {
		return domain.ErrInvalidProfileParams
	}
	if p.LobbySize < 2 || p.LobbySize < 2*p.TeamSize {
		return domain.ErrInvalidProfileParams
	}
	if !brRespawnModes[p.RespawnMode] {
		return domain.ErrInvalidProfileParams
	}
	for _, stat := range p.ScoringStats {
		if !scoringStatRE.MatchString(stat) {
			return domain.ErrInvalidProfileParams
		}
	}
	return nil
}

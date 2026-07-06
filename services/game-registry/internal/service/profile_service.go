package service

import (
	"context"
	"fmt"
	"regexp"

	"github.com/vitormartins1/go-game-backend/services/game-registry/internal/domain"
)

// Perfil de multiplayer (ADR-0023, spec 006). A validação é regra de negócio
// pura (aqui, no service); o repositório só persiste. No MVP o único arquétipo
// suportado é queue_arena e a única topologia é dedicated.

// Conjuntos suportados. O de arquétipos/topologias é fechado e cresce por
// evidência (ADR-0023). Os demais são o vocabulário do queue_arena (spec 006).
var (
	supportedArchetypes = map[string]bool{domain.ArchetypeQueueArena: true}
	supportedTopologies = map[string]bool{domain.TopologyDedicated: true}

	qaStructures    = map[string]bool{"ffa": true, "teams": true}
	qaWinConditions = map[string]bool{"first_to_score": true, "highest_at_time": true, "objective": true, "elimination": true}
	qaMatchStructs  = map[string]bool{"single": true, "best_of_rounds": true}

	// scoringStatRE: identificador estável de métrica (ex.: "kills", "flag_caps").
	scoringStatRE = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
)

func (s *service) SetMultiplayerProfile(ctx context.Context, gameID string, p domain.MultiplayerProfile) (*domain.MultiplayerProfile, error) {
	if !supportedArchetypes[p.Archetype] {
		return nil, domain.ErrUnsupportedArchetype
	}
	if !supportedTopologies[p.Topology] {
		return nil, domain.ErrUnsupportedTopology
	}
	// Arquétipo suportado no MVP == queue_arena: os params correspondentes têm de
	// estar presentes e válidos.
	if p.QueueArena == nil {
		return nil, domain.ErrInvalidProfileParams
	}
	if err := validateQueueArena(*p.QueueArena); err != nil {
		return nil, err
	}
	if err := s.repo.UpsertMultiplayerProfile(ctx, gameID, p); err != nil {
		return nil, fmt.Errorf("game-registry service: set profile: %w", err)
	}
	return &p, nil
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

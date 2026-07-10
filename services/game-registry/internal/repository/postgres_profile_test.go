package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/game-registry/internal/domain"
)

// encodeProfile/decodeProfile são puras (sem Postgres) — cobertas aqui sem
// precisar de testcontainers; os testes de integração (postgres_game_test.go)
// cobrem o round-trip via banco real.

func TestEncodeDecodeProfile_HostWorldDropIn_PreservaChaveExtra(t *testing.T) {
	original := domain.MultiplayerProfile{
		Archetype: domain.ArchetypeHostWorldDropIn,
		Topology:  domain.TopologyDedicated,
		HostWorldDropIn: &domain.HostWorldDropInParams{
			EnabledRoles: []string{"coop", "invader"},
			CoopSlots:    2, InvaderSlots: 1,
			InvasionEnabled: true,
			ScoringStats:    []string{"kills", "deaths"},
		},
		// bossPowerScalingPerPhantom é fora do vocabulário tipado (follow-up
		// #14) — precisa sobreviver ao round-trip de persistência intacta.
		ExtraParams: map[string]string{"bossPowerScalingPerPhantom": "1.2"},
	}

	raw, err := encodeProfile(&original)
	require.NoError(t, err)

	decoded, err := decodeProfile(raw)
	require.NoError(t, err)
	require.NotNil(t, decoded)

	assert.Equal(t, original.HostWorldDropIn, decoded.HostWorldDropIn)
	assert.Equal(t, map[string]string{"bossPowerScalingPerPhantom": "1.2"}, decoded.ExtraParams)
}

func TestEncodeDecodeProfile_HostWorldDropIn_SemExtra_ExtraParamsNil(t *testing.T) {
	original := domain.MultiplayerProfile{
		Archetype: domain.ArchetypeHostWorldDropIn,
		Topology:  domain.TopologyDedicated,
		HostWorldDropIn: &domain.HostWorldDropInParams{
			EnabledRoles: []string{"coop"}, CoopSlots: 2,
		},
	}

	raw, err := encodeProfile(&original)
	require.NoError(t, err)

	decoded, err := decodeProfile(raw)
	require.NoError(t, err)
	assert.Nil(t, decoded.ExtraParams)
}

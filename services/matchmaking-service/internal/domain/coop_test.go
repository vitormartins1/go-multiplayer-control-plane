package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoopEligible_Tabela(t *testing.T) {
	rangeOnly := CoopPolicy{Archetype: "host_world_dropin", CoopEnabled: true, LevelRangeEnabled: true}
	rangeOff := CoopPolicy{Archetype: "host_world_dropin", CoopEnabled: true, LevelRangeEnabled: false}
	rangeAndTier := CoopPolicy{Archetype: "host_world_dropin", CoopEnabled: true, LevelRangeEnabled: true, WeaponTierEnabled: true}

	// aliado A=100 → banda simétrica [80, 120].
	ally := CoopCriteria{SoulLevel: 100, WeaponTier: 5}

	tests := []struct {
		name       string
		ally       CoopCriteria
		hostLevel  int
		hostTier   int
		policy     CoopPolicy
		want       bool
	}{
		{"limite inferior (80) casa", ally, 80, 0, rangeOnly, true},
		{"limite superior (120) casa", ally, 120, 0, rangeOnly, true},
		{"abaixo (79) não casa", ally, 79, 0, rangeOnly, false},
		{"acima (121) não casa", ally, 121, 0, rangeOnly, false},
		{"mesmo nível casa", ally, 100, 0, rangeOnly, true},
		{"simétrico: -20 casa tanto quanto +20", ally, 80, 0, rangeOnly, true},

		{"range off casa host distante", ally, 999, 0, rangeOff, true},
		{"range off casa host sem nível", ally, 0, 0, rangeOff, true},

		{"range on + host sem nível não casa", ally, 0, 0, rangeOnly, false},
		{"range on + aliado sem nível não casa", CoopCriteria{SoulLevel: 0}, 100, 0, rangeOnly, false},

		{"tier igual casa", ally, 100, 5, rangeAndTier, true},
		{"tier +1 casa", ally, 100, 6, rangeAndTier, true},
		{"tier +2 não casa", ally, 100, 7, rangeAndTier, false},
		{"tier off ignora tier", ally, 100, 99, rangeOnly, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, CoopEligible(tt.ally, tt.hostLevel, tt.hostTier, tt.policy))
		})
	}
}

func TestCoopPolicy_SupportsCoop(t *testing.T) {
	assert.True(t, CoopPolicy{Archetype: "host_world_dropin", CoopEnabled: true}.SupportsCoop())
	assert.False(t, CoopPolicy{Archetype: "host_world_dropin", CoopEnabled: false}.SupportsCoop(), "coop desabilitado")
	assert.False(t, CoopPolicy{Archetype: "queue_arena", CoopEnabled: true}.SupportsCoop(), "arquétipo errado")
}

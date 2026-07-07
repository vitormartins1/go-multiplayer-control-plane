package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEligible_Tabela(t *testing.T) {
	// perfil base: só faixa de nível ligada (weapon tier off).
	rangeOnly := InvasionPolicy{Archetype: "host_world_dropin", InvasionEnabled: true, LevelRangeEnabled: true}
	rangeOff := InvasionPolicy{Archetype: "host_world_dropin", InvasionEnabled: true, LevelRangeEnabled: false}
	rangeAndTier := InvasionPolicy{Archetype: "host_world_dropin", InvasionEnabled: true, LevelRangeEnabled: true, WeaponTierEnabled: true}

	crit := InvaderCriteria{SoulLevel: 100, WeaponTier: 5}

	tests := []struct {
		name   string
		crit   InvaderCriteria
		world  InvadableWorld
		policy InvasionPolicy
		want   bool
	}{
		// banda assimétrica de L=100 → [90, 130].
		{"limite inferior (90) casa", crit, InvadableWorld{HostSoulLevel: 90}, rangeOnly, true},
		{"limite superior (130) casa", crit, InvadableWorld{HostSoulLevel: 130}, rangeOnly, true},
		{"abaixo da banda (89) não casa", crit, InvadableWorld{HostSoulLevel: 89}, rangeOnly, false},
		{"acima da banda (131) não casa", crit, InvadableWorld{HostSoulLevel: 131}, rangeOnly, false},
		{"mesmo nível casa", crit, InvadableWorld{HostSoulLevel: 100}, rangeOnly, true},
		{"assimetria: +30 casa, -11 não (banda desloca p/ cima)", crit, InvadableWorld{HostSoulLevel: 125}, rangeOnly, true},

		// range off → casa qualquer host (inclusive muito distante e sem atributos).
		{"range off casa host distante", crit, InvadableWorld{HostSoulLevel: 500}, rangeOff, true},
		{"range off casa host sem atributos", crit, InvadableWorld{HostSoulLevel: 0}, rangeOff, true},

		// range on + host sem atributos → não casa (não dá para casar por faixa).
		{"range on + host sem atributos não casa", crit, InvadableWorld{HostSoulLevel: 0}, rangeOnly, false},

		// weapon tier: banda 1 em torno do tier do invasor (5).
		{"tier igual casa", crit, InvadableWorld{HostSoulLevel: 100, WeaponTier: 5}, rangeAndTier, true},
		{"tier +1 casa", crit, InvadableWorld{HostSoulLevel: 100, WeaponTier: 6}, rangeAndTier, true},
		{"tier +2 não casa", crit, InvadableWorld{HostSoulLevel: 100, WeaponTier: 7}, rangeAndTier, false},
		{"tier -2 não casa", crit, InvadableWorld{HostSoulLevel: 100, WeaponTier: 3}, rangeAndTier, false},
		{"tier off ignora diferença de tier", crit, InvadableWorld{HostSoulLevel: 100, WeaponTier: 99}, rangeOnly, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Eligible(tt.crit, tt.world, tt.policy))
		})
	}
}

func TestInvasionPolicy_SupportsInvasion(t *testing.T) {
	assert.True(t, InvasionPolicy{Archetype: "host_world_dropin", InvasionEnabled: true}.SupportsInvasion())
	assert.False(t, InvasionPolicy{Archetype: "host_world_dropin", InvasionEnabled: false}.SupportsInvasion(), "invasão desligada")
	assert.False(t, InvasionPolicy{Archetype: "queue_arena", InvasionEnabled: true}.SupportsInvasion(), "arquétipo errado")
}

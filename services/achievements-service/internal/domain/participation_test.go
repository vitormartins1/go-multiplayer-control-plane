package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParticipationEventTypes_Tabela(t *testing.T) {
	tests := []struct {
		name    string
		role    string
		event   string
		outcome string
		want    []string
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
			assert.Equal(t, tt.want, ParticipationEventTypes(tt.role, tt.event, tt.outcome))
		})
	}
}

package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCollectiveEventTypes_Tabela(t *testing.T) {
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
			assert.Equal(t, tt.want, CollectiveEventTypes(tt.outcome))
		})
	}
}

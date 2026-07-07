package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPlacementEventTypes_Tabela(t *testing.T) {
	tests := []struct {
		name      string
		placement int
		want      []string
	}{
		{"1º lugar acumula played/win/top_n", 1, []string{EventBattleRoyalePlayed, EventBattleRoyaleWin, EventBattleRoyaleTopN}},
		{"top 10 (não 1º) gera played/top_n", 7, []string{EventBattleRoyalePlayed, EventBattleRoyaleTopN}},
		{"limiar exato conta como top_n", 10, []string{EventBattleRoyalePlayed, EventBattleRoyaleTopN}},
		{"fora do top N só played", 50, []string{EventBattleRoyalePlayed}},
		{"logo abaixo do 1º ainda é top_n", 2, []string{EventBattleRoyalePlayed, EventBattleRoyaleTopN}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, PlacementEventTypes(tt.placement))
		})
	}
}

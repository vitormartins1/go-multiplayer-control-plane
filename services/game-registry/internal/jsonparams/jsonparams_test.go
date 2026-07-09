package jsonparams

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtract(t *testing.T) {
	known := map[string]struct{}{"coopSlots": {}, "enabledRoles": {}}

	t.Run("chaves fora do vocabulário são preservadas cruas", func(t *testing.T) {
		raw := []byte(`{"coopSlots":2,"enabledRoles":["coop"],"bossPowerScalingPerPhantom":1.2,"hostDeathPolicy":"close_world"}`)
		extra := Extract(raw, known)
		assert.Equal(t, map[string]string{
			"bossPowerScalingPerPhantom": "1.2",
			"hostDeathPolicy":            `"close_world"`,
		}, extra)
	})

	t.Run("só chaves conhecidas -> nil", func(t *testing.T) {
		raw := []byte(`{"coopSlots":2,"enabledRoles":["coop"]}`)
		assert.Nil(t, Extract(raw, known))
	})

	t.Run("vazio ou inválido -> nil sem erro", func(t *testing.T) {
		assert.Nil(t, Extract(nil, known))
		assert.Nil(t, Extract([]byte("not json"), known))
		assert.Nil(t, Extract([]byte(`[1,2,3]`), known))
	})
}

func TestMerge(t *testing.T) {
	t.Run("injeta extras no objeto tipado", func(t *testing.T) {
		raw := []byte(`{"coopSlots":2}`)
		extra := map[string]string{"hostDeathPolicy": `"close_world"`}
		merged := Merge(raw, extra)

		var got map[string]json.RawMessage
		assert.NoError(t, json.Unmarshal(merged, &got))
		assert.Equal(t, json.RawMessage(`2`), got["coopSlots"])
		assert.Equal(t, json.RawMessage(`"close_world"`), got["hostDeathPolicy"])
	})

	t.Run("chave conhecida nunca é sobrescrita por extra", func(t *testing.T) {
		raw := []byte(`{"coopSlots":2}`)
		extra := map[string]string{"coopSlots": "999"}
		merged := Merge(raw, extra)

		var got map[string]json.RawMessage
		assert.NoError(t, json.Unmarshal(merged, &got))
		assert.Equal(t, json.RawMessage(`2`), got["coopSlots"])
	})

	t.Run("extra vazio ou raw inválido devolve raw como veio", func(t *testing.T) {
		raw := []byte(`{"coopSlots":2}`)
		assert.Equal(t, raw, Merge(raw, nil))
		assert.Equal(t, []byte("not json"), Merge([]byte("not json"), map[string]string{"a": "1"}))
	})
}

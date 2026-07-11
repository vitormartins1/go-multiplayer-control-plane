package modesnapshot

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHash_MesmaEntrada_MesmoHash(t *testing.T) {
	params := json.RawMessage(`{"structure":"teams","scoreTarget":5}`)
	a := Hash("queue_arena", "dedicated", params)
	b := Hash("queue_arena", "dedicated", params)
	assert.Equal(t, a, b)
	assert.Len(t, a, 16, "truncado a 16 hex")
}

func TestHash_EntradasDiferentes_HashesDiferentes(t *testing.T) {
	base := Hash("queue_arena", "dedicated", json.RawMessage(`{"scoreTarget":5}`))

	assert.NotEqual(t, base, Hash("battle_royale", "dedicated", json.RawMessage(`{"scoreTarget":5}`)), "archetype diferente")
	assert.NotEqual(t, base, Hash("queue_arena", "relay", json.RawMessage(`{"scoreTarget":5}`)), "topology diferente")
	assert.NotEqual(t, base, Hash("queue_arena", "dedicated", json.RawMessage(`{"scoreTarget":6}`)), "params diferente")
}

func TestHash_ParamsVazio_NaoQuebra(t *testing.T) {
	assert.NotPanics(t, func() {
		Hash("queue_arena", "dedicated", nil)
	})
	assert.Len(t, Hash("queue_arena", "dedicated", nil), 16)
}

// TestHash_SensivelABytesCrus documenta a razão pela qual params tem que ser
// o RawMessage original: dois JSONs "semanticamente iguais" com formatação
// diferente (espaço extra, ordem de chaves) produzem hashes DIFERENTES —
// é isso que garante fidelidade byte-a-byte quando dois caminhos de código
// (session-service e admin-bff) partem da mesma resposta HTTP.
func TestHash_SensivelABytesCrus(t *testing.T) {
	a := Hash("queue_arena", "dedicated", json.RawMessage(`{"a":1,"b":2}`))
	b := Hash("queue_arena", "dedicated", json.RawMessage(`{"b":2,"a":1}`))
	assert.NotEqual(t, a, b, "ordem de chaves diferente -> bytes diferentes -> hash diferente")
}

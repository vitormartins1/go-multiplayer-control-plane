// Package modesnapshot calcula o hash curto do snapshot de um modo de
// multiplayer (spec session-service/017, admin-bff/022) — SHA-256 de
// {archetype, topology, params crus}, truncado a 16 hex. Correlação/log entre
// control e data plane (e, desde a spec 022, entre o snapshot congelado numa
// sessão e o hash "atual" mostrado no console), não integridade
// criptográfica. Fonte única: session-service e admin-bff chamam esta mesma
// função para nunca divergir no cálculo.
package modesnapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// Hash calcula o hash do modo. params precisa ser o json.RawMessage tal como
// veio do game-registry (não um objeto reserializado) — a fidelidade de
// bytes (ordem de chaves, espaçamento) é o que garante que dois caminhos
// diferentes até a mesma resposta HTTP produzam o mesmo hash.
func Hash(archetype, topology string, params json.RawMessage) string {
	h := sha256.New()
	h.Write([]byte(archetype))
	h.Write([]byte{0})
	h.Write([]byte(topology))
	h.Write([]byte{0})
	h.Write(params)
	return hex.EncodeToString(h.Sum(nil))[:16]
}

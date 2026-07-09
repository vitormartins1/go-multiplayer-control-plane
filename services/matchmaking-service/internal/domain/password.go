package domain

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashWorldPassword hasheia a senha do mundo (follow-up #1, ADR-0023, spec
// matchmaking/011): SHA-256 hex, string vazia -> hash vazio (nunca o hash de
// ""). Duplicada deliberadamente de session-service
// (internal/domain/password.go) — os dois serviços são módulos independentes
// (ADR-0001) e a função é trivial; introduzir um pacote crypto compartilhado
// só pra isto não compensa. Mudar o algoritmo exige tocar os dois lados.
func HashWorldPassword(plaintext string) string {
	if plaintext == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

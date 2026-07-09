package domain

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashWorldPassword hasheia a senha do mundo (follow-up #1, ADR-0023, spec
// session/020): SHA-256 hex, string vazia -> hash vazio (nunca o hash de "").
// Não é uma credencial de autenticação — é um código de convite social (baixo
// risco, mesma categoria de um link de sala); o objetivo é só não deixar a
// senha em texto puro no estado da sessão, não resistir a força bruta
// direcionada. Duplicada deliberadamente em matchmaking-service
// (internal/domain/password.go) para não acoplar os dois serviços a um
// pacote crypto compartilhado — mudar o algoritmo exige tocar os dois lados.
func HashWorldPassword(plaintext string) string {
	if plaintext == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

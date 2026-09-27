package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// newWorldToken cunha o world token de um mundo p2p (spec 031): 32 bytes
// aleatórios em base64url. Devolve o token (vai só na resposta do OpenWorld) e
// o hash que o session guarda — um vazamento do estado não entrega a credencial.
func newWorldToken() (token, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("world token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, hashWorldToken(token), nil
}

func hashWorldToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// ResolveWorldToken devolve o mundo aberto dono do world token (spec 031). O
// índice só contém mundos abertos: closeSession remove a entrada junto com a
// sessão, então um token de mundo fechado deixa de valer na hora.
func (s *Service) ResolveWorldToken(token string) (string, bool) {
	if token == "" {
		return "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sessionID, ok := s.worldTokens[hashWorldToken(token)]
	return sessionID, ok
}

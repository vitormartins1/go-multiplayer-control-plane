package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/domain"
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

// worldTokenEntry é o mundo dono de um world token e se ele já fechou.
type worldTokenEntry struct {
	sessionID string
	closed    bool
}

// retireWorldTokenLocked marca o token do mundo que fechou. Ele continua
// resolvendo (closed=true) até sair da janela — igual ao histórico em memória,
// onde o closingRetryLocked procura a ocorrência que fechou o mundo. Exige s.mu.
func (s *Service) retireWorldTokenLocked(hash string) {
	e, ok := s.worldTokens[hash]
	if !ok || e.closed {
		return
	}
	e.closed = true
	s.worldTokens[hash] = e
	s.closedTokens = append(s.closedTokens, hash)
	if over := len(s.closedTokens) - s.historyLimit; over > 0 {
		for _, old := range s.closedTokens[:over] {
			delete(s.worldTokens, old)
		}
		s.closedTokens = s.closedTokens[over:]
	}
}

// ResolveWorldToken devolve o mundo dono do world token (spec 031) e se ele
// já fechou. Mundo fechado só serve para o reenvio do evento que o fechou
// (spec 029) — quem decide isso é o interceptor e o closingRetryLocked.
func (s *Service) ResolveWorldToken(token string) (string, bool, bool) {
	if token == "" {
		return "", false, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.worldTokens[hashWorldToken(token)]
	return e.sessionID, e.closed, ok
}

// hasParticipant diz se o jogador está ou esteve no mundo (host incluso).
func hasParticipant(sess *domain.Session, playerID string) bool {
	for _, p := range sess.Participants {
		if p.PlayerID == playerID {
			return true
		}
	}
	return false
}

// maxHostConnectionLen limita o id de conexão do host (spec 031): SteamID64
// com prefixo cabe com folga; o valor é opaco, mas vai para o histórico.
const maxHostConnectionLen = 128

// validHostConnection exige um id de conexão presente, curto e sem caracteres
// de controle.
func validHostConnection(conn string) error {
	if conn == "" {
		return domain.ErrHostConnectionRequired
	}
	if len(conn) > maxHostConnectionLen {
		return fmt.Errorf("host connection too long: %w", domain.ErrHostConnectionRequired)
	}
	for _, r := range conn {
		if r < 0x21 || r == 0x7f {
			return fmt.Errorf("host connection with invalid character: %w", domain.ErrHostConnectionRequired)
		}
	}
	return nil
}

// closeHostP2PWorlds fecha (host_left) os mundos p2p abertos do host.
func (s *Service) closeHostP2PWorlds(ctx context.Context, hostPlayerID string) {
	s.mu.Lock()
	var ids []string
	for id, sess := range s.sessions {
		if sess.HostConnection != "" && sess.HostPlayerID == hostPlayerID {
			ids = append(ids, id)
		}
	}
	s.mu.Unlock()
	for _, id := range ids {
		if _, err := s.closeSession(ctx, id, domain.CloseHostLeft); err != nil && !errors.Is(err, domain.ErrSessionNotFound) {
			s.log.Warn("open world: falha ao fechar o mundo p2p anterior do host", "session_id", id, "err", err)
		}
	}
}

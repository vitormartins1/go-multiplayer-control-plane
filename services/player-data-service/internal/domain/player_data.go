// Package domain contém os tipos de negócio puros e as interfaces (Ports)
// do player-data-service. Zero dependências externas. Ver ADR-0002.
//
// O valor é JSON opaco (`[]byte`): o serviço não interpreta nem serializa o
// conteúdo — a validação "é JSON?" vive no transport, a persistência (JSONB)
// no repository.
package domain

import "context"

// Entry é uma linha de dado do jogador (namespace/key/value), usada no admin
// read (ListForPlayer) que alimenta a aba Player data do painel.
type Entry struct {
	Namespace string
	Key       string
	Value     []byte
}

// PlayerDataService é a Port de entrada do player-data-service. Todo acesso é
// escopado por gameID (multi-tenant por jogo, ADR-0017).
type PlayerDataService interface {
	Get(ctx context.Context, gameID, playerID, namespace, key string) (value []byte, err error)
	GetNamespace(ctx context.Context, gameID, playerID, namespace string) (map[string][]byte, error)
	Put(ctx context.Context, gameID, playerID, namespace, key string, value []byte) error
	Delete(ctx context.Context, gameID, playerID, namespace, key string) error
	// ListForPlayer devolve todo o dado de um jogador num jogo (admin read).
	ListForPlayer(ctx context.Context, gameID, playerID string) ([]Entry, error)
}

// PlayerDataRepository é a Port de saída de persistência (implementada em repository).
type PlayerDataRepository interface {
	Get(ctx context.Context, gameID, playerID, namespace, key string) (value []byte, err error)
	GetNamespace(ctx context.Context, gameID, playerID, namespace string) (map[string][]byte, error)
	Put(ctx context.Context, gameID, playerID, namespace, key string, value []byte) error
	Delete(ctx context.Context, gameID, playerID, namespace, key string) error
	ListForPlayer(ctx context.Context, gameID, playerID string) ([]Entry, error)
}

// TokenVerifier valida o JWT do identity e devolve o playerID (sub).
// Implementada por shared/auth (JWKS). O domínio não conhece HTTP nem token.
type TokenVerifier interface {
	Verify(ctx context.Context, token string) (playerID string, err error)
}

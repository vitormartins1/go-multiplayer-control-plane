// Package domain contém os tipos de negócio puros e as interfaces (Ports)
// do cloud-save-service. Zero dependências externas. Ver ADR-0002.
package domain

import (
	"context"
	"time"
)

// Snapshot é o save de um jogador: blob opaco + content-type. O serviço não
// interpreta o conteúdo do blob. UpdatedAt é preenchido na leitura (LastModified
// do object storage) e fica zero na escrita.
type Snapshot struct {
	Blob        []byte
	ContentType string
	UpdatedAt   time.Time
}

// CloudSaveService é a Port de entrada do cloud-save-service. Todo acesso é
// escopado por gameID (multi-tenant por jogo, ADR-0017).
type CloudSaveService interface {
	Save(ctx context.Context, gameID, playerID string, snapshot Snapshot) error
	Load(ctx context.Context, gameID, playerID string) (Snapshot, error)
	// LoadFor é o read administrativo do save de um jogador num jogo (admin).
	LoadFor(ctx context.Context, gameID, playerID string) (Snapshot, error)
}

// SaveStore é a Port de saída de armazenamento (implementada por um adapter de
// object storage — S3/MinIO; ADR-0015). A key é escopada por jogo.
type SaveStore interface {
	Put(ctx context.Context, gameID, playerID string, snapshot Snapshot) error // sobrescreve
	Get(ctx context.Context, gameID, playerID string) (Snapshot, error)        // ErrNotFound se ausente
}

// TokenVerifier valida o JWT do identity e devolve o playerID (sub).
// Implementada por shared/auth (JWKS).
type TokenVerifier interface {
	Verify(ctx context.Context, token string) (playerID string, err error)
}

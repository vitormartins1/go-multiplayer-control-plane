package sessionclient

import (
	"context"

	"google.golang.org/grpc/credentials"
)

// internalKeyCreds manda a internal key (ADR-0018) em toda chamada ao session
// como metadata x-internal-key — é assim que o session reconhece o matchmaking
// (spec session/030, ADR-0030).
type internalKeyCreds struct{ key string }

// InternalKeyCredentials devolve as credenciais por RPC para o dial do
// session (grpc.WithPerRPCCredentials).
func InternalKeyCredentials(key string) credentials.PerRPCCredentials {
	return internalKeyCreds{key: key}
}

func (c internalKeyCreds) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{"x-internal-key": c.key}, nil
}

// RequireTransportSecurity: a rede entre serviços é interna (mesmo critério do
// HTTP da ADR-0018); TLS entre serviços fica para o deploy real.
func (internalKeyCreds) RequireTransportSecurity() bool { return false }

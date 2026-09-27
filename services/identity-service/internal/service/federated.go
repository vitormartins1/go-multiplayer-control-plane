package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/identity-service/internal/domain"
)

// AuthenticateExternal troca um ticket de loja pelo session token (spec
// 005-steam-login): o provedor valida o ticket, a conta vinculada é achada ou
// criada (sem senha) e o JWT sai igual ao do login por senha, com o claim
// extra idp. O resto do control plane não sabe de onde o jogador veio.
func (s *service) AuthenticateExternal(ctx context.Context, provider, ticket, gameID string) (string, error) {
	v, ok := s.verifiers[provider]
	if !ok || s.external == nil {
		return "", domain.ErrProviderNotSupported
	}
	ext, err := v.Verify(ctx, ticket)
	if err != nil {
		if errors.Is(err, domain.ErrProviderUnavailable) && s.log != nil {
			// sem isto a Steam fora do ar era invisível para o operador (o
			// transport só devolve 503). O erro do verificador não traz a URL.
			s.log.Warn("login externo: provedor indisponível",
				slog.String("provider", provider), slog.String("err", err.Error()))
		}
		return "", fmt.Errorf("identity service: external login: %w", err)
	}

	acc, err := s.accountFor(ctx, ext)
	if errors.Is(err, domain.ErrHandleTaken) {
		// os dois handles tentados colidiram: é falha nossa, não do jogador —
		// não pode sair como 409 handle_taken num login.
		return "", fmt.Errorf("identity service: external login: handle: %v", err)
	}
	if err != nil {
		return "", fmt.Errorf("identity service: external login: %w", err)
	}
	token, err := s.tokens.Sign(ctx, acc.ID, map[string]any{"idp": ext.Provider})
	if err != nil {
		return "", fmt.Errorf("identity service: external login: sign: %w", err)
	}
	s.markPresence(ctx, gameID, acc.ID, acc.Handle) // login-into-game (ADR-0019)
	return token, nil
}

// accountFor acha a conta vinculada ou cria uma nova, tratando as duas
// corridas possíveis: o mesmo jogador logando duas vezes ao mesmo tempo
// (vínculo já existe → relê) e o handle derivado já ocupado (tenta um
// alternativo uma vez).
func (s *service) accountFor(ctx context.Context, ext domain.ExternalIdentity) (*domain.Account, error) {
	acc, err := s.external.FindByExternal(ctx, ext.Provider, ext.ExternalID)
	if err == nil {
		return acc, nil
	}
	if !errors.Is(err, domain.ErrAccountNotFound) {
		return nil, err
	}

	handle := externalHandle(ext)
	for attempt := 0; attempt < 2; attempt++ {
		acc = &domain.Account{
			ID:           uuid.NewString(),
			Handle:       handle,
			PasswordHash: "", // sem senha: o bcrypt nunca aceita — login só pelo provedor
			CreatedAt:    time.Now().UTC(),
		}
		err = s.external.CreateWithExternal(ctx, acc, ext.Provider, ext.ExternalID)
		switch {
		case err == nil:
			return acc, nil
		case errors.Is(err, domain.ErrExternalLinked):
			return s.external.FindByExternal(ctx, ext.Provider, ext.ExternalID)
		case errors.Is(err, domain.ErrHandleTaken):
			handle = alternativeHandle(ext.Provider)
		default:
			return nil, err
		}
	}
	return nil, err
}

// externalHandle deriva o handle da identidade externa: provedor + os últimos
// dígitos do ID, dentro do limite de 20 caracteres do handle. Para a Steam, os
// 14 últimos dígitos do SteamID64 identificam a conta (base fixa + accountID
// de 32 bits), então não colidem entre jogadores Steam.
func externalHandle(ext domain.ExternalIdentity) string {
	prefix := ext.Provider + "_"
	keep := 20 - len(prefix)
	id := ext.ExternalID
	if len(id) > keep {
		id = id[len(id)-keep:]
	}
	return prefix + id
}

// alternativeHandle é o plano B quando o handle derivado já foi registrado à
// mão por alguém: provedor + sufixo aleatório.
func alternativeHandle(provider string) string {
	return provider + "_" + uuid.NewString()[:8]
}

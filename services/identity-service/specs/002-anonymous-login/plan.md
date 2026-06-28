# Plan: 002-anonymous-login
**Spec:** specs/002-anonymous-login/spec.md
**Data:** 2026-06-27

## Abordagem técnica

Incremento sobre o 001-auth-core. Não há entidade nova nem persistência (guest stateless). Adiciona-se um caso de uso `LoginAnonymous` no `service` e um endpoint `POST /sessions/anonymous` no `transport`. O `TokenIssuer` do 001 (já projetado para aceitar claims extras) é reutilizado para emitir o token com `anon: true`.

## Compute e banco

Nenhum banco (stateless). Mesmo serviço/compute do identity (decisão de deploy no 001/ADR-0009). Sem migration.

## Arquivos a criar ou modificar

- `internal/domain/account.go` — adicionar `LoginAnonymous(ctx) (string, error)` à Port `AuthService` (modificar)
- `internal/service/auth_service.go` — implementar `LoginAnonymous` (modificar)
- `internal/service/auth_service_test.go` — unit tests da 002 (modificar)
- `internal/transport/http_handler.go` — adicionar `POST /sessions/anonymous` (modificar)
- `internal/transport/http_handler_test.go` — httptest da 002 (modificar)

## Dependências e libs

Nenhuma nova — reutiliza `github.com/google/uuid` (id do guest) e o `TokenIssuer` do 001.

## Decisões técnicas e trade-offs

- **Guest stateless:** `sub = "guest-" + uuid`, claim `anon: true`; nada persistido. Continuidade/linking ficam no backlog.
- **Reuso do TokenIssuer:** o mesmo `Sign(subject, claims...)` do 001 emite o token anônimo — mesma chave, mesmo JWKS, verificação idêntica.
- **Sem validação de entrada:** endpoint sem corpo; não há regra de validação.

## Ordem de implementação

> Depende do 001 já implementado (modifica `service` e `transport` dele).

1. `service.LoginAnonymous` + unit tests (mock do TokenIssuer)
2. `transport`: endpoint `POST /sessions/anonymous` + httptest
3. `go test ./... -race` e os casos da spec 002 verdes

## Riscos e pontos de atenção

- Garantir que o token anônimo usa a MESMA chave/JWKS do 001 (nenhum caminho de assinatura separado).
- Não introduzir persistência por engano (guest é efêmero).
- Cada login anônimo deve gerar um `sub` distinto (uuid novo).

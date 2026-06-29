# Plan: 003-admin-list-users
**Spec:** specs/003-admin-list-users/spec.md
**Data:** 2026-06-29

## Abordagem técnica

Hexagonal (ADR-0002), atravessando as 3 camadas (diferente do `achievements/003`, que era só transport): novos métodos de leitura no `AccountRepository` (`ListAccounts`/`CountAccounts`), um método de service `ListPlayers` que **aplica as regras de paginação** (clamp), e uma rota admin `GET /admin/players` no transport (sem auth, control plane interno). Sem migration — usa a tabela `accounts` existente, **projetando só `id, handle, created_at`** na query (o `password_hash` nem é selecionado — defesa na fonte, não só no DTO).

O handler passa a depender de uma nova Port de entrada `domain.PlayerLister` (transport depende de interface, não do concreto). O `service.Service` atual (que já tem o repo) implementa `PlayerLister` além de `AuthService`; o `main` injeta o mesmo `svc`.

## Arquivos a criar ou modificar

- `internal/domain/account.go` (onde estão as interfaces) —
  - `AccountRepository` += `ListAccounts(ctx, limit, offset int) ([]Account, error)` e `CountAccounts(ctx) (int, error)`
  - nova Port de entrada `PlayerLister interface { ListPlayers(ctx, limit, offset int) (players []Account, total int, err error) }`
- `internal/repository/postgres_account.go` —
  - `ListAccounts`: `SELECT id, handle, created_at FROM accounts ORDER BY created_at DESC LIMIT $1 OFFSET $2` (não seleciona `password_hash`)
  - `CountAccounts`: `SELECT count(*) FROM accounts`
- `internal/repository/postgres_account_test.go` — integration (testcontainers): ordena por created_at desc; paginação (limit/offset); count global; `PasswordHash` vem vazio (não selecionado)
- `internal/service/admin_list.go` (novo) — `ListPlayers`: **clamp** (`limit` 1..200 default 50; `offset` >=0) e chama `ListAccounts` + `CountAccounts`
- `internal/service/admin_list_test.go` — unit (mock repo): clamp dos limites; repassa offset; devolve total
- `internal/transport/http_handler.go` —
  - `NewHandler` ganha o parâmetro `players domain.PlayerLister`
  - `Register` += `GET /admin/players` (sem `h.auth`/sem JWT — não há middleware obrigatório aqui, mas a rota é claramente admin)
  - handler: parseia `limit`/`offset` (default em valor ausente/inválido), chama `ListPlayers`, projeta DTO `{id, handle, createdAt}`, responde `{total, players}`
- `internal/transport/http_handler_test.go` — 200 com lista+total; **sem `password_hash`** no corpo; query `limit`/`offset` repassada; sem `Authorization` não bloqueia
- `cmd/server/main.go` — `NewHandler(svc, issuer, svc)` (injeta o lister)
- `api/identity.http` — exemplo do `GET /admin/players` (nota "interno, não expor")

> **Sem migration** e sem mudança de schema. Um índice em `created_at` ajudaria a ordenação em escala — anotado como melhoria, não necessário no MVP.

## Compute e banco (ver ADR-0005)
- Sem mudança: mesmo identity-service, mesmo Postgres. Só leitura nova.

## Dependências e libs
- Nenhuma nova. `pgx` (já usado), `net/http`, `strconv` (parse de query).

## Decisões técnicas e trade-offs
- **Projetar `id, handle, created_at` na própria query** (não selecionar `password_hash`): impede vazamento de credencial mesmo que o DTO mude no futuro — defesa na fonte. `Account.PasswordHash` fica vazio nesse caminho (ok, é leitura admin).
- **Clamp da paginação no service** (regra de domínio), parse no transport (string->int com default): a política de limites (1..200) fica testável em unit, fora do HTTP.
- **offset/limit** (não cursor): simples e suficiente para a base de usuários no MVP; cursor é evolução se a tabela crescer muito.
- **Nova Port `PlayerLister`** em vez de inflar `AuthService`: listar usuários não é autenticação; manter as interfaces coesas. O `service.Service` satisfaz ambas.
- **Sem auth** (decisão da spec): `/admin/...` interno/local, consistente com `achievements/003`.

## Ordem de implementação
1. `domain`: `ListAccounts`/`CountAccounts` no repo Port + `PlayerLister`
2. `repository`: implementar as duas queries + integration tests (testcontainers): ordem, paginação, count, sem hash
3. `service`: `ListPlayers` (clamp) + unit tests (mock repo)
4. `transport`: rota + handler + DTO + tests (sem hash; paginação; sem auth)
5. `main`: injetar o lister no `NewHandler`
6. `api/identity.http` (exemplo); `go test ./... -race` + `go build ./...`

## Riscos e pontos de atenção
- **Vazamento de hash:** garantir que nem a query nem o DTO incluem `password_hash` — coberto por teste (integration: campo vazio; handler: corpo não contém a string).
- **Ordenação determinística:** `created_at` pode empatar (mesmo instante em inserts de teste) — desempatar por `id` na ordenação para o teste de paginação ser estável (`ORDER BY created_at DESC, id`).
- **Clamp/parse:** cobrir `limit` > 200, negativo, não numérico, ausente.
- **`NewHandler` muda de assinatura:** ajustar o `main` e o setup dos handler tests existentes.

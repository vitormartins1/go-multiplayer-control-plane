# Tasks: 020-password-world-matching

Follow-up #1 (senha/play-with-friends) do `host_world_dropin` (ADR-0023), lado session. Aditiva sobre H0/H1a. Marcar `[x]` ao concluir. Ver `plan.md` para as decisões (D1–D5).

Baseline: `go test ./services/session-service/... -race` **verde** antes de começar.

## Domínio

- [x] **T1 — `HashWorldPassword`.** Novo `internal/domain/password.go`: SHA-256 hex; string vazia → hash vazio (nunca o hash de `""`). Teste (`password_test.go`): vazio→vazio, determinístico, senhas diferentes → hashes diferentes.
- [x] **T2 — campos.** `internal/domain/session.go`: `WorldAttributes` += `Password string`; `Session` += `PasswordHash string`; `InvadableWorld` (spec 012) += `PasswordHash string`.

## Service

- [x] **T3 — OpenWorld hasheia e grava.** `internal/service/session_service.go`: `OpenWorld` chama `domain.HashWorldPassword(attrs.Password)` e grava em `sess.PasswordHash`. Teste: com senha → hash correto gravado; sem senha → hash vazio.
- [x] **T4 — ListInvadableWorlds copia o hash.** Cada `InvadableWorld` montado inclui `PasswordHash: sess.PasswordHash`. Teste: diretório reflete o hash de cada mundo (com e sem senha).

## Proto + transport

- [x] **T5 — proto.** `proto/session/v1/session.proto`: índices confirmados livres (6 em `OpenWorldRequest`, 5 em `InvadableWorld`, como estimado). Regenerado com `PATH="$(go env GOPATH)/bin:$PATH" buf generate`.
- [x] **T6 — handlers.** `internal/transport/grpc_server.go`: `OpenWorld` passa `req.GetPassword()` em `WorldAttributes`; `ListInvadableWorlds` inclui `password_hash` na resposta. Teste bufconn (`host_world_grpc_test.go`): `TestOpenWorld_ComSenha_HashApareceNoDiretorio` / `TestOpenWorld_SemSenha_HashVazioNoDiretorio`.

## Fechamento

- [x] **T7 — verde + lint.** `go test ./services/session-service/... -race -cover`: 155 testes verdes; cobertura `service` 94.1%, `domain` 100%. `go vet` limpo (`golangci-lint` não estava instalado no ambiente desta sessão — não rodado). Queue_arena + H0/H1a sem regressão.
- [x] **T8 — coordenar com o par.** Hash confirmado: SHA-256 hex, `HashWorldPassword("") == ""`. `matchmaking/011` implementada na sequência usando o mesmo formato (duplicado deliberadamente, D2).

# Tasks: 020-password-world-matching

Follow-up #1 (senha/play-with-friends) do `host_world_dropin` (ADR-0023), lado session. Aditiva sobre H0/H1a. Marcar `[x]` ao concluir. Ver `plan.md` para as decisões (D1–D5).

Baseline: `go test ./services/session-service/... -race` **verde** antes de começar.

## Domínio

- [ ] **T1 — `HashWorldPassword`.** Novo `internal/domain/password.go`: SHA-256 hex; string vazia → hash vazio (nunca o hash de `""`). Teste (`password_test.go`): vazio→vazio, determinístico, senhas diferentes → hashes diferentes.
- [ ] **T2 — campos.** `internal/domain/session.go`: `WorldAttributes` += `Password string`; `Session` += `PasswordHash string`; `InvadableWorld` (spec 012) += `PasswordHash string`.

## Service

- [ ] **T3 — OpenWorld hasheia e grava.** `internal/service/session_service.go`: `OpenWorld` chama `domain.HashWorldPassword(attrs.Password)` e grava em `sess.PasswordHash`. Teste: com senha → hash correto gravado; sem senha → hash vazio.
- [ ] **T4 — ListInvadableWorlds copia o hash.** Cada `InvadableWorld` montado inclui `PasswordHash: sess.PasswordHash`. Teste: diretório reflete o hash de cada mundo (com e sem senha).

## Proto + transport

- [ ] **T5 — proto.** `proto/session/v1/session.proto`: conferir o próximo índice livre real em `OpenWorldRequest` e `InvadableWorld` (a spec estima 6/5, confirmar antes de gerar); adicionar `string password` / `string password_hash`. Regenerar: `PATH="$(go env GOPATH)/bin:$PATH" buf generate` (nunca `sed`).
- [ ] **T6 — handlers.** `internal/transport/grpc_server.go`: `OpenWorld` passa `req.GetPassword()` em `WorldAttributes`; `ListInvadableWorlds` inclui `password_hash` na resposta. Teste bufconn (`host_world_grpc_test.go`): OpenWorld com senha → ListInvadableWorlds mostra o hash certo.

## Fechamento

- [ ] **T7 — verde + lint.** `go test ./services/session-service/... -race -cover` (≥80% service/domain). `golangci-lint run ./services/session-service/...`. Confirmar queue_arena + H0/H1a sem regressão.
- [ ] **T8 — coordenar com o par.** Confirmar que `matchmaking/011-password-matching` (lado matchmaking) e a spec 003 do Unity (`unity-host-world-dropin-data-plane`) estão cientes do formato final do hash (SHA-256 hex) antes de implementar o lado deles.

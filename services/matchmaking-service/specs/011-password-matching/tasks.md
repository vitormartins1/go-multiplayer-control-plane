# Tasks: 011-password-matching

Follow-up #1 (senha/play-with-friends) do `host_world_dropin` (ADR-0023), lado matchmaking. Aditiva sobre `matchmaking/006`/`007`. Marcar `[x]` ao concluir. Ver `plan.md` para as decisões (D1–D5).

**Pré-requisito: `session/020-password-world-matching` implementada** (o `password_hash` no diretório de invadíveis precisa existir).

Baseline: `go test ./services/matchmaking-service/... -race` **verde** antes de começar.

## Domínio

- [ ] **T1 — `HashWorldPassword`.** Novo `internal/domain/password.go` (duplicada de `session/020`, D2): SHA-256 hex; vazio→vazio. Mesma tabela de teste da spec irmã.
- [ ] **T2 — campos.** `internal/domain/invasion.go`: `InvaderCriteria` += `Password string`; `InvadableWorld` += `PasswordHash string`. `internal/domain/coop.go`: `Sign` += `Password string`.

## Service

- [ ] **T3 — bypass em `Invade`.** `internal/service/invader.go`: no loop de `worlds`, calcular `matchesPassword` (D1) e trocar `if !domain.Eligible(...)` por `if !matchesPassword && !domain.Eligible(...)`. Testes: senha batendo mundo fora de faixa → injeta; senha vazia/errada → idêntico à spec 006; sem `InvasionEnabled` → rejeita mesmo com senha certa.
- [ ] **T4 — `PostSummonSign` grava a senha do sinal.** `internal/service/summoner.go` (ou onde o `Sign` é criado): grava `Password` vindo do request.
- [ ] **T5 — bypass em `ListSigns`.** `internal/service/summoner.go`: no loop de sinais, `matchesPassword` (D1) via `password == sg.Password`; `if matchesPassword || domain.CoopEligible(...)`. Testes: senha batendo sinal fora de faixa → aparece; senha vazia/errada → idêntico à spec 007.

## Adapter

- [ ] **T6 — `WorldDirectory` mapeia o hash.** Adapter que implementa `WorldDirectory.ListInvadableWorlds` (chamador do `session.v1.ListInvadableWorlds`): copiar `password_hash` da resposta pro `domain.InvadableWorld.PasswordHash`.

## Proto + transport

- [ ] **T7 — proto.** `proto/matchmaking/v1/matchmaking.proto`: conferir índices livres reais (a spec estima campo 5 em `InvadeRequest`, `PostSummonSignRequest`, `ListSignsRequest`); adicionar `string password` nos três. Regenerar: `PATH="$(go env GOPATH)/bin:$PATH" buf generate`.
- [ ] **T8 — handlers.** `internal/transport/grpc_server.go`: os 3 handlers passam `req.GetPassword()` adiante. Testes bufconn: bypass e2e em cada RPC.

## Fechamento

- [ ] **T9 — verde + lint.** `go test ./services/matchmaking-service/... -race -cover` (≥80% service/domain). `golangci-lint run ./services/matchmaking-service/...`. Confirmar `FindMatch`/`matchmaking/006`/`007` sem regressão (testes existentes de `Eligible`/`CoopEligible` intocados).
- [ ] **T10 — coordenar com o par.** Confirmar com a spec 003 do Unity (`unity-host-world-dropin-data-plane`) o formato final dos 3 campos `password` antes do client consumir.

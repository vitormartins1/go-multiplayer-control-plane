# Plan: 011-password-matching

**Serviço:** matchmaking-service · **Spec:** `spec.md` (Proposta 2026-07-09) · **ADR:** 0023 (follow-up #1, lado matchmaking)

Como o matchmaking passa a bypassar `Eligible`/`CoopEligible` quando a senha bate, sem mudar as funções puras de faixa. Fatia **aditiva** sobre `matchmaking/006` e `matchmaking/007`.

## Decisões técnicas

### D1 — bypass é um OR no call site, não um parâmetro dentro de `Eligible`/`CoopEligible`

`Eligible(crit, w, policy)` e `CoopEligible(ally, hostSoulLevel, hostWeaponTier, policy)` continuam **só faixa** — testadas isoladamente como hoje. O bypass entra em `Invader.Invade` (`invader.go:72`) e `Summoner.ListSigns` (`summoner.go:117`):

```go
// invader.go — dentro do loop de worlds
matchesPassword := crit.Password != "" && w.PasswordHash != "" &&
    domain.HashWorldPassword(crit.Password) == w.PasswordHash
if !matchesPassword && !domain.Eligible(crit, w, policy) {
    continue
}
```

```go
// summoner.go — dentro do loop de sinais
matchesPassword := password != "" && sg.Password != "" && password == sg.Password
if matchesPassword || domain.CoopEligible(ally, host.SoulLevel, host.WeaponTier, policy) {
    out = append(out, sg.Sign)
}
```

Nota: a comparação do co-op é **plaintext == plaintext** (`password == sg.Password`), não hash — a senha do sinal nunca sai do processo do matchmaking (não atravessa o session), então não há razão para hashear ali; hashear só faz sentido onde a senha atravessa uma fronteira de serviço (a invasão, via `session/020`).

### D2 — `HashWorldPassword` duplicada de `session/020`

Mesma função (SHA-256 hex, vazio→vazio) implementada aqui de novo, não importada do session-service. Ver justificativa em `session/020` plan D2 (módulos independentes, ADR-0001; função trivial). Comentário cruzado nos dois arquivos.

### D3 — `WorldDirectory` (adapter `sessionclient`, spec 006) mapeia o hash

O adapter que implementa `WorldDirectory.ListInvadableWorlds` (chama `session.v1.ListInvadableWorlds`) passa a copiar `resp.GetWorlds()[i].GetPasswordHash()` para `domain.InvadableWorld.PasswordHash`. Pequena mudança no adapter existente, sem nova Port.

### D4 — proto aditivo + `buf`

- `InvadeRequest` += `string password = 5;`.
- `PostSummonSignRequest` += `string password = 5;`.
- `ListSignsRequest` += `string password = 5;`.
- Conferir índices livres reais antes de gerar (a spec estima 5 nos três, todos com 1–4 ocupados hoje). `PATH="$(go env GOPATH)/bin:$PATH" buf generate`.

### D5 — transport

- Handler `Invade`: `crit.Password = req.GetPassword()`.
- Handler `PostSummonSign`: `Sign.Password = req.GetPassword()` ao criar o sinal.
- Handler `ListSigns`: passa `req.GetPassword()` como parâmetro extra pro `Summoner.ListSigns` (assinatura ganha o parâmetro — ver Arquivos a tocar).

## Domínio

- `invasion.go`: `InvaderCriteria` += `Password string`; `InvadableWorld` += `PasswordHash string`.
- `coop.go`: `Sign` (ou o tipo que o store usa internamente) += `Password string`.
- Novo `password.go`: `HashWorldPassword` (D2).

## Arquivos a tocar

| Arquivo | Mudança |
|---|---|
| `internal/domain/invasion.go` | `InvaderCriteria.Password`, `InvadableWorld.PasswordHash` |
| `internal/domain/coop.go` | `Sign.Password` (ou struct interna equivalente) |
| `internal/domain/password.go` (novo) | `HashWorldPassword` (duplicada de session, D2) |
| `internal/domain/password_test.go` (novo) | mesma tabela de testes da spec 020 |
| `internal/service/invader.go` | bypass no loop de `Invade` (D1) |
| `internal/service/summoner.go` | bypass no `ListSigns`; `PostSummonSign` grava `Password` no sinal |
| `internal/service/*_test.go` | casos novos: bypass por senha, senha errada = caminho normal |
| `internal/adapters/sessionclient/*.go` (nome real a conferir) | `WorldDirectory` mapeia `password_hash` (D3) |
| `proto/matchmaking/v1/matchmaking.proto` | 3 campos novos (D4); `buf generate` |
| `internal/transport/grpc_server.go` | 3 handlers passam o campo novo |
| testes bufconn existentes | casos e2e de bypass |

`cmd/server/main.go` — sem mudança.

## Testes

- Baseline `go test ./services/matchmaking-service/... -race` verde antes.
- `domain`: `HashWorldPassword` (mesma tabela da spec 020). `Eligible`/`CoopEligible` **sem mudança** nos testes existentes (prova de que não regrediram).
- `service`: `Invade` com senha batendo mundo fora de faixa → injeta; senha vazia/errada → caminho de faixa normal (idêntico à spec 006); jogo sem `InvasionEnabled` → rejeita mesmo com senha certa. `ListSigns` espelha os mesmos três casos pro co-op.
- `transport` (bufconn): e2e do bypass em cada RPC.
- Cobertura ≥80% em `service/`/`domain/`.

## Sequência de tasks (preview)

1. Baseline verde.
2. Domínio: `HashWorldPassword` + campos (`InvaderCriteria`, `InvadableWorld`, `Sign`) + testes de domínio.
3. Service: bypass em `Invade` e `ListSigns`; `PostSummonSign` grava a senha do sinal; testes (bypass, senha errada, portão de entrada).
4. Adapter `WorldDirectory`: mapeia `password_hash` do session.
5. Proto (3 campos) + `buf generate` + transport (3 handlers) + testes bufconn.
6. `go test ./... -race` verde (baseline + novo); lint.

## Riscos / gotchas

- **Depende de `session/020` estar implementada primeiro** (o `password_hash` no `ListInvadableWorldsResponse` não existe até lá) — não dá pra terminar D3/D4 sem a contraparte session.
- **Hash duplicado (D2):** mesmo risco anotado na spec 020 — mudar o algoritmo exige tocar os dois serviços.
- **Índices de proto:** conferir antes de gerar (D4).
- Sem migration (tudo em memória, mesma vida útil dos tipos existentes).

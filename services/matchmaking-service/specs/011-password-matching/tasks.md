# Tasks: 011-password-matching

Follow-up #1 (senha/play-with-friends) do `host_world_dropin` (ADR-0023), lado matchmaking. Aditiva sobre `matchmaking/006`/`007`. Marcar `[x]` ao concluir. Ver `plan.md` para as decisões (D1–D5).

**Pré-requisito: `session/020-password-world-matching` implementada** (o `password_hash` no diretório de invadíveis precisa existir). ✅ Feita.

Baseline: `go test ./services/matchmaking-service/... -race` **verde** antes de começar.

## Domínio

- [x] **T1 — `HashWorldPassword`.** Novo `internal/domain/password.go` (duplicada de `session/020`, D2): SHA-256 hex; vazio→vazio. Mesma tabela de teste da spec irmã.
- [x] **T2 — campos.** `internal/domain/invasion.go`: `InvaderCriteria` += `Password string`; `InvadableWorld` += `PasswordHash string`. `internal/domain/coop.go`: `Sign` += `Password string`; `CoopCriteria` += `Password string` (decisão de implementação: em vez de um parâmetro à parte como o plan cogitava, `Password` entrou direto em `CoopCriteria` — reusa a mesma struct nos dois call sites de `PostSign`/`ListSigns` sem crescer assinaturas).

## Service

- [x] **T3 — bypass em `Invade`.** `internal/service/invader.go`: no loop de `worlds`, `matchesPassword` (D1) e `if !matchesPassword && !domain.Eligible(...)`. Testes: senha batendo mundo fora de faixa → injeta; senha vazia/errada → idêntico à spec 006; sem `InvasionEnabled` → rejeita mesmo com senha certa.
- [x] **T4 — `PostSummonSign` grava a senha do sinal.** `internal/service/summoner.go`: `Sign.Password = crit.Password` na criação.
- [x] **T5 — bypass em `ListSigns`.** `internal/service/summoner.go`: `matchesPassword := host.Password != "" && sg.Password != "" && host.Password == sg.Password`; `if matchesPassword || domain.CoopEligible(...)`. Testes: senha batendo sinal fora de faixa → aparece; senha vazia/errada → idêntico à spec 007.

## Adapter

- [x] **T6 — `WorldDirectory` mapeia o hash.** `internal/sessionclient/grpc_client.go`: `ListInvadableWorlds` copia `w.GetPasswordHash()` pro `domain.InvadableWorld.PasswordHash`.

## Proto + transport

- [x] **T7 — proto.** Índices confirmados livres (campo 5 em `InvadeRequest`, `PostSummonSignRequest`, `ListSignsRequest`, como estimado). Regenerado com `buf generate`.
- [x] **T8 — handlers.** `internal/transport/grpc_server.go`: os 3 handlers passam `req.GetPassword()` adiante. Testes bufconn: `TestInvade_Senha_ChegaAoServiceComoCriteria`, `TestListSigns_SenhaBateForaDaFaixa_AparecePeloBufconn`.

## Fechamento

- [x] **T9 — verde + lint.** `go test ./services/matchmaking-service/... -race`: todos os pacotes verdes; cobertura `service` 93.5%, `domain` 97.0%. `go vet` limpo (`golangci-lint` não instalado no ambiente desta sessão). `FindMatch`/`matchmaking/006`/`007` sem regressão (testes existentes de `Eligible`/`CoopEligible` intocados). Build de todo o monorepo (`go build ./...`) confirmado.
- [x] **T10 — coordenar com o par.** Formato dos 3 campos `password` (proto `string`, campo 5) documentado; spec 003 do Unity já desenhada com esses campos reservados.

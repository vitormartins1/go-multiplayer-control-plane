# Tasks: 005-battle-royale-placement

**Serviço:** leaderboard-service · **Spec:** [spec.md](./spec.md)

Plan-level: decisões e contratos por arquivo na spec (§Contratos de Interface). Fatia aditiva; introduz o dispatch por kind no consumer (hoje inexistente) + ramo `placement`. Ordem: domínio → consumer → testes → verificação.

## Domínio

- [x] **L1** — `internal/domain/leaderboard.go`: consts `BoardTopNFinishes = "top_n_finishes"`, `BoardSurvivedMs = "survived_ms"`, `BattleRoyaleTopNThreshold = 10`. Reusa `BoardWins`/`TotalBoard`/`Increment`/`ValidBoard`.

## Consumer (dispatch por kind)

- [x] **L2** — `cmd/consumer/main.go`: `makeHandler` passa a **despachar por kind** (`peekKind`): `placement` → `handlePlacement`; `win_loss_draw`/v1-legado → `handleWinLossDraw` (extração do corpo atual, idêntico); kind sem ranking (participation_event) / desconhecido → descarta.
- [x] **L3** — `handlePlacement`: highscore `survived_ms` (+ cada stat via `ValidBoard`); cumulative `wins` (`placement==1`), `top_n_finishes` (`<= threshold`), `<stat>_total`; fecha com `ApplyCumulative` (idempotente por matchID).

## Testes

- [x] **L4** — `cmd/consumer/main_test.go` (novo, fake store): `placement` 1º lugar → `wins`+`top_n_finishes`+highscore `survived_ms`+`kills_total`; colocação 7 → só `top_n_finishes`; 50 → nenhum; `dup` propagado; `win_loss_draw`/v1 sem regressão; kind sem ranking → descarta.

## Verificação

- [x] **L5** — `go build ./...`; `go test ./services/leaderboard-service/... -race -cover` (verde); `golangci-lint run ./services/leaderboard-service/...` (0 issues); `gofmt` só nos arquivos tocados.

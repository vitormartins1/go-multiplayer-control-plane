# Tasks: 003-metric-boards

**Serviço:** leaderboard-service · **Spec:** [spec.md](./spec.md) · **Plan:** [plan.md](./plan.md)

Mudança ampla e rasa (dimensão `board` em toda a cadeia). Ordem: domínio → storage → service → transport → consumer → testes → verificação. Marcar `[x]` conforme conclui.

## Domínio

- [x] **T1** — `internal/domain/leaderboard.go`: `LeaderboardService` e `LeaderboardStore` ganham `board string` (após `gameID`) em `Submit`, `ForceSubmit`/`ForceScore`, `Top`, `Around`, `Rank`, `Range`. Add `const BoardScore = "score"`, `var boardRE = ^[a-z][a-z0-9_]*$`, `ErrInvalidBoard`.

## Storage (Redis)

- [x] **T2** — `internal/storage/redis_store.go`: `key(gameID, board string)` = `prefix:gameID:board`; todos os métodos passam a receber e usar `board`. Semântica ZADD GT inalterada.

## Serviço

- [x] **T3** — `internal/service/leaderboard_service.go`: cada método ganha `board`; valida `boardRE.MatchString(board)` → `ErrInvalidBoard`; delega ao store. `clampLimit`/`clampAround` inalterados.

## Transporte (HTTP)

- [x] **T4** — `internal/transport/http_handler.go`: leituras (`GET /leaderboard`, `/leaderboard/me`, admin `/admin/games/{gameId}/leaderboard`) leem `board := query.Get("board")`, default `domain.BoardScore`; admin submit/force idem. Passa `board` ao service.
- [x] **T5** — `http_handler.go`: mapear `domain.ErrInvalidBoard` → `400` (código estável `invalid_board`).

## Consumer

- [x] **T6** — `cmd/consumer/main.go`: o struct do envelope ganha `Stats map[string]int64 \`json:"stats"\`` por resultado; por jogador, `Submit(score)` no `BoardScore` + para cada `(stat, val)` com nome casando `boardRE`, `Submit(val)` no board `stat` (pula nomes inválidos). Erros de storage propagam (retry SQS).

## Testes

- [x] **T7** — ajustar mocks/asserts existentes ao `board` novo: `mockStore` (service test) e chamadas `Submit`/`Top`/`Around` passam `BoardScore`.
- [x] **T8** — `internal/service/leaderboard_service_test.go`: `Submit`/`Top` repassam o `board`; `board` inválido → `ErrInvalidBoard`; `BoardScore` aceito.
- [x] **T9** — `internal/storage/*_test.go` (integração): boards `score` e `kills` do mesmo jogo **independentes**; highscore por board (ZADD GT); board inexistente → Top vazio.
- [x] **T10** — `internal/transport/http_handler_test.go`: `GET /leaderboard?board=kills` → board kills; **sem `board` → score** (compat); `board` inválido → 400.
- [x] **T11** — N/A: makeHandler em package main sem teste; coberto pelo smoke.

## Verificação final

- [x] **T12** — `go build ./services/leaderboard-service/...`, `go test ./services/leaderboard-service/... -race -cover` (≥80% em service), `golangci-lint`, `gofmt`.
- [x] **T13 (smoke)** — rebuild do leaderboard-consumer; via session `ReportResult` com `stats` (ex.: `kills`) → `GET /admin/games/{g}/leaderboard?board=kills` mostra o ranking de kills; sem `board` → score. Envelope sem `stats` → só o board score (retrocompat).
- [x] **T14** — Aprendizado (CLAUDE.md): provável **sem novidade de Go** (dimensão em parâmetro + default) — registrar só se algo não-óbvio surgir; senão, dizer que não houve.

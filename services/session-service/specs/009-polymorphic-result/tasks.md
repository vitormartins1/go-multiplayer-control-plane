# Tasks: 009-polymorphic-result

**Serviço:** session-service · **Spec:** [spec.md](./spec.md) · **Plan:** [plan.md](./plan.md)

Ordem: proto → domínio → serviço → publishers → transporte → testes → verificação. Marcar `[x]` conforme conclui.

## Proto (contrato + código gerado)

- [x] **T1** — `proto/session/v1/session.proto`: `ReportResultRequest` vira `{ match_id; oneof outcome { WinLossDrawResult win_loss_draw = 2; } }`; adicionar `WinLossDrawResult{ repeated PlayerOutcome players }` e `PlayerOutcome{ player_id, team_id, outcome, score, map<string,int64> stats }`; **remover** `message PlayerResult`. Rodar `buf generate` (de `services/session-service/`) e conferir `gen/...` atualizado.

## Domínio

- [x] **T2** — `internal/domain/session.go`: tipos `MatchOutcome{ Kind string; WinLossDraw *WinLossDrawResult }`, `WinLossDrawResult{ Players []PlayerOutcome }`, `PlayerOutcome{ PlayerID, TeamID, Outcome string; Score int64; Stats map[string]int64 }`; **remover** `PlayerResult`; atualizar as Ports `SessionService.ReportResult(ctx, matchID string, outcome MatchOutcome) error` e `ResultPublisher.Publish(ctx, gameID, matchID string, outcome MatchOutcome) error`. Const `OutcomeWinLossDraw = "win_loss_draw"` e valores `win`/`loss`/`draw` (vocabulário).
- [x] **T3** — `internal/domain/errors.go`: `ErrUnsupportedOutcome`, `ErrInvalidOutcome`.

## Serviço

- [x] **T4** — `internal/service/session_service.go`: `ReportResult` com a nova assinatura. Valida na ordem: kind == win_loss_draw (senão `ErrUnsupportedOutcome`); `WinLossDraw != nil`, `len(Players) > 0`, cada `Outcome ∈ {win,loss,draw}` (senão `ErrInvalidOutcome`); depois busca a sessão (`ErrSessionNotFound`), `publisher.Publish(...)`, `closeSession(CloseResult)`. Erros wrappados.

## Publishers (fan-out)

- [x] **T5** — `internal/publisher/sns_publisher.go`: `Publish(MatchOutcome)`; `envelopeVersion = 2`; `envelope` ganha `Kind`; `playerResult` ganha `Outcome`/`TeamID`/`Stats` **mantendo** `Won`/`Score` (`Won = outcome == "win"`). Itera `outcome.WinLossDraw.Players`.
- [x] **T6** — `internal/publisher/log_publisher.go`: `Publish(MatchOutcome)`; loga `outcome`/`team_id`/`stats` além de `score`/`won`.

## Transporte (gRPC)

- [x] **T7** — `internal/transport/grpc_server.go`: `ReportResult` extrai o `oneof` (`GetWinLossDraw()`), monta `domain.MatchOutcome`, chama o service; mapeia `ErrUnsupportedOutcome`/`ErrInvalidOutcome` → `codes.InvalidArgument`, `ErrSessionNotFound` → `codes.NotFound`.

## Testes

- [x] **T8** — `internal/service/session_service_test.go`: ajustar os testes que usam `PlayerResult`; novos casos — win/loss FFA fecha `CloseResult`; **draw**; **teams** (teamId); **stats**; kind não suportado → `ErrUnsupportedOutcome`; outcome inválido (sem players / valor inválido) → `ErrInvalidOutcome`; matchId inexistente → `ErrSessionNotFound`; publish falha → sessão não encerra. `-race`.
- [x] **T9** — `internal/publisher/sns_publisher_test.go`: envelope tem `schemaVersion=2`, `kind`, `outcome`/`teamId`/`stats` **e** `won`/`score` corretos (`won` derivado do outcome); erro de publish propaga.
- [x] **T10** — `internal/transport/grpc_server_test.go`: ajustar `ReportResult` para o `oneof`; win_loss_draw → sucesso; erros → `InvalidArgument`/`NotFound`.
- [x] **T11** — N/A: não há log_publisher_test.go; fallback simples coberto indiretamente.

## Verificação final

- [x] **T12** — `go build ./services/session-service/...`, `go test ./services/session-service/... -race -cover` (≥80% em service), `golangci-lint`, `gofmt`.
- [x] **T13 (smoke)** — subir session; `grpcurl` `ReportResult` com `win_loss_draw` (após um `CreateSession`) → fecha a sessão (`closed·result` na aba Sessions); conferir o envelope no log/consumidor (campos novos + `won`/`score`). Caso `draw` e kind inválido.
- [x] **T14** — Aprendizado (CLAUDE.md): se o par **`oneof` do protobuf ↔ tagged union do domínio** merecer nota (mapeamento wire↔Go), estender GO-LEARNINGS (ligar a #20); senão, dizer que não houve novidade.

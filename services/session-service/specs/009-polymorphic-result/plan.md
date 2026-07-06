# Plan: 009-polymorphic-result

**Serviço:** session-service · **Spec:** [spec.md](./spec.md) · **ADR:** 0023 (fatia 3) / 0020 (fan-out)

## Objetivo

Trocar `PlayerResult{Score,Won}` por um **outcome polimórfico** (`MatchOutcome`, kind `win_loss_draw`) no domínio, no `.proto` do `ReportResult` e no envelope de fan-out, mantendo o envelope **retrocompatível** (`won`/`score` preservados). Reusa a tagged union por ponteiro (GO-LEARNINGS #20) e o fan-out durável (ADR-0020). Nenhum cliente Go consome `ReportResult` (só o data plane, ainda não implementado), então o contrato pode mudar.

## Decisões técnicas

### D1 — Domínio: `MatchOutcome` (tagged union) substitui `PlayerResult`

```go
type MatchOutcome struct {
	Kind        string             // "win_loss_draw" (único no MVP)
	WinLossDraw *WinLossDrawResult // não-nil sse Kind == "win_loss_draw"
}
type WinLossDrawResult struct{ Players []PlayerOutcome }
type PlayerOutcome struct {
	PlayerID string
	TeamID   string // "" em FFA
	Outcome  string // "win" | "loss" | "draw"
	Score    int64
	Stats    map[string]int64 // métricas declaradas; pode ser nil
}
```
`PlayerResult` é **removido**. Ports mudam: `SessionService.ReportResult(ctx, matchID, MatchOutcome)` e `ResultPublisher.Publish(ctx, gameID, matchID, MatchOutcome)`.

### D2 — Proto: `oneof` por kind (discriminated union do protobuf)

```proto
message ReportResultRequest {
  string match_id = 1;
  oneof outcome {                 // novos kinds = novos campos no oneof
    WinLossDrawResult win_loss_draw = 2;
  }
}
message WinLossDrawResult { repeated PlayerOutcome players = 1; }
message PlayerOutcome {
  string player_id = 1;
  string team_id = 2;
  string outcome = 3;             // "win" | "loss" | "draw" (validado no service)
  int64 score = 4;
  map<string, int64> stats = 5;
}
// message PlayerResult REMOVIDO
```
- **`oneof`** é o análogo no proto da tagged union do domínio; o kind é implícito no campo setado. Regenerar com `buf generate` (de `services/session-service/`).
- `outcome` como **string** (não enum) para alinhar com o domínio e simplificar; o service valida `∈ {win,loss,draw}`. Enum fica como evolução.

### D3 — Fan-out: envelope v2, retrocompatível

Ambos os publishers (`SNS`, `Log`) passam a receber `MatchOutcome`. O envelope ganha `kind` (topo) e, por jogador, `outcome`/`teamId`/`stats`, **mantendo** `won`/`score`:
```go
type playerResult struct {
	PlayerID string           `json:"playerId"`
	Won      bool             `json:"won"`             // compat: outcome == "win"
	Score    int64            `json:"score"`           // compat
	Outcome  string           `json:"outcome"`         // novo
	TeamID   string           `json:"teamId,omitempty"`
	Stats    map[string]int64 `json:"stats,omitempty"`
}
```
`SchemaVersion` passa a `2`. `won := (p.Outcome == "win")`. Idempotência/durabilidade (SNS→SQS) inalteradas. O `Log` fallback loga os campos novos além dos antigos.

### D4 — Service: validação antes do estado

`ReportResult(ctx, matchID, outcome)`: (1) valida `outcome.Kind == "win_loss_draw"` (senão `ErrUnsupportedOutcome`); (2) valida estrutura — `WinLossDraw != nil`, `len(Players) > 0`, cada `Outcome ∈ {win,loss,draw}` (senão `ErrInvalidOutcome`); (3) busca a sessão (`ErrSessionNotFound`); (4) `publisher.Publish`; (5) `closeSession(CloseResult)`. Validação barata primeiro. **Não** valida contra o perfil do jogo.

### D5 — Transport: mapear `oneof` → domínio + erros

`grpc_server.ReportResult`: extrai o campo do `oneof` (`GetWinLossDraw()`), monta `MatchOutcome`; mapeia `ErrUnsupportedOutcome`/`ErrInvalidOutcome` → `codes.InvalidArgument`, `ErrSessionNotFound` → `codes.NotFound`.

### D6 — Erros de domínio

`ErrUnsupportedOutcome`, `ErrInvalidOutcome` em `internal/domain/errors.go`.

## Mudanças por camada (arquivos)

| Arquivo | Mudança |
|---|---|
| `proto/session/v1/session.proto` | `ReportResultRequest` oneof; `WinLossDrawResult`, `PlayerOutcome`; remove `PlayerResult`. `buf generate` regenera `gen/...` |
| `internal/domain/session.go` | tipos `MatchOutcome`/`WinLossDrawResult`/`PlayerOutcome`; remove `PlayerResult`; assinaturas `ReportResult`/`Publish` |
| `internal/domain/errors.go` | `ErrUnsupportedOutcome`, `ErrInvalidOutcome` |
| `internal/service/session_service.go` | `ReportResult` nova assinatura + validação |
| `internal/publisher/sns_publisher.go` | `Publish(MatchOutcome)`; envelope v2 (`kind`, `outcome`/`teamId`/`stats`, `won`/`score` mantidos) |
| `internal/publisher/log_publisher.go` | `Publish(MatchOutcome)`; loga campos novos |
| `internal/transport/grpc_server.go` | `ReportResult` handler: oneof→domínio, mapa de erros |
| testes | ajustar os que usam `PlayerResult`; novos casos (draw/teams/stats/retrocompat/erros) |

## Testes (mapeiam os casos da spec)

- **service** (`session_service_test.go`, mock publisher): win/loss FFA fecha `CloseResult`; **draw**; **teams** (teamId); **stats**; kind não suportado → `ErrUnsupportedOutcome`; outcome inválido → `ErrInvalidOutcome`; matchId inexistente → `ErrSessionNotFound`; falha de publish propaga (sessão não encerra).
- **sns_publisher** (fake snsClient): envelope tem `schemaVersion=2`, `kind`, `outcome`/`teamId`/`stats` **e** `won`/`score` corretos (`won` derivado); erro de publish propaga.
- **transport** (bufconn/unary): oneof `win_loss_draw` → sucesso; erros → `InvalidArgument`/`NotFound`.
- **log_publisher**: loga sem erro (fallback).

## Observabilidade

Sem trabalho novo: `fanout_publish_total` e o tracing seguem. O envelope maior não muda o pipeline.

## Riscos / fora do escopo

- **Contrato gRPC muda** — seguro (nenhum cliente Go; data plane Unity ainda não existe). A spec dataplane `006-match-result` será atualizada quando o gameplay for implementado.
- **Consumidores** (achievements/leaderboard/analytics) — seguem via `won`/`score` (retrocompat); leem os campos novos em specs próprias.
- Outros kinds (`placement`/`collective`/`role_event`), composição de times no matchmaking, validação contra o perfil — fora.

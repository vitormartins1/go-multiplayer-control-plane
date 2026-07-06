# Tasks: 007-outcome-events

**Serviço:** achievements-service · **Spec:** [spec.md](./spec.md) · **Plan:** [plan.md](./plan.md)

Fatia curta: domínio → serviço → consumer → testes → verificação. Marcar `[x]` conforme conclui.

## Domínio

- [x] **T1** — `internal/domain/achievement.go`: `MatchResult` ganha `Outcome string` (comentário: `win|loss|draw|""`; `Won` mantido para o fallback).

## Serviço

- [x] **T2** — `internal/service/achievement_service.go`: consts `eventMatchLost = "match_lost"`, `eventMatchDrawn = "match_drawn"` e vocabulário `outcomeWin/outcomeLoss/outcomeDraw`.
- [x] **T3** — `ApplyMatchResult`: por jogador, `match_played` sempre; `switch r.Outcome` → win→`match_won`, loss→`match_lost`, draw→`match_drawn`; `""`→fallback no `Won` (won→`match_won`); default (valor desconhecido)→só `match_played` (não falha).

## Consumer

- [x] **T4** — `cmd/consumer/main.go`: o struct do envelope ganha `Outcome string \`json:"outcome"\`` por resultado; monta `domain.MatchResult{PlayerID, Won, Outcome}`. Loop SQS/idempotência inalterados.

## Testes

- [x] **T5** — `internal/service/achievement_service_test.go` (mock repo): table-driven — `win`→{played,won}; `loss`→{played,lost}; `draw`→{played,drawn}; `outcome:"" & won:true`→{played,won}; `outcome:"" & won:false`→{played}; `outcome:"victory"`→{played}. Assere os `EventTypes` passados ao repo. Ajustar asserts existentes de `ApplyMatchResult` se cravam os eventos.
- [x] **T6** — idempotência: garantir que o teste de `dup=true` (reprocessa mesmo matchID) segue passando com o novo mapeamento.

## Verificação final

- [x] **T7** — `go build ./services/achievements-service/...`, `go test ./services/achievements-service/... -race -cover` (≥80% em service), `golangci-lint`, `gofmt`.
- [x] **T8 (smoke)** — rebuild do achievements-consumer; via session `ReportResult` (ou publicando um envelope v2 direto) com `outcome=draw` → conferir no log/estado do consumer que gera `match_drawn` (e `win`→`match_won`, `loss`→`match_lost`); envelope sem `outcome` (v1) → cai no `won` (retrocompat).
- [x] **T9** — Aprendizado (CLAUDE.md): provável **sem novidade de Go** (mapeamento simples + fallback já cobre padrões vistos) — registrar apenas se algo não-óbvio surgir; senão, dizer que não houve.

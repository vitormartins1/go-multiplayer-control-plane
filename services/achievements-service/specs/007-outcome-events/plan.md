# Plan: 007-outcome-events

**Serviço:** achievements-service · **Spec:** [spec.md](./spec.md) · **ADR:** 0023 (fatia 3, consumidor) / 0020

## Objetivo

Fazer o achievements traduzir o `outcome` (win/loss/draw) do envelope v2 em eventos `match_won`/`match_lost`/`match_drawn`, com fallback para `won` (retrocompat). A lógica de tradução mora no **service** (`ApplyMatchResult`); o consumer só carrega o campo novo. Sem migração, sem tocar `stats`/`teamId`.

## Decisões técnicas

### D1 — `domain.MatchResult` ganha `Outcome` (mantém `Won`)

```go
type MatchResult struct {
	PlayerID string
	Won      bool   // mantido para o fallback (envelope v1 / outcome ausente)
	Outcome  string // "win" | "loss" | "draw" | "" (ausente)
}
```

### D2 — Service: novos eventos + mapeamento com fallback

Consts (junto de `eventMatchPlayed`/`eventMatchWon`):
```go
eventMatchLost  = "match_lost"
eventMatchDrawn = "match_drawn"
// vocabulário do envelope
outcomeWin  = "win"
outcomeLoss = "loss"
outcomeDraw = "draw"
```
`ApplyMatchResult` — por jogador, `match_played` sempre, e:
```go
switch r.Outcome {
case outcomeWin:  events = append(events, eventMatchWon)
case outcomeLoss: events = append(events, eventMatchLost)
case outcomeDraw: events = append(events, eventMatchDrawn)
case "": // fallback retrocompat: envelope antigo/sem outcome
	if r.Won { events = append(events, eventMatchWon) }
default: // valor desconhecido -> só match_played (não falha)
}
```

### D3 — Consumer: repassa o `outcome`

`cmd/consumer/main.go` — o struct do envelope ganha `Outcome string json:"outcome"` por resultado; `domain.MatchResult{PlayerID, Won, Outcome}`. O `won` segue lido (fallback). Nada mais muda no loop SQS/idempotência.

## Mudanças por camada (arquivos)

| Arquivo | Mudança |
|---|---|
| `internal/domain/achievement.go` | `MatchResult` += `Outcome string` |
| `internal/service/achievement_service.go` | consts `eventMatchLost`/`eventMatchDrawn` + vocabulário; `ApplyMatchResult` mapeia outcome→evento com fallback |
| `cmd/consumer/main.go` | envelope struct += `Outcome`; monta `MatchResult` com `Outcome` |
| testes | service (table-driven outcome→eventos); ajustar asserts existentes se cravam os eventos |

## Testes (mapeiam os casos da spec)

- **service** (`achievement_service_test.go`): table-driven — `win`→{played,won}; `loss`→{played,lost}; `draw`→{played,drawn}; `outcome:"" & won:true`→{played,won}; `outcome:"" & won:false`→{played}; `outcome:"victory"`→{played}. Verificar os `EventTypes` passados ao repo (mock). Idempotência (`dup`) já coberta — manter.
- **consumer** (`cmd/consumer`, se `makeHandler` for testável no pacote main): desserializa envelope com `outcome` e repassa; senão, coberto pelo service + smoke.

## Observabilidade

Sem trabalho novo: métricas do consumer (processados/dup/erro) e idempotência seguem. Os eventos novos aparecem como qualquer outro no fluxo de conquistas.

## Riscos / fora do escopo

- **Retrocompat garantida por construção:** `outcome` ausente cai no `won`; conquistas de `match_won` seguem idênticas. Nenhum consumidor existente quebra.
- `stats`/`teamId`, leaderboard/analytics, popular catálogo — fora (spec/dados próprios).

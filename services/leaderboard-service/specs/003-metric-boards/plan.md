# Plan: 003-metric-boards

**Serviço:** leaderboard-service · **Spec:** [spec.md](./spec.md) · **ADR:** 0023 (fatia 3, consumidor) / 0020

## Objetivo

Adicionar a dimensão **`board`** em toda a cadeia (domain → storage → service → transport → consumer): de 1 ranking por jogo para N (`score` + um por stat). Mesma semântica highscore (`ZADD GT`). `board` default = `score` → API e comportamento atuais preservados.

## Decisões técnicas

### D1 — `board` como parâmetro (após `gameID`) em todos os métodos

As Ports `LeaderboardService` e `LeaderboardStore` ganham `board string`: `Submit`, `ForceSubmit`/`ForceScore`, `Top`, `Around`, `Rank`, `Range`. `Entry` inalterado. Vocabulário no domínio:
```go
const BoardScore = "score" // board default
var boardRE = regexp.MustCompile(`^[a-z][a-z0-9_]*$`) // mesmo do scoringStats (game-registry)
```
Erro novo `ErrInvalidBoard` (nome fora do regex).

### D2 — Storage: key por `(gameID, board)`

`key(gameID, board) = prefix + ":" + gameID + ":" + board`. Todos os métodos passam a receber `board`. **Reset dos rankings de dev** (Redis efêmero) — as keys antigas (`prefix:gameID`) ficam órfãs; sem migração.

### D3 — Service: valida o board, delega

Cada método valida `boardRE.MatchString(board)` → `ErrInvalidBoard` se não casar; senão delega ao store. `clampLimit`/`clampAround` inalterados. `BoardScore` é válido pelo regex.

### D4 — Transport: `?board` opcional, default `score`

Nas rotas de leitura (`GET /leaderboard`, `/leaderboard/me`, admin `/admin/games/{gameId}/leaderboard`): `board := r.URL.Query().Get("board"); if board == "" { board = domain.BoardScore }`. Passa ao service. `ErrInvalidBoard` → `400`. Admin submit/force idem (`board` no corpo ou query, default `score`).

### D5 — Consumer: submete `score` + cada stat ao seu board

O envelope ganha `Stats map[string]int64` por resultado. Por jogador: `Submit(score)` no board `BoardScore` (como hoje) e, para cada `(stat, valor)` cujo **nome casa `boardRE`**, `Submit(valor)` no board `stat`. Nomes inválidos são **pulados** (não geram `ErrInvalidBoard` no fluxo; o game-registry já valida `scoringStats`, isto é defesa). Erros de **storage** propagam → retry SQS (ZADD GT é idempotente).

## Mudanças por camada (arquivos)

| Arquivo | Mudança |
|---|---|
| `internal/domain/leaderboard.go` | Ports += `board`; `BoardScore`, `boardRE`, `ErrInvalidBoard` |
| `internal/storage/redis_store.go` | `key(gameID, board)`; todos os métodos += `board` |
| `internal/service/leaderboard_service.go` | métodos += `board` + validação `ErrInvalidBoard` |
| `internal/transport/http_handler.go` | `?board` (default `score`) nas leituras/admin; `ErrInvalidBoard` → 400 |
| `cmd/consumer/main.go` | envelope += `stats`; submete `score` + stats válidos por board |
| testes | mockStore/mocks += `board`; ajustar asserts; novos casos (boards independentes, default score, stats no consumer) |

## Testes (mapeiam os casos da spec)

- **service** (`leaderboard_service_test.go`, mockStore): `Submit`/`Top` repassam o `board`; `board` inválido → `ErrInvalidBoard`; `BoardScore` aceito.
- **storage** (integração — mesmo harness dos testes atuais): boards `score` e `kills` do mesmo jogo são independentes; highscore por board (ZADD GT).
- **transport** (`httptest`): `GET /leaderboard?board=kills` → board kills; **sem `board` → score**; `board` inválido → 400.
- **consumer** (se `makeHandler` testável): resultado com `stats` → Submit no `score` + em cada board; sem `stats` → só `score` (retrocompat).

## Observabilidade

Sem trabalho novo: métricas do consumer e do serviço seguem. Mais chamadas `Submit` por resultado (uma por board), mesma natureza.

## Riscos / fora do escopo

- **Muitos call sites** (`board` em toda a cadeia) — mecânico, mas amplo; o default `score` blinda a API pública.
- **Reset de rankings** em dev (key scheme) — Redis efêmero, aceitável.
- Acumulativo (wins/total), K/D, `outcome`/`teamId`, validar boards pelo perfil, analytics — fora (spec/fatia própria).

# Tasks: 005-battle-royale-placement

**Serviço:** analytics-service · **Spec:** [spec.md](./spec.md)

Plan-level: decisões e contratos por arquivo na spec (§Contratos de Interface). Espelha o caminho de participação (spec 004): novo caminho no pool + parse + rollup. Ordem: migration → domínio → repo → pool → consumer → transport → testes → verificação.

## Persistência

- [x] **N1** — `migrations/004_placement_rollup.sql`: `game_placement_rollup(game_id PK, winners, top_n, rest, entries, sum_survived_ms)`. Reusa `processed_matches` (sem tabela de dedupe nova).

## Domínio

- [x] **N2** — `internal/domain/analytics.go`: `PlacementResult{MatchID, GameID, Entries[]}`, `PlacementEntry{Placement int; SurvivedMs int64}`, `PlacementDistribution{Winners, TopN, Rest, Entries int64; AvgSurvivedMs float64}`, const `BattleRoyaleTopNThreshold = 10`; `MatchRepository.ApplyPlacement`; `Ingestor.SubmitPlacement`; `GameAnalytics.Placement *PlacementDistribution`.

## Repositório

- [x] **N3** — `postgres_analytics.go`: `ApplyPlacement` (tx + guarda `processed_matches` + upsert do rollup; baldes exclusivos winner/top_n/rest); `GameAnalytics` lê o rollup → `out.Placement` (nil sem linha; avg no read).

## Pool

- [x] **N4** — `service/pool.go`: `job.placement`; `process` despacha placement→`ApplyPlacement`; `SubmitPlacement`.

## Consumer

- [x] **N5** — `internal/consumer/consumer.go`: `parsePlacement` + `case kind == "placement"` (antes do ramo matchId) → `SubmitPlacement`.

## Transport

- [x] **N6** — `internal/transport/http_handler.go`: `placementDTO` + campo `placement,omitempty` no `gameAnalyticsDTO`; mapeia `ga.Placement`.

## Testes

- [x] **N7** — consumer: `parsePlacement` lê entradas; `peekKind` dispatch inclui placement.
- [x] **N8** — pool: `SubmitPlacement` roteia para `ApplyPlacement` (fakeRepo ganha o método).
- [x] **N9** — repo (integração, testcontainers): `ApplyPlacement` agrega histograma/sobrevivência (partição fecha); idempotente por matchID (reentrega não dobra); jogo sem placement → `Placement` nil. Migration 004 no init.

## Verificação

- [x] **N10** — `go build ./...`; `go test ./services/analytics-service/... -race`; `go test -tags=integration ./services/analytics-service/internal/repository/`; `golangci-lint run ./services/analytics-service/...` (0 issues); `gofmt` só nos arquivos tocados.

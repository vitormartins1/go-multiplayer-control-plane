# Tasks: 010-battle-royale-placement

**Serviço:** achievements-service · **Spec:** [spec.md](./spec.md)

Plan-level: as decisões técnicas e os contratos por arquivo estão na spec (§Contratos de Interface). Fatia aditiva que espelha o ramo `participation_event` (spec 009). Ordem: domínio → serviço → consumer → testes → verificação.

## Domínio (política pura)

- [x] **A1** — `internal/domain/placement.go`: consts `EventBattleRoyalePlayed/Win/TopN`, `BattleRoyaleTopNThreshold = 10`, `StatSurvivedMs = "survived_ms"`; tipo `PlacementEntry{PlayerID string; Placement int; SurvivedMs int64; Stats map[string]int64}`; função pura `PlacementEventTypes(placement int) []string` (played sempre; +win se `==1`; +top_n se `<= threshold`).

## Serviço

- [x] **A2** — `internal/service/achievement_service.go`: `ApplyPlacementResult(ctx, gameID, matchID, entries)` — monta `[]PlayerMatchInput` (event types via `PlacementEventTypes`, stats via `withSurvivedMs`) e delega a `repo.ApplyMatchResult` (idempotente por matchID). Helper `withSurvivedMs` injeta `survived_ms` sem sobrescrever.

## Consumer (dispatch por kind)

- [x] **A3** — `cmd/consumer/main.go`: no `makeHandler`, `case head.Kind == "placement":` **antes** do ramo `win_loss_draw || matchId != ""` (placement também tem matchId); `handlePlacement` desserializa `entries[]` e chama `ApplyPlacementResult`. Atualiza o comentário do `makeHandler`.

## Testes

- [x] **A4** — `internal/domain/placement_test.go`: tabela de `PlacementEventTypes` (1 → played/win/top_n; 7 → played/top_n; 50 → played).
- [x] **A5** — service test: `ApplyPlacementResult` traduz event types por entrada e injeta `survived_ms` nos stats (mock captura os `PlayerMatchInput`); `survived_ms` já presente não é sobrescrito; `dup` propagado.
- [x] **A6** — `cmd/consumer/main_test.go`: `kind=placement` roteia para o caminho de placement (captura os players → contém `battle_royale_win`); `win_loss_draw`/`participation_event`/v1/desconhecido sem regressão.

## Verificação

- [x] **A7** — `go build ./...`; `go test ./services/achievements-service/... -race -cover` (verde, ≥ 80% service/domain); `golangci-lint run ./services/achievements-service/...` (0 issues); `gofmt` só nos arquivos tocados.

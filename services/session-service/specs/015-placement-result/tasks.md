# Tasks: 015-placement-result

**Serviço:** session-service · **Spec:** [spec.md](./spec.md) · **Plan:** [plan.md](./plan.md)

Ordem: proto → domínio → validação/carimbo → transport → publisher → testes → verificação. Marcar `[x]` conforme conclui.

## Proto (regen com buf)

- [x] **T1** — Em `proto/session/v1/session.proto`: adicionar `PlacementResult placement = 3;` ao `oneof outcome` do `ReportResultRequest`; msgs `PlacementResult{ repeated PlacementEntry entries = 1; }` e `PlacementEntry{ string player_id = 1; string team_id = 2; int32 placement = 3; int64 survived_ms = 4; map<string,int64> stats = 5; }` (comentário: outcome do battle_royale, ADR-0023/BR2).
- [x] **T2** — Regenerar: `cd services/session-service && PATH="$(go env GOPATH)/bin:$PATH" buf generate`. Conferir que o `.pb.go` ganhou `GetPlacement()`/`PlacementResult`/`PlacementEntry` e **não** editar à mão.

## Domínio (ADR-0002)

- [x] **T3** — Em `internal/domain/session.go`: const `OutcomePlacement = "placement"` (junto de `OutcomeWinLossDraw`); tipos `PlacementResult{Entries []PlacementEntry}` e `PlacementEntry{PlayerID, TeamID string; Placement int; SurvivedMs int64; Stats map[string]int64}` (doc: outcome ordinal do battle_royale); ponteiro `Placement *PlacementResult` em `MatchOutcome` (+ atualizar o comentário do tagged union).

## Serviço (validação + carimbo)

- [x] **T4** — Em `internal/service/session_service.go`: `validateOutcome` passa a **despachar por `o.Kind`** (`switch`): `OutcomeWinLossDraw` mantém a validação atual; `OutcomePlacement` → `Placement != nil && len(Entries) > 0` (senão `ErrInvalidOutcome`), e cada entrada `Placement >= 1` e `SurvivedMs >= 0` (senão `ErrInvalidOutcome`); default → `ErrUnsupportedOutcome`. Empates/não-contiguidade permitidos.
- [x] **T5** — `session_service.go`: `stampTeams` carimba também o `Placement` — para cada `PlacementEntry`, `TeamID = teamByPlayer[PlayerID]`. Manter o ramo `WinLossDraw`; estruturar por ponteiro não-nil (dois `if` ou `switch`).

## Transporte (gRPC)

- [x] **T6** — Em `internal/transport/grpc_server.go`: `toMatchOutcome` ganha `if pl := req.GetPlacement(); pl != nil { ... }` — mapeia `entries` para `[]domain.PlacementEntry` (`int32→int` no `Placement`), devolve `MatchOutcome{Kind: domain.OutcomePlacement, Placement: &domain.PlacementResult{Entries: ...}}`. `win_loss_draw` e o `oneof` vazio (→ `ErrUnsupportedOutcome`) inalterados.

## Publisher (fan-out por kind)

- [x] **T7** — Em `internal/publisher/sns_publisher.go`: `Publish` passa a despachar por `outcome.Kind` (não mais `outcome.WinLossDraw.Players` direto). `placement` → `placementEnvelope{ SchemaVersion: envelopeVersion, Kind: "placement", MatchID, GameID, Entries []placementEntryDTO }` (`playerId`/`teamId`/`placement`/`survivedMs`/`stats`), marshal e publish (reusa `publishTotal`). const `kindPlacement = "placement"`.
- [x] **T8** — Em `internal/publisher/log_publisher.go`: `Publish` ganha o ramo `placement` — loga por entrada (`player_id`/`team_id`/`placement`/`survived_ms`/`stats`). Manter o ramo `win_loss_draw`.

## Testes (mapeiam os "Casos de Teste Obrigatórios" da spec)

- [x] **T9** — `internal/service/session_service_test.go`: `ReportResult` `placement` válido → carimba `squad-K`/vazio, publica (fake publisher captura `Kind=="placement"`), encerra (`CloseResult`); `entries` vazio / `placement<1` / `survivedMs<0` → `ErrInvalidOutcome`; empate (dois `placement=1`) aceito; `matchId` inexistente → `ErrSessionNotFound`; **regressão** `win_loss_draw`. Nomenclatura `TestReportResult_Placement_*`.
- [x] **T10** — `internal/publisher/sns_publisher_test.go`: `Publish` de `placement` → SNS fake recebe JSON com `kind:"placement"` e `entries[]` (unmarshal + asserção de um campo por entrada); `win_loss_draw` intacto.
- [x] **T11** — `internal/transport/grpc_server_test.go`: `toMatchOutcome` do `oneof placement` → domínio (entries, `placement` int, `survivedMs`, `stats`); `oneof` vazio → `MatchOutcome{}` (unsupported).

## Verificação final

- [ ] **T12 (automático)** — `go build ./...`; `go test ./services/session-service/... -race -cover` (verde, service/domain ≥ 80%); `golangci-lint run ./services/session-service/...` (0 issues); `gofmt`.
- [ ] **T13 (smoke manual — pendente do Vitor)** — subir docker-compose; via cliente de teste, `ReportResult` com `placement` de um lobby BR → conferir a sessão encerrada e o envelope `kind=placement` no tópico (log do consumidor / LocalStack).
- [ ] **T14** — Loop de aprendizado (CLAUDE.md): avaliar se há algo novo de Go. Provável **nada novo** (novo ponteiro no tagged union + dispatch por kind, já documentado). Se nada, dizer e não inventar.

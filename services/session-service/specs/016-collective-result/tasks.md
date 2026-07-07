# Tasks: 016-collective-result

**Serviço:** session-service · **Spec:** [spec.md](./spec.md) · **Plan:** [plan.md](./plan.md)

Ordem: proto+buf → domínio → service → transport → publishers → testes → verificação. Sem migration.

## Proto (buf)

- [x] **T1** — `proto/session/v1/session.proto`: no `oneof outcome` do `ReportResultRequest`, `CollectiveResult collective = 4;`. Adicionar `message CollectiveResult{ string outcome = 1; repeated CollectivePlayer players = 2; }` e `message CollectivePlayer{ string player_id = 1; string team_id = 2; map<string,int64> stats = 3; }` com doc-comments (ficha 04/ADR-0023). Rodar `PATH="$(go env GOPATH)/bin:$PATH" buf generate` de `services/session-service/`; conferir `gen/session/v1/*.pb.go` atualizado (não editar à mão).

## Domínio (ADR-0002)

- [x] **T2** — `internal/domain/session.go`: consts `OutcomeCollective = "collective"`, `MissionSuccess = "MISSION_SUCCESS"`, `MissionFailure = "MISSION_FAILURE"` (junto de `OutcomeWinLossDraw`/`OutcomePlacement`). Adicionar `Collective *CollectiveResult` ao `MatchOutcome` (+ atualizar comentário do invariante e do campo `Kind`). Tipos `CollectiveResult{ Outcome string; Players []CollectivePlayer }` e `CollectivePlayer{ PlayerID, TeamID string; Stats map[string]int64 }` com doc-comment.

## Serviço

- [x] **T3** — `internal/service/session_service.go`: `case domain.OutcomeCollective` em `validateOutcome` (map de pacote `missionOutcomes = {MISSION_SUCCESS, MISSION_FAILURE → true}`; `Collective==nil || len(Players)==0` → `ErrInvalidOutcome`; `outcome` fora do vocabulário → `ErrInvalidOutcome`; **não** valida valores de `stats`).
- [x] **T4** — `session_service.go`: ramo `if o.Collective != nil` em `stampTeams` (sobrescreve `TeamID` de cada `Players[i]` com `teamByPlayer`).

## Transporte

- [x] **T5** — `internal/transport/grpc_server.go`: ramo `if c := req.GetCollective(); c != nil` em `toMatchOutcome` — mapeia `outcome` + `players[]` (`playerId`/`teamId`/`stats`) para `domain.MatchOutcome{Kind: OutcomeCollective, Collective: ...}`.

## Publishers

- [x] **T6** — `internal/publisher/sns_publisher.go`: const `kindCollective = "collective"`; `collectiveEnvelope{ SchemaVersion, Kind, MatchID, GameID, Outcome string, Players []collectivePlayerDTO }` + `collectivePlayerDTO{ PlayerID, TeamID string (omitempty), Stats map (omitempty) }`; `publishCollective` (reusa `s.publish`); `case domain.OutcomeCollective` em `Publish`.
- [x] **T7** — `internal/publisher/log_publisher.go`: ramo `if outcome.Collective != nil` em `Publish` (loga `kind`/`matchId`/`gameId`/`outcome` + um log por jogador com `player_id`/`team_id`/`stats`).

## Testes

- [x] **T8** — `internal/service/session_service_test.go`: `collective` válido (SUCCESS) → carimba `team-1`, publica `kind=collective` (fake publisher), fecha `CloseResult`; `players` vazio → `ErrInvalidOutcome`; `outcome` inválido → `ErrInvalidOutcome`; SUCCESS e FAILURE aceitos; `matchId` inexistente → `ErrSessionNotFound`; **regressão** win_loss_draw/placement/participation_event. `-race -cover` (≥80%).
- [x] **T9** — `internal/publisher/sns_publisher_test.go`: envelope `collective` serializa `kind`/`matchId`/`gameId`/`outcome`/`players[]` (fake SNS). Se houver `log_publisher_test.go`, cobrir o ramo collective.
- [x] **T10** — `internal/transport/grpc_server_test.go`: `toMatchOutcome` do `oneof` `collective` → domínio (outcome/players/stats); `oneof` vazio → Kind vazio.

## Verificação final

- [x] **T11 (automático)** — `go build ./...`; `go test ./services/session-service/... -race -cover` (≥80% service+domain); `golangci-lint run ./services/session-service/...` 0 issues; `gofmt` limpo nos arquivos tocados (o `.pb.go` gerado não conta).
- [x] **T12** — Loop de aprendizado (CLAUDE.md): **nada novo de Go** esperado (tagged-union por ponteiro + dispatch por kind + oneof↔union já documentados #20/#22). Confirmar e não inventar.

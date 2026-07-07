# Plan: 015-placement-result

**Serviço:** session-service · **Spec:** [spec.md](./spec.md) · **ADR:** 0023 · **Roadmap:** BR2

## Objetivo

Adicionar o kind de outcome **`placement`** ao `MatchOutcome`, reusando o fluxo terminal `ReportResult → stampTeams → publish → closeSession` e o envelope de fan-out por `kind`. Mudança **aditiva** (tagged union por ponteiro): novo ponteiro no domínio + um ramo `placement` em cada ponto que despacha por kind (validação, carimbo, transport, publisher). Sem RPC nova, sem store, sem migration. Exige `buf generate` (extensão do `oneof`).

## Decisões técnicas

### D1 — Proto: novo campo no `oneof outcome` (retrocompat)

- `ReportResultRequest.outcome` ganha `PlacementResult placement = 3;` (tag 3, o `win_loss_draw` fica em 2). Novo campo no oneof = **retrocompatível** (clientes do `win_loss_draw` intactos).
- Msgs novas: `PlacementResult{ repeated PlacementEntry entries = 1; }`, `PlacementEntry{ string player_id = 1; string team_id = 2; int32 placement = 3; int64 survived_ms = 4; map<string,int64> stats = 5; }`.
- **`buf generate`** dentro do serviço (`PATH="$(go env GOPATH)/bin:$PATH" buf generate`). **Nunca** editar o `.pb.go` à mão (gotcha do `go_package`).

### D2 — Domínio: espelha o `WinLossDraw` (tagged union por ponteiro)

- `OutcomePlacement = "placement"`; `MatchOutcome.Placement *PlacementResult`; `PlacementResult{Entries []PlacementEntry}`; `PlacementEntry{PlayerID, TeamID string; Placement int; SurvivedMs int64; Stats map[string]int64}`. `Placement` int (não int32) no domínio — o transport converte do int32 do proto.

### D3 — Validação: dispatch por kind, estrutural mínima

- `validateOutcome` passa a **despachar por `o.Kind`** (`switch`): `win_loss_draw` mantém a validação atual; `placement` valida: `Placement != nil && len(Entries) > 0`; cada entrada `Placement >= 1` e `SurvivedMs >= 0`. Empates e não-contiguidade **permitidos** (server autoritativo). Kind desconhecido → `ErrUnsupportedOutcome`.

### D4 — `stampTeams`: ramo `placement`

- `stampTeams` passa a carimbar também o ponteiro `Placement`: para cada `PlacementEntry`, `TeamID = teamByPlayer[PlayerID]` (vazio em solo/FFA). Mesma semântica do `win_loss_draw` (session é dono do rótulo). Estruturo com `switch` no kind ou dois `if` por ponteiro — manter legível.

### D5 — Transport: `toMatchOutcome` ganha o ramo `placement`

- `if pl := req.GetPlacement(); pl != nil { ... return MatchOutcome{Kind: OutcomePlacement, Placement: &PlacementResult{Entries: ...}} }`. Converte `int32→int` no `Placement`. `oneof` vazio segue `MatchOutcome{}` → `ErrUnsupportedOutcome`.

### D6 — Publisher: `Publish` despacha por kind; `placementEnvelope`

- `SNS.Publish` e `Log.Publish` passam a **despachar por `outcome.Kind`** em vez de assumir `WinLossDraw`. `placement` → novo `placementEnvelope{ SchemaVersion, Kind:"placement", MatchID, GameID, Entries []placementEntryDTO }` (`playerId`/`teamId`/`placement`/`survivedMs`/`stats`). Reusa `publishTotal` e a mesma chamada SNS. Log fallback loga por entrada.
- **Cuidado:** hoje `SNS.Publish` faz `outcome.WinLossDraw.Players` direto (nil-panic se placement). O dispatch por kind fecha isso.

## Mudanças por camada (arquivos)

| Arquivo | Mudança |
|---|---|
| `proto/session/v1/session.proto` | `placement` no oneof; msgs `PlacementResult`/`PlacementEntry`; `buf generate` |
| `internal/domain/session.go` | `OutcomePlacement`; `MatchOutcome.Placement`; tipos `PlacementResult`/`PlacementEntry` |
| `internal/service/session_service.go` | `validateOutcome` (dispatch por kind); `stampTeams` (ramo placement) |
| `internal/transport/grpc_server.go` | `toMatchOutcome` (ramo placement) |
| `internal/publisher/sns_publisher.go` | `Publish` dispatch por kind; `placementEnvelope` |
| `internal/publisher/log_publisher.go` | `Publish` ramo placement |
| store / migrations / RPC nova | **nenhum** |

## Testes (mapeiam os casos da spec)

- **service** (`session_service_test.go`): `placement` válido → carimba squad/vazio, publica (fake publisher captura kind=placement), encerra (`CloseResult`); `entries` vazio / `placement<1` / `survivedMs<0` → `ErrInvalidOutcome`; empate aceito; `matchId` inexistente → `ErrSessionNotFound`; regressão `win_loss_draw`.
- **publisher** (`sns_publisher_test.go`): envelope `placement` serializa `kind`/`entries[]` (SNS fake, unmarshal e asserção); `win_loss_draw` intacto.
- **transport** (`grpc_server_test.go`): `toMatchOutcome` do `oneof placement` (int32→int, entries); oneof vazio → unsupported.
- `-race -cover`, service/domain ≥ 80%.

## Observabilidade

Sem métrica nova: reusa `fanout_publish_total{result}`. A distinção `placement` no funil (contagem por kind) é surfacing de BR4/analytics — não bloqueia.

## Riscos / fora do escopo

- **Risco médio-baixo:** o ponto sensível é o `SNS.Publish` que hoje assume `WinLossDraw` (nil-deref se placement chegar sem o dispatch). O dispatch por kind + teste de publisher fecham isso. Proto regen com `buf` (não sed).
- Sinks (BR3), tracking ao vivo, console (BR4) — fora daqui.

# Plan: 016-collective-result

**Serviço:** session-service · **Spec:** [spec.md](./spec.md) · **ADR:** 0023 · **Roadmap:** C2 (núcleo)

## Objetivo

Adicionar o kind de outcome **`collective`** ao `MatchOutcome`, reusando o fluxo terminal `ReportResult → stampTeams → publish → closeSession` e o envelope de fan-out por `kind`. Mudança **aditiva** (tagged union por ponteiro): novo ponteiro no domínio + um ramo `collective` em cada ponto que despacha por kind (validação, carimbo, transport, publishers). Sem RPC nova, sem store, sem migration. Exige `buf generate` (extensão do `oneof`).

## Decisões técnicas

### D1 — Desfecho no nível da SESSÃO (o ponto novo)

- `CollectiveResult{ Outcome string; Players []CollectivePlayer }` — `Outcome` (MISSION_SUCCESS/FAILURE) é um campo do **resultado**, não de cada jogador. `CollectivePlayer{ PlayerID, TeamID, Stats }` **não** tem Outcome/Won/Score/placement. É a diferença estrutural dos três kinds atuais (todos por-participante). Registrado na spec.

### D2 — `buf generate`, nunca editar o `.pb.go`

- `oneof` ganha `CollectiveResult collective = 4;` + as duas mensagens. Regenerar de `services/session-service/` com `PATH="$(go env GOPATH)/bin:$PATH" buf generate`. Gotcha do `go_package` já conhecido.

### D3 — Validação estrutural mínima e consistente

- `validateOutcome` ganha `case OutcomeCollective`: `Collective != nil` e `len(Players) >= 1`; `Outcome ∈ {MISSION_SUCCESS, MISSION_FAILURE}` (map `missionOutcomes`, estilo do vocabulário do `win_loss_draw`). **Não** valida valores de `stats` — coerente com o `placement` (que só checa `survivedMs`, um campo dedicado, não o mapa `stats`) e com a filosofia "server autoritativo". (Diverge do rascunho do roadmap que mencionou "stats não-negativos" — a escolha consistente com os kinds existentes é não range-checar o mapa.)

### D4 — Reuso total do fluxo terminal

- `stampTeams` ganha o ramo `o.Collective != nil`. `ReportResult` não muda (já é agnóstico: valida → carimba → publica → fecha). `SNS.Publish`/`Log.Publish` ganham o `case`/ramo `collective`. Envelope `collectiveEnvelope{ schemaVersion, kind, matchId, gameId, outcome, players[] }`, `kindCollective = "collective"`.

## Mudanças por camada (arquivos)

| Arquivo | Mudança |
|---|---|
| `proto/session/v1/session.proto` | `oneof` += `CollectiveResult collective = 4`; mensagens `CollectiveResult`/`CollectivePlayer`; `buf generate` regenera `gen/...` |
| `internal/domain/session.go` | consts `OutcomeCollective`/`MissionSuccess`/`MissionFailure`; `Collective *CollectiveResult` no `MatchOutcome`; tipos `CollectiveResult`/`CollectivePlayer`; atualizar comentário do `MatchOutcome` |
| `internal/service/session_service.go` | `case OutcomeCollective` em `validateOutcome` (+ map `missionOutcomes`); ramo `Collective` em `stampTeams` |
| `internal/transport/grpc_server.go` | ramo `req.GetCollective()` em `toMatchOutcome` |
| `internal/publisher/sns_publisher.go` | `kindCollective`; `collectiveEnvelope`+`collectivePlayerDTO`; `publishCollective`; `case` em `Publish` |
| `internal/publisher/log_publisher.go` | ramo `Collective` em `Publish` |

**Nenhum erro de domínio novo, nenhuma RPC nova, nenhum store.**

## Testes (mapeiam os casos da spec)

- **service** (`session_service_test.go`): `collective` válido (SUCCESS) → carimba `team-1`, publica (fake publisher captura `kind=collective`), fecha `CloseResult`; `players` vazio / `outcome` inválido → `ErrInvalidOutcome`; SUCCESS e FAILURE aceitos; `matchId` inexistente → `ErrSessionNotFound`; **regressão** dos outros kinds. Cobertura ≥ 80%.
- **publisher** (`sns_publisher_test.go`): envelope `collective` serializa `kind`/`matchId`/`gameId`/`outcome`/`players[]` (fake SNS captura o body). **Log** (`log_publisher_test.go` se existir): loga o desfecho.
- **transport** (`grpc_server_test.go`): `toMatchOutcome` do `oneof` `collective` → domínio; `oneof` vazio → Kind vazio (→ `ErrUnsupportedOutcome` na validação).

## Observabilidade

Sem trabalho novo: reusa `fanout_publish_total` (agnóstico de kind) e o span do `ReportResult`. Sem exporter novo (sessão em memória).

## Riscos / fora do escopo

- **Risco baixo:** aditivo, mesmo padrão da BR2. Ponto de atenção: **não esquecer** um dos pontos de dispatch (validate/stamp/transport/SNS/Log) — as tasks cobrem os cinco; os testes de service+publisher pegam omissão. `buf generate` deve ser rodado (não editar `.pb.go`).
- Sinks (C3), console (C4) e as Aberturas ficam fora.

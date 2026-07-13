# Spec: 026-participation-event-stats

**Serviço:** session-service
**Status:** Done (2026-07-13) — validado ao vivo: `ReportParticipationEvent`
com `stats: {"deaths": 1}` no jogo "Achievements Test" (modo `mundo-vivo`)
desbloqueou o achievement `deaths_1` de verdade pela primeira vez.
**Data:** 2026-07-13
**Fatia:** pequena/aditiva — estende `ReportParticipationEvent` pro MESMO padrão
`map<string, int64> stats` já usado em `ReportResult` (win_loss_draw/placement/
collective, campo 5 dos respectivos protos); sem decisão técnica nova.

## Contexto

Achado ao validar o escopo de `analytics-service/011-raw-stat-capture`
(2026-07-13): `host_world_dropin` é o único arquétipo cujo evento reportado
pelo game server (`ReportParticipationEvent`) **não carrega `stats`** — nem no
proto, nem no domínio, nem no envelope do fan-out.

Isso contradiz três lugares que assumiam o contrário:
- `achievements-service/specs/018-participation-stats` (Status: implementada)
  só implementou o lado de RECEBER `stats` do envelope — o session-service
  nunca os envia, então na prática `Stats` chega sempre `nil` nesse caminho e
  nenhum achievement por stat de host_world_dropin jamais disparou de verdade
  (no-op silencioso, sem erro visível).
- `docs/game-models/host_world_dropin-dataplane-contract.md` (checklist da
  implementação Unity, linha "Preencher `stats` nos eventos... ✅ Feito")
  contradiz o próprio contrato RPC/envelope documentado ali mesmo, que não tem
  o campo — o checklist está desatualizado/otimista.
- `game-registry/011` (`HostWorldDropInParams.ScoringStats`) só declara QUAIS
  chaves de stat são válidas (vocabulário/UI) — nunca foi o transporte do
  valor; não fecha esta lacuna sozinho.

## Comportamento

- `ReportParticipationEventRequest` (proto) SHALL ganhar `map<string, int64>
  stats = 7` (campo 7, próximo livre), mesmo padrão de `ReportResultRequest`
  (`PlayerOutcome.stats`, `PlacementEntry.stats`, `CollectivePlayer.stats`).
  Ausente/vazio = comportamento atual inalterado (compat total com o data
  plane Unity de hoje, que não envia o campo).
- `domain.ParticipationEvent` SHALL ganhar `Stats map[string]int64`.
- `Service.ReportParticipationEvent` SHALL repassar `evt.Stats` ao publicar
  (`PublishParticipation`) — sem validação de conteúdo (mesma filosofia dos
  outros 3 kinds: o server é autoritativo sobre os valores).
- O envelope `participation_event` (fan-out) SHALL incluir `stats` (omitido se
  vazio, mesmo padrão `omitempty` dos outros envelopes).
- `docs/game-models/host_world_dropin-dataplane-contract.md` SHALL ser
  corrigido: o exemplo de RPC/envelope ganha `stats`; a linha 171 do checklist
  Unity é marcada como **pendente** (não mais "✅ Feito") até o data plane
  real confirmar que envia o campo — hoje o contrato control-plane nem
  aceitava, então não há como ter sido testado de ponta a ponta.

## Contratos de Interface

### Proto (`session/v1/session.proto`)
```protobuf
message ReportParticipationEventRequest {
  string session_id = 1;
  string player_id = 2;
  string role = 3;
  string event = 4;
  string outcome = 5;
  string event_id = 6;
  map<string, int64> stats = 7; // NOVO — métricas opcionais (kills, revives, ...)
}
```

### Domínio (session-service)
```go
type ParticipationEvent struct {
    PlayerID string
    Role     string
    Event    string
    Outcome  string
    EventID  string
    Stats    map[string]int64 // NOVO
}
```

### Envelope de fan-out (aditivo, `omitempty`)
```json
{
  "kind": "participation_event",
  "...": "...",
  "stats": {"kills": 2}
}
```

## Critérios de Aceite

1. `ReportParticipationEvent` com `stats` preenchido -> o envelope publicado
   no SNS carrega o mesmo `stats` (verificável no LocalStack/log publisher).
2. `stats` ausente (data plane Unity atual, que ainda não envia) -> envelope
   sem o campo `stats` — comportamento idêntico ao de hoje, zero regressão.
3. `achievements-consumer` (já pronto do lado dele desde a spec 018) passa a
   receber `stats` de verdade e atualizar achievements por métrica em
   host_world_dropin — validar com uma bateria manual end-to-end
   (`ReportParticipationEvent` com stats via grpcurl -> achievement por stat
   desbloqueia).
4. `analytics-service/011-raw-stat-capture` deixa de precisar excluir
   `host_world_dropin` — pode ser estendida numa spec-irmã depois desta.
5. `docs/game-models/host_world_dropin-dataplane-contract.md` atualizado
   (RPC/envelope com `stats`; checklist da linha 171 corrigido).

## Fora de escopo

- O repo `unity-host-world-dropin-data-plane` (Dedicated Server em C#) — essa
  mudança precisa ser feita LÁ pra stats realmente saírem do gameplay (fora
  deste repo; vira item de follow-up cross-repo, mesmo padrão das demais
  specs de contrato). Esta spec só destrava o control plane pra aceitar o
  campo quando o data plane passar a enviá-lo.

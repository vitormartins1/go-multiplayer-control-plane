# Spec: 027-session-health-passthrough

**Serviço:** admin-bff
**Status:** Done
**Data:** 2026-07-13
**Parte de:** analytics-service/013-session-health (fatia pequena/aditiva —
passthrough, sem decisão técnica nova; a cadeia `modeId` já existe desde
o item A, spec 026).

## Contexto

O analytics-service (spec 013) passa a devolver `sessionHealth` em
`GameAnalytics`. O admin-bff só precisa repassar o campo novo — a cadeia
`Client.GameAnalytics`/`Service.GameAnalytics`/handler já existe e já
aceita `modeID` (spec 026), sem mudança de assinatura.

## Comportamento

- `GET /api/analytics/games/{gameId}` (admin-bff) SHALL repassar
  `sessionHealth` tal como vem do analytics-service, sem transformação.

## Contratos de Interface

```go
// internal/domain/analytics.go
type GameAnalyticsDetail struct {
    // ... campos existentes
    SessionHealth []SessionCloseStat `json:"sessionHealth"`
}

type SessionCloseStat struct {
    ModeID        string  `json:"modeId,omitempty"`
    Archetype     string  `json:"archetype,omitempty"`
    CloseReason   string  `json:"closeReason"`
    Count         int64   `json:"count"`
    DurationSumMs int64   `json:"durationSumMs"`
    AvgDurationMs float64 `json:"avgDurationMs"`
}
```

## Critérios de Aceite

1. `GET /api/analytics/games/{gameId}` -> `sessionHealth` presente e
   igual ao do analytics-service, com e sem `?modeId=`.
2. Jogo sem nenhum `session_closed` processado ainda -> `sessionHealth`
   vazio, sem erro.

## Fora de escopo

- Qualquer lógica de interpretação (abandono etc.) — cabe ao admin-ui.

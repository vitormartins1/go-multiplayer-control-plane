# Spec: 028-session-closed-daily-passthrough

**Serviço:** admin-bff
**Status:** Done (implementada e mergeada; status corrigido na revisão 2026-09)
**Data:** 2026-07-13
**Parte de:** analytics-service/015-session-closed-daily (fatia pequena/
aditiva — passthrough, sem decisão técnica nova).

## Contexto

O analytics-service (spec 015) passa a devolver `sessionCloseDaily` em
`GameAnalytics`. Passthrough puro, mesma cadeia já usada pras outras 3
séries diárias (`Daily`/`PlacementDaily`/`MissionDaily`/
`ParticipationDaily` já existem em `GameAnalyticsDetail`).

## Comportamento

- `GET /api/analytics/games/{gameId}` SHALL repassar `sessionCloseDaily`
  tal como vem do analytics-service.

## Contratos de Interface

```go
// internal/domain/analytics.go
type GameAnalyticsDetail struct {
    // ... campos existentes
    SessionCloseDaily []AnalyticsDaily `json:"sessionCloseDaily"`
}
```

## Critérios de Aceite

1. `sessionCloseDaily` presente e igual ao do analytics-service, com e sem
   `?modeId=`.

## Fora de escopo

Nenhum — fatia mecânica.

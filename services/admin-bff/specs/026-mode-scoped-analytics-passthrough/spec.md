# Spec: 026-mode-scoped-analytics-passthrough

**Serviço:** admin-bff
**Status:** Done
**Data:** 2026-07-13
**Parte de:** analytics-service/012-mode-scoped-analytics-filter (fatia
pequena/aditiva — passthrough do parâmetro `modeId`, sem decisão técnica
nova)

## Contexto

O analytics-service (spec 012) passa a aceitar `?modeId=` opcional em
`GET /admin/games/{gameId}/analytics`, filtrando as séries mode-scoped. O
admin-bff só precisa repassar o parâmetro pela cadeia já existente
(`Client.GameAnalytics` → `Service.GameAnalytics` → handler HTTP) — mesmo
padrão de `days`, sem lógica nova.

## Comportamento

- `GET /api/games/{gameId}/analytics?days=30&modeId=mundo-vivo` (admin-bff)
  SHALL repassar `modeId` pro analytics-service tal como chega. Ausente =
  comportamento atual (todos os modos), sem mudança.
- `domain.GameAnalyticsDetail` SHALL ganhar `ModeID` (eco do filtro
  aplicado, passthrough do analytics-service).

## Contratos de Interface

```go
// internal/domain/analytics.go
type GameAnalyticsDetail struct {
    // ... campos existentes
    ModeID string // eco do filtro (spec 012 do analytics-service); "" = todos os modos
}

// internal/analytics/client.go, internal/service/admin_service.go
GameAnalytics(ctx context.Context, gameID string, days int, modeID string) (domain.GameAnalyticsDetail, error)
```

## Critérios de Aceite

1. Chamada sem `modeId` -> resposta idêntica à atual (regressão zero).
2. Chamada com `modeId` -> repassada ao analytics-service, resposta reflete
   o filtro (validado contra o analytics-service real).

## Fora de escopo

- Qualquer UI que use o parâmetro — item B, próxima fatia da ordem
  combinada com o usuário.

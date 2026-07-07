# Spec: 015-battle-royale-placement

**Serviço:** admin-bff · **Status:** Rascunho (aguardando aprovação) · **Data:** 2026-07-07
**ADR:** 0023 — arquétipo `battle_royale` · **Roadmap:** BR4 (surfacing), parte bff
**Depende de:** BR3 (`analytics/005` — o `GameAnalytics` já expõe `placement`)

## Contexto

O analytics-service passou a expor, no detalhe de um jogo (`GET /admin/games/{id}/analytics`), a distribuição de colocação do `battle_royale` (`placement`: histograma 1º/top-10/resto + sobrevivência média — BR3). O admin-bff é o passthrough tipado desse detalhe (`GameAnalyticsDetail`, injeta `gameName`) — hoje ele **não** conhece o campo `placement`, então ele **some** no repasse ao console. Esta fatia propaga o campo.

O **perfil** de multiplayer já é passthrough **raw** (`json.RawMessage`) no bff — o `battle_royale` já flui sem mudança; nada a fazer no perfil aqui.

## Comportamento

- WHEN o bff lê o detalhe de analytics de um jogo com `placement` presente → THE SYSTEM SHALL desserializar e repassar o objeto `placement` intacto (`winners`, `topN`, `rest`, `entries`, `avgSurvivedMs`) ao console, junto do resto (`gameName` injetado como hoje).
- IF o jogo não tem `placement` (não-BR) → o campo é omitido (`omitempty`), sem regressão no queue_arena/host_world_dropin.

## Contrato

- `internal/domain/analytics.go`: `GameAnalyticsDetail` ganha `Placement *PlacementDistribution` (json `placement,omitempty`); novo tipo `PlacementDistribution{ Winners, TopN, Rest, Entries int64; AvgSurvivedMs float64 }` (mesmas chaves JSON do analytics-service: `winners`/`topN`/`rest`/`entries`/`avgSurvivedMs`).
- Sem mudança no client (decodifica no mesmo struct) nem no handler (retorna o struct; o campo flui por tag).

## Casos de Teste Obrigatórios

- [ ] (client/integração fake) resposta do analytics com `placement` → `GameAnalyticsDetail.Placement` preenchido (winners/topN/rest/entries/avgSurvivedMs)
- [ ] (handler) resposta sem `placement` → campo ausente no JSON de saída (omitempty); analytics sem regressão

## Fora do escopo

- Perfil `battle_royale` (passthrough raw, já flui) · surfacing no front (`admin-ui/021`).
